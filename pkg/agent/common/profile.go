/*
Copyright 2024 Intel Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	utils "sigs.k8s.io/IOIsolation/pkg"

	"k8s.io/klog/v2"
	"sigs.k8s.io/IOIsolation/pkg/agent"
)

const (
	Filename       = "/opt/diskProfiler/Result.json"
	FakeDisk       = "fakedisk"
	FakeNic        = "fakenic"
	MaxCurrFIOJobs = 1
	profileDir     = "/opt/diskProfiler/"
)

var (
	CurrFIOJobs int
	rw          sync.Mutex
)

type HostInfo struct {
	IpAddr string `json:"ip"`
}

type DiskInput struct {
	DiskName string `json:"disk_name"`
}

var blockSize = []string{"512", "1k", "4k", "8k", "16k", "32k"}

type Bandwidth struct {
	Ratio     int     `json:"ratio,omitempty"`
	Blocksize string  `json:"blocksize,omitempty"`
	Read      float64 `json:"read,omitempty"`
	Write     float64 `json:"write,omitempty"`
}

type DiskProfiler struct {
	Path           string      `json:"path,omitempty"`
	Id             string      `json:"id,omitempty"`
	DeviceMM       string      `json:"deviceMM,omitempty"`
	Status         int         `json:"status,omitempty"`
	Type           string      `json:"type,omitempty"`
	Mount          string      `json:"mount,omitempty"`
	Size           int64       `json:"size,omitempty"`
	SingleWorkload []Bandwidth `json:"singleWorkload,omitempty"`
	HybridWorkload Bandwidth   `json:"hybridWorkload,omitempty"`
}

type Profile struct {
	BaselineBs string                  `json:"baselineBs,omitempty"`
	BwUnit     string                  `json:"bwUnit,omitempty"`
	Disks      map[string]DiskProfiler `json:"disks,omitempty"`
}

type FIO struct {
	TestName  string
	DiskName  string
	RW        string
	BlockSize string
	OupDir    string
	OupFile   string
	Ratio     int
	Size      string
}

type FioGlobalOptions struct {
	Directory  string `json:"directory,omitempty"`
	RandRepeat string `json:"randrepeat,omitempty"`
	Verify     string `json:"verify,omitempty"`
	IOEngine   string `json:"ioengine,omitempty"`
	Direct     string `json:"direct,omitempty"`
	GtodReduce string `json:"gtod_reduce,omitempty"`
}

type FioJobOptions struct {
	Name     string `json:"name,omitempty"`
	BS       string `json:"bs,omitempty"`
	IoDepth  string `json:"iodepth,omitempty"`
	Size     string `json:"size,omitempty"`
	RW       string `json:"rw,omitempty"`
	RampTime string `json:"ramp_time,omitempty"`
	RunTime  string `json:"runtime,omitempty"`
}

type FioNS struct {
	Min    int64   `json:"min,omitempty"`
	Max    int64   `json:"max,omitempty"`
	Mean   float32 `json:"mean,omitempty"`
	StdDev float32 `json:"stddev,omitempty"`
	N      int64   `json:"N,omitempty"`
}

type FioStats struct {
	IOBytes     int64   `json:"io_bytes,omitempty"`
	IOKBytes    int64   `json:"io_kbytes,omitempty"`
	BWBytes     int64   `json:"bw_bytes,omitempty"`
	BW          int64   `json:"bw,omitempty"`
	Iops        float32 `json:"iops,omitempty"`
	Runtime     int64   `json:"runtime,omitempty"`
	TotalIos    int64   `json:"total_ios,omitempty"`
	ShortIos    int64   `json:"short_ios,omitempty"`
	DropIos     int64   `json:"drop_ios,omitempty"`
	SlatNs      FioNS   `json:"slat_ns,omitempty"`
	ClatNs      FioNS   `json:"clat_ns,omitempty"`
	LatNs       FioNS   `json:"lat_ns,omitempty"`
	BwMin       int64   `json:"bw_min,omitempty"`
	BwMax       int64   `json:"bw_max,omitempty"`
	BwAgg       float32 `json:"bw_agg,omitempty"`
	BwMean      float32 `json:"bw_mean,omitempty"`
	BwDev       float32 `json:"bw_dev,omitempty"`
	BwSamples   int32   `json:"bw_samples,omitempty"`
	IopsMin     int32   `json:"iops_min,omitempty"`
	IopsMax     int32   `json:"iops_max,omitempty"`
	IopsMean    float32 `json:"iops_mean,omitempty"`
	IopsStdDev  float32 `json:"iops_stddev,omitempty"`
	IopsSamples int32   `json:"iops_samples,omitempty"`
}

type FioDepth struct {
	FioDepth0    float32 `json:"0,omitempty"`
	FioDepth1    float32 `json:"1,omitempty"`
	FioDepth2    float32 `json:"2,omitempty"`
	FioDepth4    float32 `json:"4,omitempty"`
	FioDepth8    float32 `json:"8,omitempty"`
	FioDepth16   float32 `json:"16,omitempty"`
	FioDepth32   float32 `json:"32,omitempty"`
	FioDepth64   float32 `json:"64,omitempty"`
	FioDepthGE64 float32 `json:">=64,omitempty"`
}

type FioLatency struct {
	FioLat2      float32 `json:"2,omitempty"`
	FioLat4      float32 `json:"4,omitempty"`
	FioLat10     float32 `json:"10,omitempty"`
	FioLat20     float32 `json:"20,omitempty"`
	FioLat50     float32 `json:"50,omitempty"`
	FioLat100    float32 `json:"100,omitempty"`
	FioLat250    float32 `json:"250,omitempty"`
	FioLat500    float32 `json:"500,omitempty"`
	FioLat750    float32 `json:"750,omitempty"`
	FioLat1000   float32 `json:"1000,omitempty"`
	FioLat2000   float32 `json:"2000,omitempty"`
	FioLatGE2000 float32 `json:">=2000,omitempty"`
}

type FioJobs struct {
	JobName           string        `json:"jobname,omitempty"`
	GroupID           int           `json:"groupid,omitempty"`
	Error             int           `json:"error,omitempty"`
	Eta               int           `json:"eta,omitempty"`
	Elapsed           int           `json:"elapsed,omitempty"`
	JobOptions        FioJobOptions `json:"job options,omitempty"`
	Read              FioStats      `json:"read,omitempty"`
	Write             FioStats      `json:"write,omitempty"`
	Trim              FioStats      `json:"trim,omitempty"`
	Sync              FioStats      `json:"sync,omitempty"`
	JobRuntime        int32         `json:"job_runtime,omitempty"`
	UsrCpu            float32       `json:"usr_cpu,omitempty"`
	SysCpu            float32       `json:"sys_cpu,omitempty"`
	Ctx               int32         `json:"ctx,omitempty"`
	MajF              int32         `json:"majf,omitempty"`
	MinF              int32         `json:"minf,omitempty"`
	IoDepthLevel      FioDepth      `json:"iodepth_level,omitempty"`
	IoDepthSubmit     FioDepth      `json:"iodepth_submit,omitempty"`
	IoDepthComplete   FioDepth      `json:"iodepth_complete,omitempty"`
	LatencyNs         FioLatency    `json:"latency_ns,omitempty"`
	LatencyUs         FioLatency    `json:"latency_us,omitempty"`
	LatencyMs         FioLatency    `json:"latency_ms,omitempty"`
	LatencyDepth      int32         `json:"latency_depth,omitempty"`
	LatencyTarget     int32         `json:"latency_target,omitempty"`
	LatencyPercentile float32       `json:"latency_percentile,omitempty"`
	LatencyWindow     int32         `json:"latency_window,omitempty"`
}

type FioDiskUtil struct {
	Name        string  `json:"name,omitempty"`
	ReadIos     int64   `json:"read_ios,omitempty"`
	WriteIos    int64   `json:"write_ios,omitempty"`
	ReadMerges  int64   `json:"read_merges,omitempty"`
	WriteMerges int64   `json:"write_merges,omitempty"`
	ReadTicks   int64   `json:"read_ticks,omitempty"`
	WriteTicks  int64   `json:"write_ticks,omitempty"`
	InQueue     int64   `json:"in_queue,omitempty"`
	Util        float32 `json:"util,omitempty"`
}

type FioResult struct {
	FioVersion    string           `json:"fio version,omitempty"`
	Timestamp     int64            `json:"timestamp,omitempty"`
	TimestampMS   int64            `json:"timestamp_ms,omitempty"`
	Time          string           `json:"time,omitempty"`
	GlobalOptions FioGlobalOptions `json:"global options,omitempty"`
	Jobs          []FioJobs        `json:"jobs,omitempty"`
	DiskUtil      []FioDiskUtil    `json:"disk_util,omitempty"`
}

func (fio *FIO) TestSingleWorkload() error {
	cmdStr := fmt.Sprintf("fio -filename=%s -direct=1 -iodepth=128 -thread "+
		"-rw=%s -ioengine=libaio -bs=%s -size=%s -numjobs=1 -time_based "+
		"-runtime=60 -group_reporting -name=%s --eta=always --eta-interval=1 "+
		"--eta-newline=1 --output=%s%s --output-format=json", fio.DiskName, fio.RW, fio.BlockSize,
		fio.Size, fio.TestName, fio.OupDir, fio.OupFile)
	_, err := os.Stat(fio.OupDir)
	if errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(fio.OupDir, 0755)
		if err != nil {
			klog.Error(fio.OupDir, " create failed")
			return err
		}
	}
	klog.Info(cmdStr)
	_, err = utils.RunBash(cmdStr)
	if err != nil {
		klog.Error("fio run failed")
		return err
	}

	return nil
}

func (fio *FIO) TestHybridSingleWorkload() error {
	cmdStr := fmt.Sprintf("fio -filename=%s -direct=1 -iodepth=128 -thread "+
		"-rw=%s -rwmixread=%d -ioengine=libaio -bs=%s -size=%s -numjobs=1 -time_based "+
		"-runtime=60 -group_reporting -name=%s --eta=always --eta-interval=1 "+
		"--eta-newline=1 --output=%s%s --output-format=json", fio.DiskName, fio.RW, fio.Ratio, fio.BlockSize,
		fio.Size, fio.TestName, fio.OupDir, fio.OupFile)
	_, err := os.Stat(fio.OupDir)
	if errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(fio.OupDir, 0755)
		if err != nil {
			klog.Error(fio.OupDir, " create failed")
			return err
		}
	}
	klog.Info(cmdStr)
	_, err = utils.RunBash(cmdStr)
	if err != nil {
		klog.Error("fio run failed")
		return err
	}

	return nil
}

func (fio *FIO) ParsingFIOResults(mode string) ([]int64, error) {
	bws := []int64{}
	file := fmt.Sprintf("%s%s", fio.OupDir, fio.OupFile)
	var fioRes FioResult
	data, err := os.ReadFile(file)
	if err != nil {
		klog.Error("read file failed: ", err)
		return []int64{}, err
	}
	err = json.Unmarshal(data, &fioRes)
	if err != nil {
		klog.Error("deserialization failed: ", err)
		return []int64{}, err
	}
	if mode == "SingleWorkload" {
		var bw int64
		if fio.RW[4:] == "read" {
			if len(fioRes.Jobs) > 0 {
				bw = fioRes.Jobs[0].Read.BWBytes
			} else {
				bw = 0
			}
		}
		if fio.RW[4:] == "write" {
			if len(fioRes.Jobs) > 0 {
				bw = fioRes.Jobs[0].Write.BWBytes
			} else {
				bw = 0
			}
		}
		bw /= int64(utils.MiB)
		bw = getMax(bw, 1) // in case meet zero
		bws = append(bws, bw)
	}
	if mode == "HybridSingleWorkload" {
		var rbw, wbw int64
		if len(fioRes.Jobs) > 0 {
			rbw = fioRes.Jobs[0].Read.BWBytes
			wbw = fioRes.Jobs[0].Write.BWBytes
		} else {
			rbw = 0
			wbw = 0
		}

		rbw /= (1024 * 1024)
		wbw /= (1024 * 1024)
		rbw = getMax(rbw, 1)
		wbw = getMax(wbw, 1)
		bws = append(bws, rbw, wbw)
	}
	return bws, nil
}

func getMax(a int64, b int64) int64 {
	if a > b {
		return a
	} else {
		return b
	}
}

// GetDeviceStatus reads disk status from disk profiler result
func GetDiskStatus(filename string) (map[string]int, error) {
	ds := make(map[string]int)
	JsonParse := NewJsonStruct()
	v := Profile{}
	_, err := os.ReadFile(filename)
	if err != nil {
		klog.Warning("0.Read disk profile err: ", err)
		return nil, err
	} else {
		klog.V(utils.INF).Infof("0.GetDiskStatus: v %v", v)
		JsonParse.Load(filename, &v)
		for k, v := range v.Disks {
			ds[k] = v.Status
		}
	}
	return ds, nil
}

func ParseAllDiskProfileForController(filename string, deviceStatus map[string]int) utils.DiskInfos {
	JsonParse := NewJsonStruct()
	v := Profile{}
	JsonParse.Load(filename, &v)
	var profile utils.DiskInfos
	singlepf := make(map[string]utils.DiskInfo)
	for did := range v.Disks {
		if _, ok := deviceStatus[did]; !ok {
			delete(v.Disks, did)
		}
	}
	for dkId := range v.Disks {
		pf := ParseDiskProfileForController(dkId, filename)
		singlepf[dkId] = pf
	}
	profile = singlepf
	return profile
}

func ParseDiskProfileForController(id, filename string) utils.DiskInfo {
	JsonParse := NewJsonStruct()
	v := Profile{}
	var pf utils.DiskInfo
	JsonParse.Load(filename, &v)
	num := 6
	readRatios := []float64{}
	writeRatios := []float64{}

	if _, ok := v.Disks[id]; ok {
		pf.Name = v.Disks[id].Path
		pf.Type = v.Disks[id].Type
		pf.MajorMinor = v.Disks[id].DeviceMM
		pf.MountPoint = v.Disks[id].Mount
		pf.Capacity = strconv.FormatInt(v.Disks[id].Size, 10)
		klog.V(utils.INF).Infof("disk id: %s, pf.Capacity %s", id, pf.Capacity)
		pf.TotalRBPS = v.Disks[id].SingleWorkload[num-1].Read
		pf.TotalWBPS = v.Disks[id].SingleWorkload[num-1].Write
		pf.TotalBPS = v.Disks[id].HybridWorkload.Read + v.Disks[id].HybridWorkload.Write
		for j := 0; j < num; j++ {
			rr_tmp := v.Disks[id].SingleWorkload[num-1].Read / v.Disks[id].SingleWorkload[j].Read
			rr, err := strconv.ParseFloat(fmt.Sprintf("%.2f", rr_tmp), 64)
			if err != nil {
				klog.Errorf("failed to convert throughput %v to float64", rr_tmp)
				continue
			}
			readRatios = append(readRatios, float64(rr))
			wr_tmp := v.Disks[id].SingleWorkload[num-1].Write / v.Disks[id].SingleWorkload[j].Write
			wr, err := strconv.ParseFloat(fmt.Sprintf("%.2f", wr_tmp), 64)
			if err != nil {
				klog.Errorf("failed to convert throughput %v to float64", rr_tmp)
				continue
			}
			writeRatios = append(writeRatios, float64(wr))
		}
		pf.ReadRatio = make(map[string]float64)
		pf.WriteRatio = make(map[string]float64)
		for index, ratio := range readRatios {
			pf.ReadRatio[blockSize[index]] = ratio
		}
		for index, ratio := range writeRatios {
			pf.WriteRatio[blockSize[index]] = ratio
		}
	} else {
		klog.Warningf("No such disk %s\n", id)
	}
	return pf
}

func GetAllDiskProfileForController(deviceStatus map[string]int, filename string) *utils.DiskInfos {
	var pf utils.DiskInfos
	pf = make(utils.DiskInfos)
	_, err := os.ReadFile(filename)
	if err != nil {
		klog.Warning("4.1 no disk profiler file!\n")
		// do not return nil for engine, engine init value is not nil
	} else {
		pf = ParseAllDiskProfileForController(filename, deviceStatus)
	}
	return &pf
}

type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}

func (jst *JsonStruct) Load(filename string, v interface{}) {
	data, err := os.ReadFile(filename)
	if err != nil {
		klog.Errorf("read file %s failed: %v", filename, err)
		return
	}

	err = json.Unmarshal(data, v)
	if err != nil {
		klog.Errorf("unmarshal json: (%v) failed: %v", data, err)
		return
	}
}

// AnalysisDiskProfile gets disk profile config with id
func AnalysisDiskProfile(deviceStatus map[string]int, filename string) *utils.DiskInfos {
	var allProfile agent.AllProfile
	profileConfig := GetAllDiskProfileForController(deviceStatus, filename)
	allProfile.T = 0
	allProfile.DProfile = *profileConfig
	agent.ProfileChan <- &allProfile
	return profileConfig
}

func initDiskCnt(profile *Profile, pDsInfo *map[string]utils.BlockDevice) {
	for did, dv := range *pDsInfo {
		dd := DiskProfiler{
			Path:     dv.Name,
			Id:       did,
			DeviceMM: dv.MajMin,
			Status:   1,
			Type:     dv.Type,
			Mount:    dv.Mount,
			Size:     dv.AvailSize,
			SingleWorkload: []Bandwidth{{Blocksize: "512B", Read: 0, Write: 0}, {Blocksize: "1k", Read: 0, Write: 0},
				{Blocksize: "4k", Read: 0, Write: 0}, {Blocksize: "8k", Read: 0, Write: 0}, {Blocksize: "16k", Read: 0, Write: 0},
				{Blocksize: "32k", Read: 0, Write: 0}},
			HybridWorkload: Bandwidth{50, "32k", 0, 0},
		}
		profile.Disks[did] = dd
	}
}

func addNewDisksToFile(pDsInfo *map[string]utils.BlockDevice, filePath string) error {
	var cnt Profile
	cnt.Disks = make(map[string]DiskProfiler)
	// deserialization
	data, err := os.ReadFile(filePath)
	if err != nil {
		klog.Error("read file failed: ", err)
		return err
	}
	err = json.Unmarshal(data, &cnt)
	if err != nil {
		klog.Error("deserialization failed: ", err)
		return err
	}
	// add data
	initDiskCnt(&cnt, pDsInfo)
	// serialization
	f, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		klog.Error("open file failed: ", err)
		return err
	}
	defer f.Close()
	jsonStr, err := json.MarshalIndent(&cnt, "", "\t")
	if err != nil {
		klog.Error("serialization failed: ", err)
		return err
	}
	_, err = f.Write(jsonStr)
	if err != nil {
		klog.Error("write file failed: ", err)
		return err
	}
	return nil
}

func createDisksToFile(pDsInfo *map[string]utils.BlockDevice, filePath string) error {
	var res Profile
	res.Disks = make(map[string]DiskProfiler)
	res.BaselineBs = "32k"
	res.BwUnit = "MiB/s"
	initDiskCnt(&res, pDsInfo)

	f, err := os.Create(filePath)
	if err != nil {
		klog.Error("create file failed: ", err)
		return err
	}
	defer f.Close()
	jsonStr, err := json.MarshalIndent(&res, "", "\t")
	if err != nil {
		klog.Error("serialization failed: ", err)
		return err
	}
	_, err = f.Write(jsonStr)
	if err != nil {
		klog.Error("write file failed: ", err)
		return err
	}
	return nil
}

func GetNewDisks(pDsInfo *map[string]utils.BlockDevice, filename string) {
	// file not exist, init file
	_, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		_, err = os.Stat(profileDir)
		if errors.Is(err, os.ErrNotExist) {
			err := os.MkdirAll(profileDir, 0644)
			if err != nil {
				klog.Error("create profiler directory failed: ", err)
			}
		}
		err = createDisksToFile(pDsInfo, filename)
		if err != nil {
			klog.Error("createDisksToFile failed")
		}
		klog.V(utils.DBG).Infof("GetNewDisks: file not exit: %v", *pDsInfo)
	}
	// file exist
	JsonParse := NewJsonStruct()
	v := Profile{}
	JsonParse.Load(Filename, &v)
	klog.V(utils.DBG).Infof("GetNewDisks: v.Disks %v", v.Disks)
	for _, dd := range v.Disks {
		if dd.Status == 2 { // profiler complete, so we don't profile it again.
			delete(*pDsInfo, dd.Id)
		}
	}
	klog.V(utils.DBG).Infof("GetNewDisks: %v", *pDsInfo)
	if len(*pDsInfo) != 0 {
		err := addNewDisksToFile(pDsInfo, filename)
		if err != nil {
			klog.Error("addNewDisksToFile failed")
		}
	}
}

func DiskProducer(dsInfo map[string]utils.BlockDevice, ch chan<- utils.BlockDevice) {
	for _, d := range dsInfo {
		ch <- d
	}
	klog.V(utils.INF).Info("diskProducer done")
}

func getFioTestFile(mnt, bs, rw string) string {
	var res string
	if string(mnt[len(mnt)-1]) == "/" {
		res = fmt.Sprintf("%s%s_%s", mnt, bs, rw)
	} else {
		res = fmt.Sprintf("%s/%s_%s", mnt, bs, rw)
	}
	return res
}

func getFioFileSize(sz int64) string {
	sz /= int64(utils.MiB) // GB
	if sz <= 20 {
		return fmt.Sprintf("%s%s", strconv.FormatInt(sz/2, 10), "G")
	} else {
		return "20G"
	}
}

func WriteFailDisk(disk utils.BlockDevice) {
	var fileCnt Profile

	rw.Lock()
	f, err := os.OpenFile(Filename, os.O_RDWR, 0644)
	if err != nil {
		klog.Error("open file failed: ", err)
	}
	defer f.Close()

	data, err := os.ReadFile(Filename)
	if err != nil {
		klog.Error("read file failed: ", err)
	}
	err = json.Unmarshal(data, &fileCnt)
	if err != nil {
		klog.Error("deserialization failed: ", err)
	}

	did := utils.GetDiskId(disk.Model, disk.Serial)
	if item, ok := fileCnt.Disks[did]; ok {
		item.Status = 3 // denote profile done
		fileCnt.Disks[did] = item
	}

	jsonStr, err := json.MarshalIndent(&fileCnt, "", "\t")
	if err != nil {
		klog.Error("serialization failed: ", err)
	}
	_, err = f.Write(jsonStr)
	if err != nil {
		klog.Error("write file failed: ", err)
	}
	// mutex dec
	CurrFIOJobs -= 1
	rw.Unlock()
}

func launchFIO(disk utils.BlockDevice) {
	klog.V(utils.INF).Info("launch a FIO about disk ", disk.Name)
	var fileCnt Profile
	var diskCnt DiskProfiler
	var fio FIO
	diskCnt.SingleWorkload = make([]Bandwidth, 6)
	diskCnt.HybridWorkload = Bandwidth{}
	dSymbolTmp := strings.Split(disk.Name, "/")
	dSymbol := dSymbolTmp[len(dSymbolTmp)-1]
	// mutex inc
	rw.Lock()
	CurrFIOJobs += 1
	rw.Unlock()
	// run profile
	for _, rwi := range []string{"randread", "randwrite"} {
		for bsidx, bsi := range []string{"512B", "1k", "4k", "8k", "16k", "32k"} {
			fio = FIO{
				DiskName:  getFioTestFile(disk.Mount, bsi, rwi),
				RW:        rwi,
				BlockSize: bsi,
				Size:      getFioFileSize(disk.AvailSize),
				TestName:  fmt.Sprintf("%s%s%s", dSymbol, bsi, rwi),
				OupDir:    fmt.Sprintf("%s%s%s/", profileDir, "SingleWorkload/", dSymbol),
				OupFile:   fmt.Sprintf("%s_%s%s", bsi, rwi, ".json"),
			}
			// run fio
			err := fio.TestSingleWorkload()
			if err != nil {
				klog.Error("fio test failed: ", err)
				WriteFailDisk(disk) // status=3-fail
				return
			}
			klog.Infof("Profile job %s done", fio.TestName)
			// read result
			bw, err1 := fio.ParsingFIOResults("SingleWorkload")
			if err1 != nil {
				klog.Error("parsing fio result failed: ", err)
				WriteFailDisk(disk) // status=3-fail
				return
			}
			// store to mem
			if rwi == "randread" {
				diskCnt.SingleWorkload[bsidx].Read = float64(bw[0])
			}
			if rwi == "randwrite" {
				diskCnt.SingleWorkload[bsidx].Write = float64(bw[0])
			}
			// del test file
			if err = os.Remove(getFioTestFile(disk.Mount, bsi, rwi)); err != nil {
				klog.Warning("remove fio test file failed: ", err)
			}
			time.Sleep(1 * time.Second)
		}
	}
	// run hybrid workload
	fio = FIO{
		DiskName:  getFioTestFile(disk.Mount, "32k", "randrw"),
		RW:        "randrw",
		Ratio:     50,
		BlockSize: "32k",
		Size:      getFioFileSize(disk.AvailSize),
		TestName:  fmt.Sprintf("%s%s", dSymbol, "32krandrw"),
		OupDir:    fmt.Sprintf("%s%s%s/", profileDir, "HybridSingleWorkload/", dSymbol),
		OupFile:   "32k_randrw.json",
	}
	err2 := fio.TestHybridSingleWorkload()
	if err2 != nil {
		klog.Error("fio test failed: ", err2)
		WriteFailDisk(disk) // status=3-fail
		return
	}

	bw, err := fio.ParsingFIOResults("HybridSingleWorkload")
	if err != nil {
		klog.Error("parsing fio result failed: ", err)
		WriteFailDisk(disk) // status=3-fail
		return
	}
	// store to mem
	diskCnt.HybridWorkload.Read = float64(bw[0])
	diskCnt.HybridWorkload.Write = float64(bw[1])
	// del test file
	if err = os.Remove(getFioTestFile(disk.Mount, "32k", "randrw")); err != nil {
		klog.Warning("remove fio test file failed: ", err)
	}
	// update file with bw
	rw.Lock()
	f, err := os.OpenFile(Filename, os.O_RDWR, 0644)
	if err != nil {
		klog.Error("open file failed: ", err)
	}
	defer f.Close()

	data, err := os.ReadFile(Filename)
	if err != nil {
		klog.Error("read file failed: ", err)
	}
	err = json.Unmarshal(data, &fileCnt)
	if err != nil {
		klog.Error("deserialization failed: ", err)
	}

	did := utils.GetDiskId(disk.Model, disk.Serial)
	if item, ok := fileCnt.Disks[did]; ok {
		for idx := range item.SingleWorkload {
			item.SingleWorkload[idx].Read = diskCnt.SingleWorkload[idx].Read
			item.SingleWorkload[idx].Write = diskCnt.SingleWorkload[idx].Write
		}
		item.HybridWorkload.Read = diskCnt.HybridWorkload.Read
		item.HybridWorkload.Write = diskCnt.HybridWorkload.Write
		item.Status = 2 // denote profile done
		fileCnt.Disks[did] = item
	}

	jsonStr, err := json.MarshalIndent(&fileCnt, "", "\t")
	if err != nil {
		klog.Error("serialization failed: ", err)
	}
	_, err = f.Write(jsonStr)
	if err != nil {
		klog.Error("write file failed: ", err)
	}
	// mutex dec
	CurrFIOJobs -= 1
	rw.Unlock()
}

func DiskConsumer(ch <-chan utils.BlockDevice) {
	for {
		dd, ok := <-ch
		if !ok {
			return // have used all the data from channel
		}
		for {
			rw.Lock()
			nums := CurrFIOJobs
			rw.Unlock()
			if nums < MaxCurrFIOJobs {
				go launchFIO(dd)
				break
			} else {
				time.Sleep(5 * time.Minute)
			}
		}
	}
}
