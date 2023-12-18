/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/IOIsolation/pkg/agent/common"
)

var de = &DiskEngine{
	agent.IOEngine{
		Flag:          0,
		ExecutionFlag: agent.ProfileFlag | agent.PolicyFlag | agent.AdminFlag,
	},
	utils.DiskInfos{},
	make(map[string]struct{}),
	DiskMetric{
		Cache:    make(map[string]NodeDiskBw),
		PodCache: make(map[string]*PodBw),
	},
	DiskMetric{
		Cache:    make(map[string]NodeDiskBw),
		PodCache: make(map[string]*PodBw),
	},
	v1.ResourceConfigSpec{},
	0,
	1,
	20,
	make(map[string]*PodRegInfo),
}

var RdRatio = map[string]float64{
	"512b": 32,
	"1k":   28.35,
	"4k":   5,
	"8k":   1.4,
	"16K":  1.12,
	"32K":  1,
}
var WrRatio = map[string]float64{
	"512b": 32,
	"1k":   28.35,
	"4k":   5,
	"8k":   1.4,
	"16K":  1.12,
	"32K":  1,
}

var c = agent.GetAgent()

var podData = &agent.PodData{
	T:          0,
	Generation: 10,
}

var gDiskId = "test"

/*
var policyData = &agent.PolicyConfig{
	"DiskGArules": {
		Throttle:          "",
		Select_method:     "All",
		Compress_method:   "",
		Decompress_method: "",
		Report_Flag:       "",
	},
	"DiskBTrules": {
		Throttle:          "",
		Select_method:     "All",
		Compress_method:   "",
		Decompress_method: "",
		Report_Flag:       "",
	},
	"DiskBErules": {
		Throttle:          "",
		Select_method:     "",
		Compress_method:   "",
		Decompress_method: "",
		Report_Flag:       "",
	},
	"test": {
		Throttle:          "",
		Select_method:     "",
		Compress_method:   "",
		Decompress_method: "",
		Report_Flag:       "",
	},
}
*/

var profileData = &agent.AllProfile{
	DProfile: utils.DiskInfos{
		"test": utils.DiskInfo{
			Name:       "test",
			Type:       "noemptydir",
			MajorMinor: "[8:0]",
			MountPoint: "/dev/test",
			Capacity:   "1000",
			TotalRBPS:  1000,
			TotalWBPS:  1000,
			TotalBPS:   1500,
			ReadRatio:  RdRatio,
			WriteRatio: WrRatio,
		},
		"test1": utils.DiskInfo{
			Name:       "test1",
			Type:       "emptyDir",
			MajorMinor: "[8:1]",
			MountPoint: "/dev/test1",
			Capacity:   "1000",
			TotalRBPS:  1000,
			TotalWBPS:  1000,
			TotalBPS:   1500,
			ReadRatio:  RdRatio,
			WriteRatio: WrRatio,
		},
	},
}

var adminData = &agent.AdminResourceConfig{
	DiskBePool:  20,
	DiskSysPool: 100,
	NamespaceWhitelist: map[string]struct{}{
		"ioi-system":       {},
		"kube-system":      {},
		"kube-flannel":     {},
		"calico-apiserver": {},
		"calico-system":    {},
		"tigera-operator":  {},
	},
	SysDiskRatio: 20,
	Loglevel: agent.IOILogLevel{
		NodeAgentInfo:   true,
		NodeAgentDebug:  true,
		SchedulerInfo:   true,
		SchedulerDebug:  true,
		AggregatorInfo:  true,
		AggregatorDebug: true,
		ServiceInfo:     true,
		ServiceDebug:    true,
	},
}

