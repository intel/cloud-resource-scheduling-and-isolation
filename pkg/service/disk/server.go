/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	utils "sigs.k8s.io/IOIsolation/pkg"

	"k8s.io/klog/v2"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
	"sigs.k8s.io/IOIsolation/pkg/service"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

const (
	Num             uint32  = 6
	kubeletPath     string  = "/var/lib/kubelet"
	lsblkCmd        string  = "lsblk %s -pbJo name,type,maj:min,mountpoint,model,serial,rota,pkname"
	mountLinksDir   string  = "/opt/ioi/"
	getInodeCmd     string  = "ls -id %s"
	devicesPath     string  = "/sys/block"
	FilterDataValue float64 = 1024
	mountBindDirCmd string  = "mount --bind %s %s"
	mountDirCmd     string  = "mount %s %s"
)

var (
	MessageHeader = "Disk IO"
	TypeDiskIO    = "DiskIO"
)

type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}

type bdata []float64
type RW struct {
	R map[string]bdata // key is pod id/name, value is read bandwidth
	W map[string]bdata
}

type AppInfo struct {
	AppName    string   `json:"appname"`
	CgroupPath string   `json:"cgrouppath"`
	DeviceMM   []string `json:"devicemm"`
}

// DiskEngine
type DiskEngine struct {
	service.ServiceEngine
	persist        common.IPersist
	Apps           map[string]*AppInfo
	Interval       utils.IntervalConfigInfo
	ContainerToPod map[uint64]string
	EbpfBuckets    map[uint64]uint64
	EbpfEnabled    bool
	prevRawData    *RawData
	prevMsg        *pb.AppsBandwidth
	LoopDevMap     map[string]string
	sync.Mutex
}

func (d *DiskEngine) Type() int32 {
	return utils.DiskIOIType
}

func (d *DiskEngine) Initialize(persist common.IPersist) error {
	// create buckets
	d.persist = persist
	diskBuckets := []string{"APPS", "GROUPS"}
	err := persist.CreateTable("DISK", diskBuckets)
	if err != nil {
		klog.Warningf("Create disk buckets fail: %s", err)
	}

	d.Apps = make(map[string]*AppInfo)
	// load app info from db
	apps, err1 := GetAllAppInfos(d, "")
	if err1 != nil {
		klog.Warningf("Get all app infos fail: %s", err)
	} else {
		d.Apps = apps
	}

	clearPodTme := viper.GetInt64("disk.clear_pod_time")
	go d.CheckPodCgroup(clearPodTme)

	d.ContainerToPod = make(map[uint64]string)
	d.EbpfBuckets = make(map[uint64]uint64)
	d.LoopDevMap = make(map[string]string)
	d.EbpfEnabled = viper.GetBool("disk.ebpf_enabled")

	// for interval
	d.Interval = utils.IntervalConfigInfo{
		Interval: 2,
	}
	configInterval, err1 := LoadInterval(d)
	if err1 != nil {
		klog.Warningf("Get interval failed: %s", err1)
	} else {
		d.Interval = configInterval
	}
	klog.Infof("The server is start interval: %v", configInterval)

	// load ebpf first
	if d.EbpfEnabled {
		err = InitEbpf()
		if err != nil {
			d.EbpfEnabled = false
		}
	}
	// ebpf fail or not enabled, then go cgroup
	if !d.EbpfEnabled {
		d.prevRawData = d.GetAppsReadWrite()
		curBw := d.GetCurBandwidth(d.prevRawData, d.prevRawData)
		d.prevMsg = diskCreateMsgCgroup(1, curBw)
	}

	return nil
}

func InitEbpf() error {
	// Allow the current process to lock memory for eBPF resources.
	err := rlimit.RemoveMemlock()
	if err != nil {
		klog.Errorf("removing memlock rlimit: %v", err)
		return err
	}

	// Load pre-compiled programs and maps into the kernel.
	objs = bpfObjects{}
	err = loadBpfObjects(&objs, nil)
	if err != nil {
		klog.Errorf("loading objects: %v", err)
		return err
	}

	tp, err = link.Tracepoint("block", "block_bio_queue", objs.BlockBioQueue, nil)
	if err != nil {
		klog.Errorf("opening tracepoint: %v", err)
		return err
	}

	return nil
}

func (d *DiskEngine) Uninitialize() error {
	klog.V(utils.INF).Info("persistent: file close")
	if d.EbpfEnabled {
		err := objs.Close()
		if err != nil {
			klog.Errorf("closing objects: %v", err)
			return err
		}
		err = tp.Close()
		if err != nil {
			klog.Errorf("closing tracepoint: %v", err)
			return err
		}
	}

	return nil
}

func (d *DiskEngine) CheckPodCgroup(clearPodTime int64) {
	for {
		for appId, appInfo := range d.Apps {
			_, err := os.Stat(appInfo.CgroupPath)
			if errors.Is(err, os.ErrNotExist) {
				err := d.deleteAppInfo(appId)
				if err != nil {
					klog.Warningf("delete appId failed %v: %s", err, appId)
				}
			}
		}
		time.Sleep(time.Duration(clearPodTime) * time.Second)
	}
}

