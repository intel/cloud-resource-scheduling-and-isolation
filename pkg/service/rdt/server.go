/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"

	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
	"sigs.k8s.io/IOIsolation/pkg/service"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
	"sigs.k8s.io/yaml"

	_ "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/engine"
	_ "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/executor"
	_ "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/monitor"
	_ "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/policy"
	_ "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/watcher"
)

const (
	schemata         = "schemata"
	TableName string = "RDT"
)

type JsonStruct struct {
}

type AppInfo struct {
	AppName    string           `json:"appname"`
	RdtAppInfo utils.RDTAppInfo `json:"rdtappinfo"`
}

type RdtEngine struct {
	service.ServiceEngine
	persist    common.IPersist
	Apps       map[string]*AppInfo      `json:"apps"`
	Interval   utils.IntervalConfigInfo `json:"interval"`
	config     RDTConfig
	ConfigPath string
	eng        rdt.Engine
}

func (r *RdtEngine) Type() int32 {
	return utils.RDTIOIType
}

func (r *RdtEngine) Initialize(persist common.IPersist) error {
	r.persist = persist
	err := r.persist.CreateTable(TableName, []string{"APPS"})
	if err != nil {
		klog.Warningf("Create rdt buckets fail: %s", err)
	}

	r.Apps = make(map[string]*AppInfo)
	r.ConfigPath = viper.GetString("rdt.config-path")

	// for interval
	r.Interval = utils.IntervalConfigInfo{
		Interval: 2,
	}
	configInterval, err1 := LoadInterval(r)
	if err1 != nil {
		klog.Warningf("Get interval failed: ", err1)
	} else {
		r.Interval = configInterval
	}

	if err := r.loadRDTConfig(); err != nil {
		klog.Fatalf("Failed to load rdt config: %v", err)
		return err
	}
	if resctrlL3Info, err = getRdtL3Info(); err != nil {
		klog.Fatalf("Failed to get rdt l3 info: %v", err)
		return err
	}

	r.Apps, err = GetAllAppInfos(r, "")
	if err != nil {
		klog.Fatalf("Failed to read applist %v", err)
		return err
	}

	return nil
}

func (r *RdtEngine) Uninitialize() error {
	klog.V(utils.INF).Info("persistent: file close")
	return nil
}

func (r *RdtEngine) Configure(ctx context.Context, req *pb.ConfigureRequest) (*pb.ConfigureResponse, error) {
	return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, status.Error(codes.InvalidArgument, "the Req type is invalid")
}

