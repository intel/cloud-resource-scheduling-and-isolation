/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"errors"
	"reflect"
	"testing"

	"k8s.io/client-go/kubernetes"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/IOIsolation/pkg/agent/common"
)

var nde = &NetEngine{
	agent.IOEngine{
		Flag:          0,
		ExecutionFlag: agent.ProfileFlag | agent.PolicyFlag | agent.AdminFlag,
	},
	utils.NicInfos{},
	make(map[string]struct{}),
	0,
	20,
	NetMetric{
		Cache:    make(map[string]NodeNetBw),
		PodCache: make(map[string]*PodBw),
	},
	NetMetric{
		Cache:    make(map[string]NodeNetBw),
		PodCache: make(map[string]*PodBw),
	},
	v1.ResourceConfigSpec{},
	make(map[string]*PodRegInfo),
}

var c = agent.GetAgent()

var podData = &agent.PodData{
	T:          0,
	Generation: 10,
}

var gNicId = "12:34:45:56:67:78"

var policyData = &agent.PolicyConfig{
	"NetGArules": {
		Throttle:          "",
		Select_method:     "",
		Compress_method:   "",
		Decompress_method: "",
		Report_Flag:       "",
	},
	"NetBTrules": {
		Throttle:          "",
		Select_method:     "",
		Compress_method:   "",
		Decompress_method: "",
		Report_Flag:       "",
	},
	"NetBErules": {
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

/*
var profileData = &agent.AllProfile{
	NProfile: utils.NicInfos{
		"test": utils.NicInfo{
			Name:          "test",
			Type:          "masterNic",
			IpAddr:        "[8:0]",
			TotalBPS:      1500,
			QueueNum:      10,
			GroupSettable: true,
		},
		"test1": utils.NicInfo{
			Name:          "test1",
			Type:          "adq",
			IpAddr:        "[8:0]",
			TotalBPS:      1500,
			QueueNum:      10,
			GroupSettable: true,
		},
	},
}
*/

var adminData = &agent.AdminResourceConfig{
	DiskBePool:  20,
	DiskSysPool: 20,
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

func init_net_de() {
	err := c.InitStaticCR()
	if err != nil {
		return
	}
	nde.InitializeWorkloadGroup(common.DiskWorkloadType)
	var netPolicy agent.Policy

	NicInfos[gNicId] = utils.NicInfo{
		IpAddr:      "192.168.2.1",
		Name:        "eno1",
		RcvGroupBPS: 1000,
		SndGroupBPS: 1000,
	}

	netPolicy.Throttle = "POOL_SIZE"
	netPolicy.Select_method = "All"
	netPolicy.Compress_method = "CALC_REQUEST_RATIO"
	netPolicy.Decompress_method = "CALC_REQUEST_RATIO"
	nde.GpPolicy = make([]*agent.Policy, utils.GroupIndex)
	nde.GpPolicy[utils.GaIndex] = &netPolicy
	nde.GpPolicy[utils.BtIndex] = &netPolicy
	nde.GpPolicy[utils.BeIndex] = &netPolicy
	for i := utils.GaIndex; i < utils.GroupIndex; i++ {
		wg := agent.WorkloadGroup{
			IoType:        common.NetWorkloadType,
			GroupSettable: false,
			WorkloadType:  i,
			LimitPods:     make(map[string]*utils.Workload),
			ActualPods:    make(map[string]*utils.Workload),
			RequestPods:   make(map[string]*utils.Workload),
			PodsLimitSet:  make(map[string]*utils.Workload),
			PodName:       make(map[string]string),
		}
		nde.AddWorkloadGroup(wg, gNicId)
	}

	nde.NSWhitelist = make(map[string]struct{})
	ns := []string{"ioi-system", "kube-system", "kube-flannel", "calico-apiserver", "calico-system", "tigera-operator"}
	nsLen := len(ns) - 1
	for i := 0; i < nsLen; i++ {
		nde.NSWhitelist[ns[i]] = struct{}{}
	}
	nde.NodeName = "ioi-1"
	nde.EngineCache.Cache["ioi-1"] = NodeNetBw{
		"test": &NetBw{
			NetRequestBw: []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
			NetAllocBw:   []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
			NetTotalBw:   []*utils.Workload{{InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}},
		},
	}

	reqBw := make(map[string]PodInfos)
	limBw := make(map[string]PodInfos)
	tranBw := make(map[string]PodInfos)

	nde.EngineCache.PodCache["ioi-1"] = &PodBw{
		PodRequestBw: reqBw,
		PodLimitBw:   limBw,
		PodTranBw:    tranBw,
	}

	nde.MetricCache.Cache["ioi-1"] = NodeNetBw{
		"test": &NetBw{
			NetRequestBw: []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
			NetAllocBw:   []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
			NetTotalBw:   []*utils.Workload{{InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}},
		},
	}

	podInfos1 := make(map[string]agent.PodEventInfo)

	wl := utils.Workload{InBps: 1, OutBps: 2}

	podInfos1[gNicId] = agent.PodEventInfo{
		"pod1": agent.PodRunInfo{
			Operation: 1,
			PodName:   "pod1",
			Actual:    &wl,
			Limit:     &wl,
			Request:   &wl,
			Namespace: "default",
		},
		"pod2": agent.PodRunInfo{
			Operation: 1,
			PodName:   "pod2",
			Actual:    &wl,
			Limit:     &wl,
			Request:   &wl,
			Namespace: "ioi-system",
		},
	}

	podData.PodEvents = podInfos1
}

func TestNetEngine_ProcessNriData(t *testing.T) {
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
			init_net_de()
			if err := nde.ProcessNriData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.ProcessNriData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNetEngine_ProcessAdmin(t *testing.T) {
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
			init_net_de()
			if err := nde.ProcessAdmin(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.ProcessAdmin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNetEngine_ProcessPolicy(t *testing.T) {
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
			init_net_de()
			if err := nde.ProcessPolicy(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.ProcessPolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

/*
func TestNetEngine_ProcessProfile(t *testing.T) {
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
			init_net_de()
			if err := nde.ProcessProfile(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.ProcessProfile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
*/

// to be fix unit test
func TestNetEngine_ProcessNicIOData(t *testing.T) {
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
			init_net_de()
			if err := nde.ProcessNicIOData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.ProcessNicIOData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNetEngine_ProcessNetworkCrData(t *testing.T) {
	init_net_de()
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test net cr",
			args:    args{data: podData},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := nde.ProcessNetworkCrData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.ProcessNetworkCrData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNetEngine_calNicAllocatable(t *testing.T) {
	init_net_de()
	tests := []struct {
		name  string
		want  []float64
		want1 []float64
		want2 []bool
	}{
		{
			name:  "NIC Test1",
			want:  []float64{595.0, 595.0, 300},
			want1: []float64{595.0, 595.0, 300},
			want2: []bool{false, false, false},
		},
		{
			name:  "NIC Test2",
			want:  []float64{-5.0, -5.0, 1000},
			want1: []float64{-5.0, -5.0, 1000},
			want2: []bool{false, false, false},
		},
		{
			name:  "NIC Test3",
			want:  []float64{795.0, 795.0, 100},
			want1: []float64{795.0, 795.0, 100},
			want2: []bool{false, false, false},
		},
	}
	for index, tt := range tests {
		switch index {
		case 0:
			nde.resourceConfig.GAPool = 60
			nde.resourceConfig.BEPool = 30
			wl1 := utils.Workload{
				InBps:  100.0,
				OutBps: 100.0,
			}
			wl2 := utils.Workload{
				InBps:  5.0,
				OutBps: 5.0,
			}
			nde.WgDevices[gNicId][utils.GaIndex].LimitPods["podGA"] = &wl1
			nde.WgDevices[gNicId][utils.GaIndex].RequestPods["podGA"] = &wl1
			nde.WgDevices[gNicId][utils.BtIndex].LimitPods["podBE"] = &wl1
			nde.WgDevices[gNicId][utils.BtIndex].RequestPods["podBE"] = &wl2
		case 1:
			nde.resourceConfig.GAPool = 60
			nde.resourceConfig.BEPool = 100
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
			nde.WgDevices[gNicId][utils.GaIndex].LimitPods["podGA"] = &wl1
			nde.WgDevices[gNicId][utils.GaIndex].RequestPods["podGA"] = &wl1
			nde.WgDevices[gNicId][utils.BtIndex].LimitPods["podBE"] = &wl3
			nde.WgDevices[gNicId][utils.BtIndex].RequestPods["podBE"] = &wl2
		case 2:
			nde.resourceConfig.GAPool = 100
			nde.resourceConfig.BEPool = 10
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
			nde.WgDevices[gNicId][utils.GaIndex].LimitPods["podGA"] = &wl2
			nde.WgDevices[gNicId][utils.GaIndex].RequestPods["podGA"] = &wl1
			nde.WgDevices[gNicId][utils.BtIndex].LimitPods["podBE"] = &wl1
			nde.WgDevices[gNicId][utils.BtIndex].RequestPods["podBE"] = &wl3
		}
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := nde.calNicAllocatable(gNicId)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("calNicLimitAndAlloc() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("calNicLimitAndAlloc() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("calNicLimitAndAlloc() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestNetEngine_GetPodGroup(t *testing.T) {
	type args struct {
		podId string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test get pod group",

			args: args{},
			want: "",
		},
	}
	for _, tt := range tests {
		init_net_de()
		t.Run(tt.name, func(t *testing.T) {
			if got := nde.GetPodGroup(tt.args.podId); got != tt.want {
				t.Errorf("NetEngine.GetPodGroup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNetEngine_Initialize(t *testing.T) {
	type fields struct {
		IOEngine       agent.IOEngine
		Profile        utils.NicInfos
		NSWhitelist    map[string]struct{}
		Queues         int
		resourceConfig v1.ResourceConfigSpec
		podRegInfos    map[string]*PodRegInfo
	}
	type args struct {
		coreClient *kubernetes.Clientset
		client     *versioned.Clientset
		mtls       bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr error
	}{
		{
			name:   "test net engine initialize",
			fields: fields{},
			args: args{
				coreClient: &kubernetes.Clientset{},
			},
			wantErr: errors.New("net label is not set"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &NetEngine{
				IOEngine:       tt.fields.IOEngine,
				Profile:        tt.fields.Profile,
				NSWhitelist:    tt.fields.NSWhitelist,
				Queues:         tt.fields.Queues,
				resourceConfig: tt.fields.resourceConfig,
				podRegInfos:    tt.fields.podRegInfos,
				EngineCache: NetMetric{
					Cache:    make(map[string]NodeNetBw),
					PodCache: make(map[string]*PodBw),
				},
			}

			err := e.Initialize(tt.args.coreClient, tt.args.client, tt.args.mtls)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("NetEngine.Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNetEngine_RegisterPod(t *testing.T) {
	type fields struct {
		IOEngine agent.IOEngine
		Profile  utils.NicInfos
		Queues   int
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "net register pod",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_net_de()
			nde.podRegInfos = make(map[string]*PodRegInfo)
			nde.podRegInfos["pod1"] = &PodRegInfo{
				Group: "GA",
				Ns:    "test",
			}

			nde.RegisterPod()
		})
	}
}
