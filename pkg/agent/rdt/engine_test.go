/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"errors"
	"reflect"
	"testing"

	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/pkg/agent"
)

var re = &RDTEngine{
	agent.IOEngine{
		Flag:          0,
		ExecutionFlag: agent.ProfileFlag | agent.PolicyFlag | agent.AdminFlag,
	},
	make(map[string]struct{}),
	v1.ResourceConfigSpec{},
	make(map[string]agent.ClassInfo, 0),
}

var podData = &agent.PodData{
	T:          0,
	Generation: 10,
	PodEvents:  make(map[string]agent.PodEventInfo),
}

var containerData = &agent.ContainerData{
	T:          0,
	Generation: 0,
}

var classData = &agent.ClassData{
	T: 0,
}

var adminData = &agent.AdminResourceConfig{
	NamespaceWhitelist: map[string]struct{}{
		"ioi-system":       {},
		"kube-system":      {},
		"kube-flannel":     {},
		"calico-apiserver": {},
		"calico-system":    {},
		"tigera-operator":  {},
	},
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

func init_re() {

	re.NSWhitelist = make(map[string]struct{})
	ns := []string{"ioi-system", "kube-system", "kube-flannel", "calico-apiserver", "calico-system", "tigera-operator"}
	nsLen := len(ns) - 1
	for i := 0; i < nsLen; i++ {
		re.NSWhitelist[ns[i]] = struct{}{}
	}
	re.NodeName = "ioi-1"

	//podInfoes1 := make(map[string]agent.PodEventInfo)

	podInfos1 := make(map[string]agent.PodRunInfo)
	podInfos1["pod1"] = agent.PodRunInfo{
		Operation: 1,
		PodName:   "pod1",
		Namespace: "default",
	}
	podInfos1["pod2"] = agent.PodRunInfo{
		Operation: 1,
		PodName:   "pod2",
		Namespace: "ioi-system",
	}

	podData.PodEvents["dev"] = podInfos1

	containerInfos := make(map[string]agent.ContainerInfo)
	containerInfos["1"] = agent.ContainerInfo{
		Operation:           1,
		PodName:             "pod1",
		ContainerName:       "container1",
		ContainerId:         "1",
		Namespace:           "default",
		RDTAnnotation:       "class1",
		ContainerCGroupPath: "/xxx",
	}
	containerInfos["2"] = agent.ContainerInfo{
		Operation:           1,
		PodName:             "pod2",
		ContainerName:       "container2",
		ContainerId:         "2",
		Namespace:           "default",
		RDTAnnotation:       "class1",
		ContainerCGroupPath: "/xxx",
	}
	containerData.ContainerInfos = containerInfos

	classInfos := make(map[string]agent.ClassInfo)
	classInfos["class1"] = agent.ClassInfo{
		Score: 100,
	}
	classInfos["class2"] = agent.ClassInfo{
		Score: 80,
	}
	classData.ClassInfos = classInfos
}

func TestRDTEngine_ProcessRDTCrData(t *testing.T) {
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test rdt cr",
			args:    args{data: podData},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_re()
			if err := re.ProcessRDTCrData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("RDTEngine.ProcessRDTCrData() error = %v, wantErr %v", err, tt.wantErr)
			}
			if RDTObservedGeneration != podData.Generation {
				t.Errorf("RDTEngine.ProcessRDTCrData() error: RDTObservedGeneration: %v, poddata.Generation: %v", RDTObservedGeneration, podData.Generation)
			}
		})
	}
}

func TestRDTEngine_ProcessRDTData(t *testing.T) {
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test process rdt data",
			args:    args{data: classData},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_re()
			if err := re.ProcessRDTData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("RDTEngine.ProcessRDTIOData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRDTEngine_ProcessAdmin(t *testing.T) {
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
			init_re()
			if err := re.ProcessAdmin(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("RDTEngine.ProcessAdmin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRDTEngine_ProcessNriData(t *testing.T) {
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
			args:    args{data: containerData},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_re()
			if err := re.ProcessNriData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("RDTEngine.ProcessNriData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRDTEngine_Initialize(t *testing.T) {
	type fields struct {
		IOEngine       agent.IOEngine
		rdtConn        *grpc.ClientConn
		NSWhitelist    map[string]struct{}
		resourceConfig v1.ResourceConfigSpec
		classInfos     map[string]agent.ClassInfo
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
			name: "",
			fields: fields{
				IOEngine:       agent.IOEngine{},
				rdtConn:        nil,
				NSWhitelist:    make(map[string]struct{}),
				resourceConfig: v1.ResourceConfigSpec{},
				classInfos:     make(map[string]agent.ClassInfo),
			},
			args:    args{},
			wantErr: errors.New("rdt label is not set"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_re()
			e := &RDTEngine{
				IOEngine:       tt.fields.IOEngine,
				NSWhitelist:    tt.fields.NSWhitelist,
				resourceConfig: tt.fields.resourceConfig,
				classInfos:     tt.fields.classInfos,
			}

			err := e.Initialize(tt.args.coreClient, tt.args.client, tt.args.mtls)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("RDTEngine.Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