func (d *DiskEngine) Configure(ctx context.Context, req *pb.ConfigureRequest) (*pb.ConfigureResponse, error) {
	if d.persist == nil {
		return nil, status.Error(codes.Unimplemented, "db is nil")
	}
	if req.Configure == "" {
		return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, nil
	}

	if req.Type == utils.INTERVAL {
		var interval utils.IntervalConfigInfo
		err := json.Unmarshal([]byte(req.Configure), &interval)
		if err == nil {
			klog.Infof("parseConfigureRequest Interval success %v\n", interval)
		} else {
			klog.Errorf("parseConfigureRequest Interval failed because string format is wrong, %s\n", err)
			return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, nil
		}
		klog.Infof("Configure Interval %v.", d.Interval)
		d.Interval.Interval = interval.Interval
		err = SaveInterval(d, interval)
		if err != nil {
			klog.Errorf("Configure %s", err)
			return nil, status.Error(codes.Unimplemented, "update db for interval failed")
		}
		return &pb.ConfigureResponse{Status: pb.ResponseState_OK}, nil
	} else {
		return nil, status.Error(codes.InvalidArgument, "the Req type is invalid")
	}
}

func (s *DiskEngine) SetAppLimit(ctx context.Context, req *pb.SetAppLimitRequest) (*pb.SetAppLimitResponse, error) {
	//TODO: Need to avoid overwrite another disk when pod has multiple disks
	klog.V(utils.DBG).Infof("Now start to SetAppLimit. req = %v", req)

	appid := strings.Split(req.AppId, ".")
	if len(appid) <= 1 {
		return nil, errors.New("disk not support group limit")
	}
	if app, OK := s.Apps[appid[1]]; OK {
		// set the limit via cgroup -- AppId is the cgroup path
		values := ""
		for _, limit := range req.Limit {
			if (limit.In != "" && limit.In != "0") || (limit.Out != "" && limit.Out != "0") {
				values += fmt.Sprintf("%s ", limit.Id)
				if limit.In != "" && limit.In != "0" {
					values += fmt.Sprintf("rbps=%s ", limit.In)
					values += fmt.Sprintf("riops=%d ", limit.InIops)
				}

				if limit.Out != "" && limit.Out != "0" {
					values += fmt.Sprintf("wbps=%s ", limit.Out)
					values += fmt.Sprintf("wiops=%d ", limit.OutIops)
				}
			}
			values += "\n"
		}
		values = values[:len(values)-1]
		klog.V(utils.DBG).Info(values)
		if values != "" {
			max := "/io.max"
			path := fmt.Sprintf("%s%s", app.CgroupPath, max)
			klog.V(utils.DBG).Info(path)
			_, err := os.ReadFile(path)
			if err != nil {
				klog.V(utils.INF).Info("It does not exist this path: ", path)
				return &pb.SetAppLimitResponse{}, nil
			}
			klog.V(utils.DBG).Info("writeFile")
			err = os.WriteFile(path, []byte(values), 0666)
			if err != nil {
				klog.V(utils.INF).Infof("write failed: %s", err)
				return nil, err
			} else {
				klog.V(utils.INF).Infof("This AppLimit is set: %s", app.AppName)
				klog.V(utils.INF).Infof("The limit value is: %s", values)
				return &pb.SetAppLimitResponse{}, nil
			}
		} else {
			klog.V(utils.INF).Info("no value to be written to io.max!\n")
			return &pb.SetAppLimitResponse{}, nil
		}
	} else {
		return nil, errors.New("app not registered")
	}
}

func parseDiskAppkString(mockString string) (utils.DiskCgroupInfo, error) {
	var diskapp utils.DiskCgroupInfo

	if err := json.Unmarshal([]byte(mockString), &diskapp); err == nil {
		klog.V(utils.DBG).Info(diskapp)
	} else {
		klog.Warning("parse mock string failed because mock string format is wrong")
		return diskapp, err
	}

	return diskapp, nil
}

func checkValidDevice(path string) bool {
	match, err := regexp.MatchString("([0-9]+:[0-9])+", path)
	if err != nil {
		klog.Errorf("failed to validate device: %s", err)
	}
	klog.V(utils.DBG).Infof("device id: %v, match: %v", path, match)
	return match
}

func (s *DiskEngine) RegisterApp(ctx context.Context, req *pb.RegisterAppRequest) (*pb.RegisterAppResponse, error) {
	if _, OK := s.Apps[req.AppName]; OK {
		return nil, status.Error(codes.AlreadyExists, "App Already Registered")
	}

	pp, err := parseDiskAppkString(req.AppInfo)
	if err != nil {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, err
	}

	if len(pp.DeviceId) == 0 || req.AppName == "" || pp.CgroupPath == "" {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("input information for RegisterApp is not complete. AppName, CgroupPath and DeviceId are required")
	}

	_, err = os.Stat(pp.CgroupPath)
	if err != nil {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("CgroupPath is invalid or does not exist")
	}

	for _, d := range pp.DeviceId {
		if !checkValidDevice(d) {
			klog.V(utils.INF).Infof("checkValidDevice failed %v", d)
			return &pb.RegisterAppResponse{
				AppId:  req.AppName,
				Status: pb.ResponseState_FAIL,
			}, errors.New("DeviceId is invalid")
		}
	}

	_, err = os.Readlink(pp.CgroupPath)
	if err == nil {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("CgroupPath is a symbolic link, which is not allowed")
	}
	err = s.MapLoopToDisk()
	if err != nil {
		klog.Warningf("MapLoopToDisk failed: %v", err)
	}

	a := AppInfo{
		AppName:    req.AppName,
		CgroupPath: pp.CgroupPath,
		DeviceMM:   pp.DeviceId,
	}
	s.Apps[req.AppName] = &a

	err = AddAppInfo(s, a)
	if err != nil {
		klog.Errorf("update app file %s", err)
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("persistent file failed")
	}

	return &pb.RegisterAppResponse{
		AppId:  req.AppName,
		Status: pb.ResponseState_OK,
	}, nil
}