func init_de() {
	err := c.InitStaticCR()
	if err != nil {
		return
	}
	de.InitializeWorkloadGroup(common.DiskWorkloadType)
	var diskPolicy agent.Policy

	DiskInfos[gDiskId] = utils.DiskInfo{
		Name:       "test",
		MajorMinor: "[8:0]",
		TotalBPS:   1500,
		TotalRBPS:  1000,
		TotalWBPS:  1000,
		ReadRatio:  RdRatio,
		WriteRatio: WrRatio,
		MountPoint: "/dev/test",
	}

	diskPolicy.Throttle = "POOL_SIZE"
	diskPolicy.Select_method = "All"
	diskPolicy.Compress_method = "CALC_REQUEST_RATIO"
	diskPolicy.Decompress_method = "CALC_REQUEST_RATIO"
	de.GpPolicy = make([]*agent.Policy, utils.GroupIndex)
	de.GpPolicy[utils.GaIndex] = &diskPolicy
	de.GpPolicy[utils.BtIndex] = &diskPolicy
	de.GpPolicy[utils.BeIndex] = &diskPolicy
	for i := utils.GaIndex; i < utils.GroupIndex; i++ {
		wg := agent.WorkloadGroup{
			IoType:        common.NetWorkloadType,
			GroupSettable: false,
			WorkloadType:  i,
			LimitPods:     make(map[string]*utils.Workload),
			ActualPods:    make(map[string]*utils.Workload),
			RequestPods:   make(map[string]*utils.Workload),
			PodsLimitSet:  make(map[string]*utils.Workload),
			RawRequest:    make(map[string]*utils.Workload),
			RawLimit:      make(map[string]*utils.Workload),
			PodName:       make(map[string]string),
		}
		de.AddWorkloadGroup(wg, gDiskId)
	}
	de.NSWhitelist = make(map[string]struct{})
	ns := []string{"ioi-system", "kube-system", "kube-flannel", "calico-apiserver", "calico-system", "tigera-operator"}
	nsLen := len(ns) - 1
	for i := 0; i < nsLen; i++ {
		de.NSWhitelist[ns[i]] = struct{}{}
	}
	de.NodeName = "ioi-1"
	diskBw := DiskBw{
		DiskRequestBw: []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
		DiskAllocBw:   []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
		DiskTotalBw:   []*utils.Workload{{InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}},
	}
	de.EngineCache.Cache["ioi-1"] = NodeDiskBw{
		"test": &diskBw,
	}

	reqBw := make(map[string]PodInfos)
	limBw := make(map[string]PodInfos)
	tranBw := make(map[string]PodInfos)
	untranBw := make(map[string]PodInfos)

	de.EngineCache.PodCache["ioi-1"] = &PodBw{
		PodRequestBw: reqBw,
		PodLimitBw:   limBw,
		PodTranBw:    tranBw,
		PodRawBw:     untranBw,
	}

	de.MetricCache.Cache["ioi-1"] = NodeDiskBw{
		"test": &diskBw,
	}

	diskinfo4k := &utils.Workload{
		InBps:    2,
		OutBps:   3,
		TotalBps: 5,
	}
	diskinfo8k := &utils.Workload{
		InBps:    2,
		OutBps:   3,
		TotalBps: 5,
	}

	dskInfoBs := make(map[string]*utils.Workload)
	dskInfoBs["4k"] = diskinfo4k
	dskInfoBs["8k"] = diskinfo8k

	dskData := make(map[string]map[string]*utils.Workload) // diskId
	dskData[gDiskId] = dskInfoBs
	podEvents := make(map[string]agent.PodEventInfo)
	wl := utils.Workload{InBps: 1, OutBps: 2}
	podEvents[gDiskId] = agent.PodEventInfo{
		"pod1": agent.PodRunInfo{
			Operation:   1,
			PodName:     "pod1",
			RawActualBs: &dskInfoBs,
			Actual:      &wl,
			Request:     &wl,
			Limit:       &wl,
			RawRequest:  &wl,
			RawLimit:    &wl,
			Namespace:   "default",
		},
		"pod2": agent.PodRunInfo{
			Operation:   1,
			PodName:     "pod2",
			RawActualBs: &dskInfoBs,
			Actual:      &wl,
			Request:     &wl,
			Limit:       &wl,
			RawRequest:  &wl,
			RawLimit:    &wl,
			Namespace:   "ioi-system",
		},
	}

	podData.PodEvents = podEvents
}