func (r *RdtEngine) RegisterApp(ctx context.Context, req *pb.RegisterAppRequest) (*pb.RegisterAppResponse, error) {
	if _, OK := r.Apps[req.AppName]; OK {
		return nil, status.Error(codes.AlreadyExists, "App Already Registered")
	}
	app, err := parseApp2String(req.AppInfo)
	if err != nil {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, err
	}
	if req.AppName == "" || app.RdtAppInfo.CgroupPath == "" || app.RdtAppInfo.ContainerName == "" {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("input information for RegisterApp is not complete. ContainerId, ContainerName and ContainerCgroupPath are required")
	}

	_, err = os.Stat(app.RdtAppInfo.CgroupPath)
	if err != nil {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("CgroupPath is invalid or does not exist")
	}

	_, err = os.Readlink(app.RdtAppInfo.CgroupPath)
	if err == nil {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("CgroupPath is a symbolic link, which is not allowed")
	}

	a := AppInfo{
		AppName:    req.AppName,
		RdtAppInfo: app.RdtAppInfo,
	}

	klog.V(utils.INF).Infof("RegisterApp: %+v", a)

	r.Apps[req.AppName] = &a
	err = AddAppInfo(r, a)
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

func (r *RdtEngine) UnRegisterApp(ctx context.Context, req *pb.UnRegisterAppRequest) (*pb.UnRegisterAppResponse, error) {
	if _, OK := r.Apps[req.AppId]; !OK {
		return nil, status.Error(codes.NotFound, "App Not Found")
	}

	klog.V(utils.INF).Infof("UnRegisterApp: %+v", r.Apps[req.AppId])
	err := DeleteAppInfo(r, r.Apps[req.AppId].AppName)
	if err != nil {
		klog.Errorf("update app file %s", err)
	}
	delete(r.Apps, req.AppId)

	return &pb.UnRegisterAppResponse{}, nil
}

func parseApp2String(mockString string) (AppInfo, error) {
	var app AppInfo
	if err := json.Unmarshal([]byte(mockString), &app); err != nil {
		klog.Warning("Parse string failed because string format is wrong")
		return app, err
	}
	return app, nil
}

func (s *RdtEngine) loadRDTConfig() error {
	config, err := os.ReadFile(s.ConfigPath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(config, &s.config)
	return err
}

func (s *RdtEngine) checkRDTConfig() error {
	// check rdt config is right
	classMap := make(map[string]struct{})
	for _, class := range s.config.Classes {
		classpath := filepath.Join(resctrlL3Info.resctrlPath, class)
		if _, err := os.Stat(classpath); err != nil {
			return fmt.Errorf("%v does not exist: %v", classpath, err)
		}
		classMap[class] = struct{}{}
	}
	for k, v := range s.config.Adjustments {
		if v.Threshold.CpuUtilization == nil && v.Threshold.L3CacheMiss == nil {
			return fmt.Errorf("adjuestment %v does not have threshold", k)
		}
		if v.Threshold.CpuUtilization != nil {
			for class := range v.Threshold.CpuUtilization {
				if _, ok := classMap[class]; !ok {
					return fmt.Errorf("rdt class %v does not exist", class)
				}
			}
		}
		if v.Threshold.L3CacheMiss != nil {
			for class := range v.Threshold.L3CacheMiss {
				if _, ok := classMap[class]; !ok {
					return fmt.Errorf("rdt class %v does not exist", class)
				}
			}
		}
		for class, l3 := range v.AR.L3 {
			if _, ok := classMap[class]; !ok {
				return fmt.Errorf("rdt class %v does not exist", class)
			}
			// check whether l3 is valid
			if err := checkBitmask(l3); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *RdtEngine) monitorAndAdjust() {
	klog.V(utils.INF).Infof("RDT Config: %+v", s.config)

	oldAdjustment := ""
	for {
		klog.V(utils.INF).Infof("---------------Load Metrics and Adjust Resources------------------")
		flag, adjustment, err := s.findAppropriateAdjustment()
		if err != nil {
			klog.Fatalf("Can not find appropriate adjustment: %v", err)
		}
		if !flag && oldAdjustment == "" {
			klog.V(utils.INF).Infof("Donot need to adjust resource. Resources have not been adjusted yet.")
		} else if !flag && oldAdjustment != "" || flag && adjustment == oldAdjustment {
			klog.V(utils.INF).Infof("Donot need to adjust resource. The current adjustment is %v", oldAdjustment)
		} else {
			klog.V(utils.INF).Infof("Need to adjust resource to %v.", adjustment)
			oldAdjustment = adjustment
			if err := s.adjustResource(adjustment); err != nil {
				klog.Fatalf("Can not adjust resource: %v", err)
			}
		}
		klog.V(utils.INF).Infof("------------------------------------------------------------------")
		time.Sleep(time.Duration(s.config.UpdateInterval) * time.Second)
	}
}

func (s *RdtEngine) findAppropriateAdjustment() (bool, string, error) {
	// collect metrics and decide whether and which adjustment to make.
	cpus := make(map[string]float64, 0)
	l3misses := make(map[string]float64, 0)
	for _, class := range s.config.Classes {
		ans, err := s.getRDTClassCPUUtilization(class)
		if err != nil {
			return false, "", err
		}
		cpus[class] = ans
		ans, err = s.getRDTClassL3CacheMiss(class)
		if err != nil {
			return false, "", err
		}
		l3misses[class] = ans
		klog.V(utils.DBG).Infof("rdt class %v cpu utilization: %v, l3 cache miss: %v", class, cpus[class], l3misses[class])
	}
	for k, v := range s.config.Adjustments {
		flag := true
		if v.Threshold.CpuUtilization != nil {
			for class, cpu := range v.Threshold.CpuUtilization {
				if cpus[class] < cpu {
					flag = false
					break
				}
			}
		}
		if flag && v.Threshold.L3CacheMiss != nil {
			for class, l3miss := range v.Threshold.L3CacheMiss {
				if l3misses[class] < l3miss {
					flag = false
					break
				}
			}
		}
		if flag {
			return true, k, nil
		}
	}
	return false, "", nil
}

func (s *RdtEngine) adjustResource(adjustment string) error {
	if err := s.adjustL3Cache(adjustment); err != nil {
		return fmt.Errorf("can not adjust L3 cache: %v", err)
	}
	return nil
}

func (s *RdtEngine) adjustL3Cache(adjustment string) error {
	for class, l3 := range s.config.Adjustments[adjustment].AR.L3 {
		if err := modifyRDTClassL3schema(class, l3); err != nil {
			return err
		}
	}
	return nil
}

func (s *RdtEngine) Subscribe(req *pb.SubscribeContext, stream pb.Ioiservice_SubscribeServer) error {
	for {
		curData, err := s.getRDTclassScore()
		if err != nil {
			return err
		}
		klog.V(utils.DBG).Info("curData: ", curData)
		err = send(stream, req.IoiType, curData)
		if err != nil {
			return err
		}
		time.Sleep(time.Duration(s.config.UpdateInterval) * time.Second)
	}
}

func (s *RdtEngine) getRDTclassScore() (map[string]float64, error) {
	// get rdt classes score
	curData := make(map[string]float64, 0)
	for _, class := range s.config.Classes {
		v, err := s.getRDTClassL3CacheMiss(class)
		if err != nil {
			return curData, err
		}
		curData[class] = v
	}
	return curData, nil
}

func send(ss pb.Ioiservice_SubscribeServer, ioiType int32, data map[string]float64) error {
	klog.V(utils.INF).Info("Send rdt classes score.")
	instance := createMsg(ioiType, data)
	err := ss.Send(instance)
	if err != nil {
		klog.Warning("send failed", err)
	}
	return err
}

func createMsg(ioiType int32, data map[string]float64) *pb.AppsBandwidth {
	if ioiType == 3 {
		msg := pb.AppsBandwidth{
			IoiType: ioiType,
			Payload: &pb.AppsBandwidth_RdtInfo{
				RdtInfo: &pb.RDTInfo{
					ClassInfo: data,
				}},
		}
		return &msg
	}
	return nil
}

func init() {
	a := service.GetService()
	if (a.EnginesSwitch & service.RDTSwitch) == 0 {
		return
	}

	rdtConfig := "/etc/ioi-service/fixedclass.yaml"
	factory, err := rdt.NewFactoryFromFile(rdtConfig)
	if err != nil {
		klog.Errorf("err %v", err)
	}

	eng, err := factory.GetEngine("FixedClass")
	if err != nil {
		klog.Fatal("start engine error, %v", err)
	}
	go eng.Start()

	engine := &RdtEngine{
		service.ServiceEngine{},
		nil,
		make(map[string]*AppInfo),
		utils.IntervalConfigInfo{
			Interval: 1,
		},
		RDTConfig{},
		"",
		eng,
	}

	a.RegisterEngine(engine)
}