func (s *DiskEngine) UnRegisterApp(ctx context.Context, req *pb.UnRegisterAppRequest) (*pb.UnRegisterAppResponse, error) {
	if _, OK := s.Apps[req.AppId]; !OK {
		return nil, status.Error(codes.NotFound, "App Not Found")
	}

	err := s.deleteAppInfo(req.AppId)
	if err != nil {
		return &pb.UnRegisterAppResponse{Status: pb.ResponseState_FAIL}, err
	}

	return &pb.UnRegisterAppResponse{Status: pb.ResponseState_OK}, nil
}

func (s *DiskEngine) deleteAppInfo(AppId string) error {
	// delete app info from db
	err := DeleteAppInfo(s, s.Apps[AppId].AppName)
	if err != nil {
		klog.Warningf("delete app info failed: %s", err)
	}

	delete(s.Apps, AppId)

	if s.EbpfEnabled {
		// clear container mapping data and ebpf filter
		cgroupids := []uint64{}
		for ci, pod := range s.ContainerToPod {
			if pod == AppId {
				cgroupids = append(cgroupids, ci)
			}
		}
		for _, ci := range cgroupids {
			delete(s.ContainerToPod, ci)
			err := objs.FilterMap.Delete(CGroupID{Cgroup: ci})
			if err != nil {
				klog.Error("Delete filter failed: ", err)
			}
		}
	}

	return nil
}

func diskCreateMsgEbpf(ioiType int32, data map[string]RW) *pb.AppsBandwidth {
	BWs := make(map[string]*pb.AppBandwidthInfo)

	for po, rw := range data {
		var bwinfo []*pb.BandwidthInfo
		for di, re := range rw.R {
			bw := make(map[string]string)
			re[1] += re[2]
			re[6] += re[7] + re[8]
			bw["512b"] = strconv.FormatFloat(re[0], 'f', -1, 64)
			bw["1k"] = strconv.FormatFloat(re[1], 'f', -1, 64)
			bw["4k"] = strconv.FormatFloat(re[3], 'f', -1, 64)
			bw["8k"] = strconv.FormatFloat(re[4], 'f', -1, 64)
			bw["16k"] = strconv.FormatFloat(re[5], 'f', -1, 64)
			bw["32k"] = strconv.FormatFloat(re[6], 'f', -1, 64)
			b := &pb.BandwidthInfo{
				Id:         di,
				IsRead:     true,
				Bandwidths: bw,
			}
			bwinfo = append(bwinfo, b)
		}
		for di, wr := range rw.W {
			bw := make(map[string]string)
			wr[1] += wr[2]
			wr[6] += wr[7] + wr[8]
			bw["512b"] = strconv.FormatFloat(wr[0], 'f', -1, 64)
			bw["1k"] = strconv.FormatFloat(wr[1], 'f', -1, 64)
			bw["4k"] = strconv.FormatFloat(wr[3], 'f', -1, 64)
			bw["8k"] = strconv.FormatFloat(wr[4], 'f', -1, 64)
			bw["16k"] = strconv.FormatFloat(wr[5], 'f', -1, 64)
			bw["32k"] = strconv.FormatFloat(wr[6], 'f', -1, 64)
			c := &pb.BandwidthInfo{
				Id:         di,
				IsRead:     false,
				Bandwidths: bw,
			}
			bwinfo = append(bwinfo, c)
		}
		BWs[po] = &pb.AppBandwidthInfo{BwInfo: bwinfo}
	}
	if ioiType == 1 {
		msg := pb.AppsBandwidth{
			IoiType: ioiType,
			Payload: &pb.AppsBandwidth_DiskIoBw{
				DiskIoBw: &pb.AppDiskIOBandwidth{
					AppBwInfo: BWs,
				}},
		}
		return &msg
	}

	return nil
}