/*
	func TestDiskEngine_ProcessPolicy(t *testing.T) {
		init_de()
		type args struct {
			data agent.EventData
		}
		tests := []struct {
			name    string
			args    args
			wantErr bool
		}{
			{
				name:    "test process policy",
				args:    args{data: policyData},
				wantErr: false,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				if err := de.ProcessPolicy(tt.args.data); (err != nil) != tt.wantErr {
					t.Errorf("DiskEngine.ProcessPolicy() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	}
*/
func TestDiskEngine_ProcessAdmin(t *testing.T) {
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test process admin",
			args:    args{data: adminData},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_de()
			if err := de.ProcessAdmin(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.ProcessAdmin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDiskEngine_ProcessNriData(t *testing.T) {
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test process nri data",
			args:    args{data: podData},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_de()
			if err := de.ProcessNriData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.ProcessNriData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetMountPath(t *testing.T) {
	type args struct {
		deviceID string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "test get exist mount path",
			args:    args{deviceID: "test"},
			want:    "/dev/test",
			wantErr: false,
		},
		{
			name:    "test get not exist mount path",
			args:    args{deviceID: "test1"},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_de()
			got, err := GetMountPath(tt.args.deviceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMountPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetMountPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalysisDiskStatus(t *testing.T) {
	type args struct {
		DeviceStatus map[string]int
	}

	devStatus := make(map[string]int)
	devStatus["test"] = 2
	devStatus1 := make(map[string]int)
	devStatus1["test"] = 1

	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name:    "test analysis disk status is 2",
			args:    args{DeviceStatus: devStatus},
			want:    true,
			wantErr: false,
		},
		{
			name:    "test analysis disk status is 1",
			args:    args{DeviceStatus: devStatus1},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AnalysisDiskStatus(tt.args.DeviceStatus)
			if (err != nil) != tt.wantErr {
				t.Errorf("AnalysisDiskStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("AnalysisDiskStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiskEngine_ProcessProfile(t *testing.T) {
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test process profile",
			args:    args{data: profileData},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_de()
			if err := de.ProcessProfile(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.ProcessProfile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// to be fix unit test

func TestDiskEngine_ProcessDiskCrData(t *testing.T) {
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test disk cr",
			args:    args{data: podData},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_de()
			DiskInfos = map[string]utils.DiskInfo{}
			if err := de.ProcessDiskCrData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.ProcessDiskCrData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDiskEngine_ProcessDiskIOData(t *testing.T) {
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test process disk io data",
			args:    args{data: podData},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_de()
			if err := de.ProcessDiskIOData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.ProcessDiskIOData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDiskEngine_calDiskAllocatable(t *testing.T) {
	init_de()

	tests := []struct {
		name  string
		want  []float64
		want1 []float64
		want2 []float64
		want3 []bool
	}{
		{
			name:  "Disk Test1",
			want:  []float64{595.0, 595.0, 300.0},
			want1: []float64{595.0, 595.0, 300.0},
			want2: []float64{840.0, 840.0, 450.0},
			want3: []bool{false, false, false},
		},
		{
			name:  "Disk Test2",
			want:  []float64{795.0, 795.0, 200.0},
			want1: []float64{795.0, 795.0, 200.0},
			want2: []float64{1190.0, 1190.0, 300},
			want3: []bool{false, false, false},
		},
		{
			name:  "Disk Test3",
			want:  []float64{795.0, 795.0, 100},
			want1: []float64{795.0, 795.0, 100},
			want2: []float64{1140.0, 1140.0, 150},
			want3: []bool{false, false, false},
		},
	}
	for index, tt := range tests {
		switch index {
		case 0:
			de.resourceConfig.GAPool = 60
			de.resourceConfig.BEPool = 30
			wl1 := utils.Workload{
				InBps:  100.0,
				OutBps: 100.0,
			}
			wl2 := utils.Workload{
				InBps:  5.0,
				OutBps: 5.0,
			}
			de.WgDevices[gDiskId][utils.GaIndex].LimitPods["pod1"] = &wl1
			de.WgDevices[gDiskId][utils.GaIndex].RequestPods["pod1"] = &wl1
			de.WgDevices[gDiskId][utils.GaIndex].RawRequest["pod1"] = &wl1
			de.WgDevices[gDiskId][utils.BtIndex].LimitPods["pod2"] = &wl1
			de.WgDevices[gDiskId][utils.BtIndex].RequestPods["pod2"] = &wl2
			de.WgDevices[gDiskId][utils.BtIndex].RawRequest["pod2"] = &wl1
		case 1:
			de.resourceConfig.GAPool = 80
			de.resourceConfig.BEPool = 20
			wl1 := utils.Workload{
				InBps:  0.0,
				OutBps: 0.0,
			}
			wl2 := utils.Workload{
				InBps:  5.0,
				OutBps: 5.0,
			}
			wl3 := utils.Workload{
				InBps:  100.0,
				OutBps: 100.0,
			}

			de.WgDevices[gDiskId][utils.GaIndex].LimitPods["pod1"] = &wl1
			de.WgDevices[gDiskId][utils.GaIndex].RequestPods["pod1"] = &wl1
			de.WgDevices[gDiskId][utils.GaIndex].RawRequest["pod1"] = &wl1
			de.WgDevices[gDiskId][utils.BtIndex].LimitPods["pod2"] = &wl3
			de.WgDevices[gDiskId][utils.BtIndex].RequestPods["pod2"] = &wl2
			de.WgDevices[gDiskId][utils.BtIndex].RawRequest["pod2"] = &wl1
		case 2:
			de.resourceConfig.GAPool = 100
			de.resourceConfig.BEPool = 10
			wl1 := utils.Workload{
				InBps:  100.0,
				OutBps: 100.0,
			}
			wl2 := utils.Workload{
				InBps:  450.0,
				OutBps: 450.0,
			}
			wl3 := utils.Workload{
				InBps:  5.0,
				OutBps: 5.0,
			}
			de.WgDevices[gDiskId][utils.GaIndex].LimitPods["pod1"] = &wl2
			de.WgDevices[gDiskId][utils.GaIndex].RequestPods["pod1"] = &wl1
			de.WgDevices[gDiskId][utils.GaIndex].RawRequest["pod1"] = &wl1
			de.WgDevices[gDiskId][utils.BtIndex].LimitPods["pod2"] = &wl1
			de.WgDevices[gDiskId][utils.BtIndex].RequestPods["pod2"] = &wl3
			de.WgDevices[gDiskId][utils.BtIndex].RawRequest["pod2"] = &wl1

		}
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, got3 := de.calDiskAllocatable(gDiskId)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("calDiskLimitAndAlloc() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("calDiskLimitAndAlloc() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("calDiskLimitAndAlloc() got2 = %v, want %v", got2, tt.want2)
			}
			if !reflect.DeepEqual(got3, tt.want3) {
				t.Errorf("calDiskLimitAndAlloc() got3 = %v, want %v", got3, tt.want3)
			}
		})
	}
}

func TestStartDiskProfiler(t *testing.T) {
	type args struct {
		disksInput map[string][]string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test start disk profiler",
			args: args{
				disksInput: map[string][]string{
					"test": {"8:0"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			patch := gomonkey.ApplyFunc(common.IOIGetServiceInfoDisk, func(_ string) *agent.AllProfile {
				a := agent.AllProfile{
					DInfo: agent.AllDiskInfo{
						"test": utils.BlockDevice{
							Name:   "test",
							Type:   "noemptydir",
							MajMin: "8:0",
							Mount:  "/dev/test",
						},
					},
				}
				return &a
			})
			defer patch.Reset()
			StartDiskProfiler(tt.args.disksInput)
		})
	}
}
