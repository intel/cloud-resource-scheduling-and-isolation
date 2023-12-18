/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package agent

import (
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

func TestIOEngine_SetFlag(t *testing.T) {
	type fields struct {
		Flag          int
		ExecutionFlag int
		WgDevices     map[string][]WorkloadGroup
		GpPolicy      []*Policy
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		NodeName      string
		cacheDatas    []CacheData
	}
	type args struct {
		flag int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name:   "set flag test",
			fields: fields{Flag: 1, cacheDatas: []CacheData{}},
			args:   args{flag: 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IOEngine{
				Flag:          tt.fields.Flag,
				ExecutionFlag: tt.fields.ExecutionFlag,
				WgDevices:     tt.fields.WgDevices,
				GpPolicy:      tt.fields.GpPolicy,
				CoreClient:    tt.fields.CoreClient,
				Clientset:     tt.fields.Clientset,
				NodeName:      tt.fields.NodeName,
				cacheDatas:    tt.fields.cacheDatas,
			}
			e.SetFlag(tt.args.flag)
		})
	}
}

func TestIOEngine_IsExecution(t *testing.T) {
	type fields struct {
		Flag          int
		ExecutionFlag int
		WgDevices     map[string][]WorkloadGroup
		GpPolicy      []*Policy
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		NodeName      string
		cacheDatas    []CacheData
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "test is execution",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IOEngine{
				Flag:          tt.fields.Flag,
				ExecutionFlag: tt.fields.ExecutionFlag,
				WgDevices:     tt.fields.WgDevices,
				GpPolicy:      tt.fields.GpPolicy,
				CoreClient:    tt.fields.CoreClient,
				Clientset:     tt.fields.Clientset,
				NodeName:      tt.fields.NodeName,
				cacheDatas:    tt.fields.cacheDatas,
			}
			if got := e.IsExecution(); got != tt.want {
				t.Errorf("IOEngine.IsExecution() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIOEngine_IsAdmin(t *testing.T) {
	type fields struct {
		Flag          int
		ExecutionFlag int
		WgDevices     map[string][]WorkloadGroup
		GpPolicy      []*Policy
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		NodeName      string
		cacheDatas    []CacheData
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "test is admin",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IOEngine{
				Flag:          tt.fields.Flag,
				ExecutionFlag: tt.fields.ExecutionFlag,
				WgDevices:     tt.fields.WgDevices,
				GpPolicy:      tt.fields.GpPolicy,
				CoreClient:    tt.fields.CoreClient,
				Clientset:     tt.fields.Clientset,
				NodeName:      tt.fields.NodeName,
				cacheDatas:    tt.fields.cacheDatas,
			}
			if got := e.IsAdmin(); got != tt.want {
				t.Errorf("IOEngine.IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIOEngine_AddWorkloadGroup(t *testing.T) {
	type fields struct {
		Flag          int
		ExecutionFlag int
		WgDevices     map[string][]WorkloadGroup
		GpPolicy      []*Policy
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		NodeName      string
		cacheDatas    []CacheData
	}
	type args struct {
		wg    WorkloadGroup
		DevId string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name:   "AddWorkloadGroup",
			fields: fields{WgDevices: make(map[string][]WorkloadGroup)},
			args:   args{wg: WorkloadGroup{WorkloadType: 1}, DevId: "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IOEngine{
				Flag:          tt.fields.Flag,
				ExecutionFlag: tt.fields.ExecutionFlag,
				WgDevices:     tt.fields.WgDevices,
				GpPolicy:      tt.fields.GpPolicy,
				CoreClient:    tt.fields.CoreClient,
				Clientset:     tt.fields.Clientset,
				NodeName:      tt.fields.NodeName,
				cacheDatas:    tt.fields.cacheDatas,
			}
			e.AddWorkloadGroup(tt.args.wg, tt.args.DevId)
		})
	}
}

func TestIOEngine_InitializeWorkloadGroup(t *testing.T) {
	type fields struct {
		Flag          int
		ExecutionFlag int
		WgDevices     map[string][]WorkloadGroup
		GpPolicy      []*Policy
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		NodeName      string
		cacheDatas    []CacheData
	}
	type args struct {
		t int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name:   "test initialize workload group",
			fields: fields{WgDevices: make(map[string][]WorkloadGroup)},
			args:   args{t: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IOEngine{
				Flag:          tt.fields.Flag,
				ExecutionFlag: tt.fields.ExecutionFlag,
				WgDevices:     tt.fields.WgDevices,
				GpPolicy:      tt.fields.GpPolicy,
				CoreClient:    tt.fields.CoreClient,
				Clientset:     tt.fields.Clientset,
				NodeName:      tt.fields.NodeName,
				cacheDatas:    tt.fields.cacheDatas,
			}
			e.InitializeWorkloadGroup(tt.args.t)
		})
	}
}

func TestIOEngine_IsExpectStatus(t *testing.T) {
	type fields struct {
		Flag          int
		ExecutionFlag int
		WgDevices     map[string][]WorkloadGroup
		GpPolicy      []*Policy
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		NodeName      string
		cacheDatas    []CacheData
	}

	data := &AdminResourceConfig{
		DiskBePool:  1,
		DiskSysPool: 1,
		NetBePool:   1,
		NetSysPool:  1,
	}

	TestChan := make(chan EventData, chanSize)
	TestChan <- data

	type args struct {
		data EventData
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "test is expect status",
			fields: fields{Flag: 1, ExecutionFlag: 1, cacheDatas: []CacheData{}},
			args:   args{data: <-TestChan},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IOEngine{
				Flag:          tt.fields.Flag,
				ExecutionFlag: tt.fields.ExecutionFlag,
				WgDevices:     tt.fields.WgDevices,
				GpPolicy:      tt.fields.GpPolicy,
				CoreClient:    tt.fields.CoreClient,
				Clientset:     tt.fields.Clientset,
				NodeName:      tt.fields.NodeName,
				cacheDatas:    tt.fields.cacheDatas,
			}
			if got := e.IsExpectStatus(tt.args.data); got != tt.want {
				t.Errorf("IOEngine.IsExpectStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func ProcessTest(data EventData) error {
	adminData := data.(*AdminResourceConfig)
	klog.V(utils.INF).Info("now in disk ProcessAdmin: ", *adminData)
	return nil
}

func TestIOEngine_HandleEvent(t *testing.T) {

	type fields struct {
		Flag          int
		ExecutionFlag int
		WgDevices     map[string][]WorkloadGroup
		GpPolicy      []*Policy
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		NodeName      string
		cacheDatas    []CacheData
	}
	type args struct {
		data EventData
		ef   EventFunc
	}

	data := &AdminResourceConfig{
		DiskBePool:  1,
		DiskSysPool: 1,
		NetBePool:   1,
		NetSysPool:  1,
	}

	TestChan := make(chan EventData, chanSize)
	TestChan <- data

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "test is expect status",
			fields:  fields{Flag: 1, ExecutionFlag: 1, cacheDatas: []CacheData{}},
			args:    args{data: <-TestChan, ef: ProcessTest},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IOEngine{
				Flag:          tt.fields.Flag,
				ExecutionFlag: tt.fields.ExecutionFlag,
				WgDevices:     tt.fields.WgDevices,
				GpPolicy:      tt.fields.GpPolicy,
				CoreClient:    tt.fields.CoreClient,
				Clientset:     tt.fields.Clientset,
				NodeName:      tt.fields.NodeName,
				cacheDatas:    tt.fields.cacheDatas,
			}
			if err := e.HandleEvent(tt.args.data, tt.args.ef); (err != nil) != tt.wantErr {
				t.Errorf("IOEngine.HandleEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