func diskCreateMsgCgroup(ioiType int32, data map[string]RW) *pb.AppsBandwidth {
	BWs := make(map[string]*pb.AppBandwidthInfo)

	for po, rw := range data {
		var bwinfo []*pb.BandwidthInfo
		for di, re := range rw.R {
			bw := make(map[string]string)
			bw["512b"] = strconv.FormatFloat(re[0], 'f', -1, 64)
			bw["1k"] = strconv.FormatFloat(re[1], 'f', -1, 64)
			bw["4k"] = strconv.FormatFloat(re[2], 'f', -1, 64)
			bw["8k"] = strconv.FormatFloat(re[3], 'f', -1, 64)
			bw["16k"] = strconv.FormatFloat(re[4], 'f', -1, 64)
			bw["32k"] = strconv.FormatFloat(re[5], 'f', -1, 64)
			b := &pb.BandwidthInfo{
				Id:         di,
				IsRead:     true,
				Bandwidths: bw,
			}
			bwinfo = append(bwinfo, b)
		}
		for di, wr := range rw.W {
			bw := make(map[string]string)
			bw["512b"] = strconv.FormatFloat((wr[0]), 'f', -1, 64)
			bw["1k"] = strconv.FormatFloat(wr[1], 'f', -1, 64)
			bw["4k"] = strconv.FormatFloat(wr[2], 'f', -1, 64)
			bw["8k"] = strconv.FormatFloat(wr[3], 'f', -1, 64)
			bw["16k"] = strconv.FormatFloat(wr[4], 'f', -1, 64)
			bw["32k"] = strconv.FormatFloat(wr[5], 'f', -1, 64)
			c := &pb.BandwidthInfo{
				Id:         di,
				IsRead:     false,
				Bandwidths: bw,
			}
			bwinfo = append(bwinfo, c)
		}
		BWs[po] = &pb.AppBandwidthInfo{BwInfo: bwinfo}
	}
	if ioiType == 1 {
		msg := pb.AppsBandwidth{
			IoiType: ioiType,
			Payload: &pb.AppsBandwidth_DiskIoBw{
				DiskIoBw: &pb.AppDiskIOBandwidth{
					AppBwInfo: BWs,
				}},
		}
		return &msg
	}

	return nil
}

func (s *DiskEngine) Subscribe(req *pb.SubscribeContext, stream pb.Ioiservice_SubscribeServer) error {
	if req.IoiType != 1 {
		return nil
	}
	s.Name = ""
	// s.Stream = stream
	err := s.MapLoopToDisk()
	if err != nil {
		klog.Warningf("MapLoopToDisk failed: %v", err)
	}
	if s.EbpfEnabled {
		klog.Info("ebpf-based disk IO monitoring is enabled")
	} else {
		klog.Info("cgroup-based disk IO monitoring is enabled")
	}
	for {
		if s.EbpfEnabled {
			if err := s.EbpfSyncAndSend(req, stream); err != nil {
				return err
			}
		} else {
			if err := s.CgroupSyncAndSend(req, stream); err != nil {
				return err
			}

		}
	}
}

func (s *DiskEngine) MapLoopToDisk() error {
	if s.LoopDevMap == nil {
		s.LoopDevMap = make(map[string]string)
	}
	devices, err := os.ReadDir(devicesPath)
	if err != nil {
		return err
	}
	for _, device := range devices {
		if strings.HasPrefix(device.Name(), "loop") {
			backingFilePath := filepath.Join(devicesPath, device.Name(), "loop", "backing_file")
			loopFilePath := filepath.Join(devicesPath, device.Name(), "dev")

			// Read the backing file path
			backingFile, err := os.ReadFile(backingFilePath)
			if err != nil {
				continue
			}
			if !strings.HasPrefix(string(backingFile), mountLinksDir) {
				continue
			}
			diskName := ""
			rest := strings.TrimPrefix(string(backingFile), mountLinksDir)
			parts := strings.Split(rest, "/")
			if len(parts) > 0 {
				diskName = parts[0]
			} else {
				continue
			}
			diskFilePath := filepath.Join(devicesPath, diskName, "dev")
			diskMM, err := os.ReadFile(diskFilePath)
			if err != nil {
				klog.Warningf("Error reading dev file for %s: %s\n", diskName, err)
				continue
			}
			// Read loop major:minor numbers
			loopMM, err := os.ReadFile(loopFilePath)
			if err != nil {
				klog.Warningf("Error reading dev file for %s: %s\n", device.Name(), err)
				continue
			}
			loop := strings.ReplaceAll(string(loopMM), "\n", "")
			loop = strings.ReplaceAll(loop, "\r", "")
			disk := strings.ReplaceAll(string(diskMM), "\n", "")
			disk = strings.ReplaceAll(disk, "\r", "")
			s.LoopDevMap[loop] = disk
		}
	}
	klog.Info("Update LoopDevMap: ", s.LoopDevMap)
	return nil
}

func (s *DiskEngine) EbpfSyncAndSend(req *pb.SubscribeContext, stream pb.Ioiservice_SubscribeServer) error {
	// 1. update container list in service cache and ebpf filter
	s.SynchronizeContainerList()
	// 2.get current bandwidth data
	time.Sleep(time.Duration(s.Interval.Interval) * time.Second)
	curData := s.UpdateCountingMap()
	// 3. send bandwidth data to client
	err := s.Send(req.IoiType, curData, stream)
	if err != nil {
		klog.Error("send app bandwidth failed: ", err)
		return err
	}
	return nil
}

func (s *DiskEngine) CgroupSyncAndSend(req *pb.SubscribeContext, stream pb.Ioiservice_SubscribeServer) error {
	// 1.get current bandwidth data
	num := len(s.Apps)
	if num == 0 {
		klog.V(utils.INF).Infof("there is no registered app now, please wait")
	} else {
		prvRawData := s.prevRawData
		curRawData := s.GetAppsReadWrite()
		curData := s.GetCurBandwidth(prvRawData, curRawData)
		// 2.send bandwidth data to client
		klog.V(utils.DBG).Info("curData", curData)
		s.prevRawData = curRawData
		err := s.Send(req.IoiType, curData, stream)
		if err != nil {
			klog.Error("send app bandwidth failed: ", err)
			return err
		}
	}
	time.Sleep(time.Second * time.Duration(s.Interval.Interval))
	return nil
}

