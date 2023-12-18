/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdtQuantity

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

var re = &RDTQuantityEngine{
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
		Operation:             1,
		PodName:               "pod1",
		ContainerName:         "container1",
		ContainerId:           "1",
		Namespace:             "default",
		RDTQuantityAnnotation: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"50MBps\",\"l3cache\":\"100MiB\"},\"class\":\"class2\"}",
		ContainerCGroupPath:   "/xxx",
	}
	containerInfos["2"] = agent.ContainerInfo{
		Operation:             1,
		PodName:               "pod2",
		ContainerName:         "container2",
		ContainerId:           "2",
		Namespace:             "default",
		RDTQuantityAnnotation: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"50MBps\",\"l3cache\":\"100MiB\"},\"class\":\"class2\"}",
		ContainerCGroupPath:   "/xxx",
	}
	containerData.ContainerInfos = containerInfos

	classInfos := make(map[string]agent.ClassInfo)
	classInfos["BE"] = agent.ClassInfo{
		Score: 100,
	}
	classData.ClassInfos = classInfos
}

func TestRDTQuantityEngine_ProcessRDTQuantityCrData(t *testing.T) {
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test rdt quantity cr",
			args:    args{data: podData},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_re()
			if err := re.ProcessRDTQuantityCrData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("RDTQuantityEngine.ProcessRDTQuantityCrData() error = %v, wantErr %v", err, tt.wantErr)
			}
			if RDTQuantityObservedGeneration != podData.Generation {
				t.Errorf("RDTQuantityEngine.ProcessRDTQuantityCrData() error: RDTQuantityObservedGeneration: %v, poddata.Generation: %v", RDTQuantityObservedGeneration, podData.Generation)
			}
		})
	}
}

func TestRDTQuantityEngine_ProcessRDTQuantityData(t *testing.T) {
	type args struct {
		data agent.EventData
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test process rdt quantity data",
			args:    args{data: classData},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init_re()
			if err := re.ProcessRDTQuantityData(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("RDTQuantityEngine.ProcessRDTQuantityData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRDTQuantityEngine_ProcessAdmin(t *testing.T) {
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
				t.Errorf("RDTQuantityEngine.ProcessAdmin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRDTQuantityEngine_ProcessNriData(t *testing.T) {
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
				t.Errorf("RDTQuantityEngine.ProcessNriData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRDTQuantityEngine_Initialize(t *testing.T) {
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
			wantErr: errors.New("rdt quantity label is not set"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &RDTQuantityEngine{
				IOEngine:       tt.fields.IOEngine,
				NSWhitelist:    tt.fields.NSWhitelist,
				resourceConfig: tt.fields.resourceConfig,
				classInfos:     tt.fields.classInfos,
			}

			err := e.Initialize(tt.args.coreClient, tt.args.client, tt.args.mtls)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("RDTQuantityEngine.Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