func (s *DiskEngine) Send(ioiType int32, data map[string]RW, stream pb.Ioiservice_SubscribeServer) error {
	var curMsg *pb.AppsBandwidth
	if len(data) == 0 {
		return nil
	}

	if s.EbpfEnabled {
		curMsg = diskCreateMsgEbpf(ioiType, data)
	} else {
		curMsg = diskCreateMsgCgroup(ioiType, data)
	}
	sndflag, err := FilterData(s.prevMsg, curMsg)
	if err != nil {
		klog.Errorf("filter data failed %s", err)
		return err
	}
	s.prevMsg = curMsg
	if sndflag {
		klog.V(utils.DBG).Info("result:", curMsg.String())
		err = stream.Send(curMsg)
		if err != nil {
			klog.Errorf("send failed %s", err)
		}
	}
	return err
}

func ComparePodList(prevAppBwInfo map[string]*pb.AppBandwidthInfo, curAppBwInfo map[string]*pb.AppBandwidthInfo) bool {
	pKeys, cKeys := []string{}, []string{}
	for key := range prevAppBwInfo {
		pKeys = append(pKeys, key)
	}
	for key := range curAppBwInfo {
		cKeys = append(cKeys, key)
	}
	sort.Strings(pKeys)
	sort.Strings(cKeys)
	return reflect.DeepEqual(pKeys, cKeys)
}

// filter the data change greater than > 1024, will report it again.
func FilterData(prev *pb.AppsBandwidth, cur *pb.AppsBandwidth) (bool, error) {
	var curBsBw, preBsBw float64
	var err error
	if cur == nil {
		klog.Errorf("cur is nil")
		return false, errors.New("cur is nil")
	}
	if prev == nil {
		return true, nil
	}
	bsList := []string{"512b", "1k", "4k", "8k", "16k", "32k"}

	prevAppBw := prev.GetDiskIoBw()
	if prevAppBw == nil {
		klog.Errorf("prevAppBw is nil")
		return false, errors.New("prevAppBw is nil")
	}
	curAppBw := cur.GetDiskIoBw()
	if curAppBw == nil {
		klog.Errorf("curAppBw is nil")
		return false, errors.New("curAppBw is nil")
	}
	prevAppBwInfo := prevAppBw.AppBwInfo
	curAppBwInfo := curAppBw.AppBwInfo

	isPodsSame := ComparePodList(prevAppBwInfo, curAppBwInfo)
	if !isPodsSame {
		return true, nil
	}

	for po, bwInfo := range curAppBwInfo {
		if prevBwInfo, ok := prevAppBwInfo[po]; ok {
			for _, bw := range bwInfo.BwInfo {
				if bw.IsRead {
					for _, prevBw := range prevBwInfo.BwInfo {
						if prevBw.IsRead && bw.Id == prevBw.Id {
							for _, bs := range bsList {
								curBsBw, err = strconv.ParseFloat(bw.Bandwidths[bs], 32)
								if err != nil {
									klog.Errorf("parse float failed %s", err)
									return false, err
								}
								preBsBw, err = strconv.ParseFloat(prevBw.Bandwidths[bs], 32)
								if err != nil {
									klog.Errorf("parse float failed %s", err)
									return false, err
								}
								if math.Abs(float64(curBsBw)-float64(preBsBw)) > FilterDataValue {
									return true, nil
								}
							}
						}
					}
				} else {
					for _, prevBw := range prevBwInfo.BwInfo {
						if !prevBw.IsRead && bw.Id == prevBw.Id {
							for _, bs := range bsList {
								curBsBw, err = strconv.ParseFloat(bw.Bandwidths[bs], 32)
								if err != nil {
									klog.Errorf("parse float failed %s", err)
									return false, err
								}
								preBsBw, err = strconv.ParseFloat(prevBw.Bandwidths[bs], 32)
								if err != nil {
									klog.Errorf("parse float failed %s", err)
									return false, err
								}
								if math.Abs(float64(curBsBw)-float64(preBsBw)) > FilterDataValue {
									return true, nil
								}
							}
						}
					}
				}
			}
		}
	}

	return false, nil
}

type RawData struct {
	rbps  *map[string]map[string]int
	wbps  *map[string]map[string]int
	riops *map[string]map[string]int
	wiops *map[string]map[string]int
}

func (s *DiskEngine) GetAppsReadWrite() *RawData {
	var runData RawData
	rbps := make(map[string]map[string]int)
	wbps := make(map[string]map[string]int)
	riops := make(map[string]map[string]int)
	wiops := make(map[string]map[string]int)

	for _, appinfo := range s.Apps {
		cgroupPath := appinfo.CgroupPath
		if strings.Contains(cgroupPath, "cri-containerd") {
			continue
		}
		r := make(map[string]int)
		w := make(map[string]int)
		ri := make(map[string]int)
		wi := make(map[string]int)
		for _, di := range appinfo.DeviceMM {
			re, wr, rie, wir := s.GetReadWrite(cgroupPath, di)
			if re == -1 && wr == -1 && rie == -1 && wir == -1 {
				klog.V(utils.INF).Info("There is currently no disk IO in this pod: ", appinfo.AppName, " diskMM:", di)
			} else {
				r[di] = re
				w[di] = wr
				ri[di] = rie
				wi[di] = wir
			}
		}
		rbps[appinfo.AppName] = r
		wbps[appinfo.AppName] = w
		riops[appinfo.AppName] = ri
		wiops[appinfo.AppName] = wi
	}
	runData.rbps = &rbps
	runData.wbps = &wbps
	runData.riops = &riops
	runData.wiops = &wiops

	return &runData
}

func (s *DiskEngine) GetCurBandwidth(preRawData *RawData, curRawData *RawData) map[string]RW {
	curData := make(map[string]RW, 0)
	inteval := float64(s.Interval.Interval)
	for name, read := range *curRawData.rbps {
		actualBW := RW{
			R: make(map[string]bdata),
			W: make(map[string]bdata),
		}
		for di := range read {
			var deltaRbps, deltaWbps, deltaRiops, deltaWiops float64
			r := make([]float64, Num)
			w := make([]float64, Num)
			if _, ok := (*preRawData.rbps)[name][di]; ok {
				deltaRbps = math.Max(float64((*curRawData.rbps)[name][di]-(*preRawData.rbps)[name][di]), 0) / inteval
				deltaWbps = math.Max(float64((*curRawData.wbps)[name][di]-(*preRawData.wbps)[name][di]), 0) / inteval
				deltaRiops = math.Max(float64((*curRawData.riops)[name][di]-(*preRawData.riops)[name][di]), 0) / inteval
				deltaWiops = math.Max(float64((*curRawData.wiops)[name][di]-(*preRawData.wiops)[name][di]), 0) / inteval

			} else {
				deltaRbps = math.Max(float64((*curRawData.rbps)[name][di]), 0) / inteval
				deltaWbps = math.Max(float64((*curRawData.wbps)[name][di]), 0) / inteval
				deltaRiops = math.Max(float64((*curRawData.riops)[name][di]), 0) / inteval
				deltaWiops = math.Max(float64((*curRawData.wiops)[name][di]), 0) / inteval
			}

			ConvertData(deltaRbps, deltaRiops, r)
			ConvertData(deltaWbps, deltaWiops, w)

			actualBW.R[di] = r
			actualBW.W[di] = w
		}
		curData[name] = actualBW
	}
	return curData
}

func ConvertData(bps float64, iops float64, data []float64) {
	bs := bps / iops

	klog.V(utils.DBG).Infof("bps = %v, iops = %v, bs = %v", bps, iops, bs)

	if bs <= 512 {
		data[0] = bps
	} else if bs < 1024 {
		data[0], data[1] = SolveEquations(512, 1024, bps, 1, 1, iops)
	} else if bs == 1024 {
		data[1] = bps
	} else if bs < 4096 {
		data[1], data[2] = SolveEquations(1024, 4096, bps, 1, 1, iops)
	} else if bs == 4096 {
		data[2] = bps
	} else if bs < 8192 {
		data[2], data[3] = SolveEquations(4096, 8192, bps, 1, 1, iops)
	} else if bs == 8192 {
		data[3] = bps
	} else if bs < 16384 {
		data[3], data[4] = SolveEquations(8192, 16384, bps, 1, 1, iops)
	} else if bs == 16384 {
		data[4] = bps
	} else if bs < 32768 {
		data[4], data[5] = SolveEquations(16384, 32768, bps, 1, 1, iops)
	} else {
		data[5] = bps
	}
}

func SolveEquations(a1, b1, c1, a2, b2, c2 float64) (float64, float64) {
	// Calculate the line expression
	det := a1*b2 - a2*b1

	if det == 0 {
		klog.Error("Can not solve equations!")
		return 0, 0
	}

	// Calculate variable
	x := (c1*b2 - c2*b1) / det
	y := (a1*c2 - a2*c1) / det

	return x * a1, y * b1
}

func ListDir(dirname string) []string {
	infos, err := os.ReadDir(dirname)
	if err != nil {
		klog.Error("ListDir failed")
		return nil
	}
	names := []string{}
	for _, info := range infos {
		if strings.HasPrefix(info.Name(), "cri-containerd") {
			names = append(names, info.Name())
		}
	}
	return names
}

func (s *DiskEngine) GetReadWrite(cgroupPath string, DeviceMM string) (int, int, int, int) {
	stat := "/io.stat"
	conPaths := ListDir(cgroupPath)
	for i, p := range conPaths {
		conPaths[i] = cgroupPath + "/" + p + stat
	}
	readResult := 0
	writeResult := 0
	rIops := 0
	wIops := 0
	for _, path := range conPaths {
		file, err := os.Open(path)
		if err != nil {
			klog.V(utils.INF).Infof("no cgroup path now, please wait")
			return -1, -1, -1, -1
		} else {
			defer file.Close()
			rd := bufio.NewReader(file)
			var split []string
			for {
				txt, _, err := rd.ReadLine()
				if err == io.EOF {
					break
				}
				str := string(txt)
				split = append(split, str)
			}
			if len(split) == 0 {
				klog.V(utils.INF).Info("io.stat is empty, path: ", path)
			}
			for i := 0; i < len(split); i++ {
				line := strings.Split(split[i], " ")
				if len(line) < 3 {
					klog.Infof("parse failed")
					continue
				}
				diskID := line[0]
				if diskID != DeviceMM {
					diskID = s.LoopDevMap[diskID]
				}
				if diskID != DeviceMM {
					continue
				}
				rbytes := strings.Split(line[1], "=")
				read, err := strconv.Atoi(rbytes[1])
				if err != nil {
					klog.Warning(err)
				}
				wbytes := strings.Split(line[2], "=")
				write, err := strconv.Atoi(wbytes[1])
				if err != nil {
					klog.Warning(err)
				}
				riops := strings.Split(line[3], "=")
				ri, err := strconv.Atoi(riops[1])
				if err != nil {
					klog.Warning(err)
				}
				wiops := strings.Split(line[4], "=")
				wi, err := strconv.Atoi(wiops[1])
				if err != nil {
					klog.Warning(err)
				}
				readResult += read
				writeResult += write
				rIops += ri
				wIops += wi
			}
		}
	}
	return readResult, writeResult, rIops, wIops
}

type DiskServiceInfo struct {
	DiskName string `json:"disk_name"`
}

func ParseDiskServiceInfoRequestString(s string) ([]DiskServiceInfo, error) {
	dinfos := []DiskServiceInfo{}
	if s == "" {
		return dinfos, errors.New("empty disk ServiceInfo request")
	}

	err := json.Unmarshal([]byte(s), &dinfos)
	if err == nil {
		klog.V(utils.DBG).Infof("parseServiceInfoRequestString success %v\n", dinfos)
	} else {
		klog.Errorf("parseServiceInfoRequestString failed because string format is wrong, %s\n", s)
		return dinfos, err
	}
	return dinfos, nil
}

func getPartSize(path string) int64 {
	dfRes, err := utils.RunBash(fmt.Sprintf("%s %s", "df", path))
	if err != nil {
		klog.Errorf("df command failed: %s", err)
	}
	size, err := strconv.ParseInt(strings.Fields(dfRes)[10], 10, 64)
	if err != nil {
		klog.Errorf("disk size trans to int64 failed: %s", err)
	}
	return size * 1024
}

func judgePartProfile(mnt string) bool {
	// drop sys,swap,not mounted partition
	if strings.HasPrefix(mnt, "/boot/") || mnt == "/boot" || mnt == "[SWAP]" {
		return false
	}
	return true
}

func getAllDiskInfo() (map[string]utils.BlockDevice, error) {
	var bd map[string][]utils.BlockDevice
	lsblkStr, err := utils.RunBash(fmt.Sprintf(lsblkCmd, ""))
	if err != nil {
		klog.Errorf("lsblk command failed: %v", err)
		return nil, err
	}
	if err := json.Unmarshal([]byte(lsblkStr), &bd); err != nil {
		klog.Errorf("parse lsblk result failed: %v", err)
		return nil, err
	}

	res := make(map[string]utils.BlockDevice)

	for _, v := range bd["blockdevices"] {
		if v.Type != "disk" {
			continue
		}
		res[v.Name] = utils.BlockDevice{
			Name: v.Name,
		}
	}
	return res, nil
}

func getADiskInfo(diskName string) (map[string][]utils.BlockDevice, error) {
	var bd map[string][]utils.BlockDevice
	lsblkStr, err := utils.RunBash(fmt.Sprintf(lsblkCmd, diskName))
	if err != nil {
		klog.Errorf("lsblk command failed: %s %s", diskName, err)
		return nil, err
	}
	if err := json.Unmarshal([]byte(lsblkStr), &bd); err != nil {
		klog.Errorf("parse lsblk result failed: %s %s", diskName, err)
		return nil, err
	}
	return bd, nil
}

func getDiskType(id int) string {
	if id == 0 {
		return utils.EmptyDir
	} else {
		return utils.NonEmptyDir
	}
}

func checkDiskDetails(diskList []string) map[string]utils.BlockDevice {
	res := make(map[string]utils.BlockDevice)
	var dName, dMM, dMount, dModel, dSerial, partName string
	var dSize int64
	for idx, inDisk := range diskList {
		bd, err := getADiskInfo(inDisk)
		if err != nil {
			klog.Warningf("the disk %v is skipped: ", err)
			continue
		}
		// differentiate disk or partition
		if bd["blockdevices"][0].PName == "" { // disk
			dName = bd["blockdevices"][0].Name
			dModel = bd["blockdevices"][0].Model
			dSerial = bd["blockdevices"][0].Serial
			dMM = bd["blockdevices"][0].MajMin
			if len(bd["blockdevices"][0].Children) == 0 { // disk don't have partitions
				dMount = bd["blockdevices"][0].Mount
				if !judgePartProfile(dMount) {
					klog.Infof("By the rules, the disk %v is skipped", inDisk)
					continue
				}
				dSize = getPartSize(dName)
			} else { // disk have partitions
				for _, chd := range bd["blockdevices"][0].Children {
					if !judgePartProfile(chd.Mount) {
						continue
					}
					partSize := getPartSize(chd.Name)
					if partSize > dSize {
						partName = chd.Name
						dMount = chd.Mount
						dSize = partSize
					}
				}
				if partName == "" {
					klog.V(utils.INF).Infof("By the rules, the disk %v is skipped", inDisk)
					continue
				}
			}
		} else { //partition
			dName = bd["blockdevices"][0].PName
			dMount = bd["blockdevices"][0].Mount
			if !judgePartProfile(dMount) {
				klog.V(utils.INF).Infof("By the rules, the disk %v is skipped", inDisk)
				continue
			}
			dSize = getPartSize(bd["blockdevices"][0].Name)
			pParent, err := getADiskInfo(dName)
			if err != nil {
				klog.Errorf("get parent disk info failed: %v", err)
				continue
			}
			dMM, dModel, dSerial = pParent["blockdevices"][0].MajMin, pParent["blockdevices"][0].Model, pParent["blockdevices"][0].Serial
		}
		did := utils.GetDiskId(dModel, dSerial)
		res[did] = utils.BlockDevice{
			Name:      dName,
			Type:      getDiskType(idx),
			MajMin:    dMM,
			AvailSize: dSize,
			Mount:     dMount,
			Model:     dModel,
			Serial:    dSerial,
		}
	}
	return res
}

func getMapPath(mnt string, dk string) string {
	var res string
	if string(mnt[len(mnt)-1]) == "/" {
		res = fmt.Sprintf("%s%s_%s", mnt, "dir", dk)
	} else {
		res = fmt.Sprintf("%s/%s_%s", mnt, "dir", dk)
	}
	return res
}

func checkBinds(path1 string, path2 string) bool {
	p1, err := utils.RunBash(fmt.Sprintf(getInodeCmd, path1))
	if err != nil {
		klog.Error("get inode of a directory failed: ", err)
	}
	p1 = strings.Fields(p1)[0]
	p2, err := utils.RunBash(fmt.Sprintf(getInodeCmd, path2))
	if err != nil {
		klog.Error("get inode of a directory failed: ", err)
	}
	p2 = strings.Fields(p2)[0]
	if p1 == p2 {
		return true
	} else {
		return false
	}
}

func bindMounts(disks map[string]utils.BlockDevice) map[string]utils.BlockDevice {
	for dkId, dk := range disks {
		// create binding point
		dSymbolTmp := strings.Split(dk.Name, "/")
		dSymbol := dSymbolTmp[len(dSymbolTmp)-1]
		sftPath := fmt.Sprintf("%s%s", mountLinksDir, dSymbol)
		_, err := os.Stat(sftPath)
		if errors.Is(err, os.ErrNotExist) {
			err := os.MkdirAll(sftPath, 0644)
			if err != nil {
				klog.Error("create binding point failed: ", err)
			}
		}

		if dk.Mount == "" {
			_, err = utils.RunBash(fmt.Sprintf(mountDirCmd, dk.Name, sftPath))
			if err != nil {
				klog.Errorf("mount block device failed: %v %v %v", dk.Name, sftPath, err)
			}
			dTmp := disks[dkId]
			dTmp.Mount = sftPath
			disks[dkId] = dTmp
			continue
		}

		// create mapping point
		mapPath := getMapPath(dk.Mount, dSymbol)
		_, err = os.Stat(mapPath)
		if errors.Is(err, os.ErrNotExist) {
			err := os.MkdirAll(mapPath, 0644)
			if err != nil {
				klog.Error("create mapping point failed: ", err)
			}
		}
		// mount --bind
		if !checkBinds(sftPath, mapPath) {
			_, err = utils.RunBash(fmt.Sprintf(mountBindDirCmd, mapPath, sftPath))
			if err != nil {
				klog.Errorf("mount bind failed: %v %v %v", mapPath, sftPath, err)
			}
		}

		dTmp := disks[dkId]
		dTmp.Mount = sftPath
		disks[dkId] = dTmp
	}
	return disks
}

func (s *DiskEngine) GetServiceInfo(ctx context.Context, req *pb.ServiceInfoRequest) (*pb.ServiceInfoResponse, error) {
	allDisks := []string{}
	if req == nil {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, fmt.Errorf("req is nil")
	}
	klog.V(utils.DBG).Info("Envoke GetServiceInfo:", req.Req, "  Type:", req.IoiType)
	if req.IoiType != utils.DiskIOIType {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, fmt.Errorf("get service info not for disk io service")
	}

	emptyDirPath, err := utils.RunBash(fmt.Sprintf("%s %s", "df", kubeletPath))
	if err != nil {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, err
	}
	emptyDir := strings.Fields(emptyDirPath)[7]
	allDisks = append(allDisks, emptyDir)
	config, err := getAllDiskInfo()
	if err != nil {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, err
	}
	for name := range config {
		if !strings.Contains(emptyDir, name) {
			allDisks = append(allDisks, name)
		}
	}

	config = checkDiskDetails(allDisks)
	for name, v := range config {
		if _, ok := config[name]; !ok {
			config[name] = v
		}
	}

	// bind mounts to defined dir, update mount point
	config = bindMounts(config)
	data, err := json.Marshal(config)
	if err != nil {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, fmt.Errorf("marshal config info fails: %v", err)
	}
	klog.V(utils.DBG).Info("data:", string(data))
	resp := &pb.ServiceInfoResponse{
		Type:          req.IoiType,
		Configuration: string(data),
	}
	return resp, nil
}

func init() {
	a := service.GetService()
	if (a.EnginesSwitch & service.DiskSwitch) == 0 {
		return
	}

	engine := &DiskEngine{
		service.ServiceEngine{},
		nil,
		make(map[string]*AppInfo),
		utils.IntervalConfigInfo{
			Interval: 1,
		},
		make(map[uint64]string),
		make(map[uint64]uint64),
		true,
		&RawData{},
		nil,
		make(map[string]string),
		sync.Mutex{},
	}

	a.RegisterEngine(engine)
}
