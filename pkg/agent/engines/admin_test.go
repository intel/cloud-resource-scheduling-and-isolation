/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package engines

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/IOIsolation/pkg/agent"
)

func TestAdminEngine_GetAdminConfig(t *testing.T) {
	adp := &corev1.ConfigMap{
		Data: map[string]string{
			"diskpools":          "bePool=20\nsysPool=20\n",
			"networkpools":       "bePool=30\nsysPool=20\n",
			"loglevel":           "nodes=io\nlevel=info\n",
			"diskRatio":          "system=50\n",
			"namespaceWhitelist": "ioi-system\n",
			"reportInterval":     "netInterval=1\ndiskInterval=1\nrdtInterval=10\n",
			"profiler":           "times=1\n",
			"min_be_bw":          "disk_read=5\ndisk_write=5\nnet_read=5\nnet_write=5\n",
		},
	}
	rsc := &agent.AdminResourceConfig{
		DiskBePool:  20,
		DiskSysPool: 20,
		NetBePool:   30,
		NetSysPool:  20,
		Loglevel: agent.IOILogLevel{
			NodeAgentInfo:   true,
			NodeAgentDebug:  false,
			SchedulerInfo:   false,
			SchedulerDebug:  false,
			AggregatorInfo:  true,
			AggregatorDebug: false,
			ServiceInfo:     true,
			ServiceDebug:    false,
		},
		SysDiskRatio: 50,
		NamespaceWhitelist: map[string]struct{}{
			"ioi-system": {},
		},
		Interval: agent.ReportInterval{
			NetInterval:  1,
			DiskInterval: 1,
			RdtInterval:  10,
		},
		ProfileTime:         1,
		Disk_min_pod_in_bw:  5,
		Disk_min_pod_out_bw: 5,
		Net_min_pod_in_bw:   5,
		Net_min_pod_out_bw:  5,
	}

	adp_wrong := &corev1.ConfigMap{
		Data: map[string]string{
			"diskpools":          "bePool=2h0\nsysPool=2h0\n",
			"networkpools":       "bePool=3h0\nsysPool=2h0\n",
			"loglevel":           "nodes=ioi-1\nlevel=warn\n",
			"diskRatio":          "system=5h0\n",
			"namespaceWhitelist": "ioi-system\n",
			"reportInterval":     "netInterval=1\ndiskInterval=1\nrdtInterval=10\n",
			"profiler":           "times=h1\n",
			"min_be_bw":          "disk_read=5\ndisk_write=5\nnet_read=5\nnet_write=5\n",
		},
	}

	rsc_wrong := &agent.AdminResourceConfig{
		DiskBePool:  20,
		DiskSysPool: 20,
		NetBePool:   20,
		NetSysPool:  20,
		Loglevel: agent.IOILogLevel{
			NodeAgentInfo:   false,
			NodeAgentDebug:  false,
			SchedulerInfo:   false,
			SchedulerDebug:  false,
			AggregatorInfo:  false,
			AggregatorDebug: false,
			ServiceInfo:     false,
			ServiceDebug:    false,
		},
		SysDiskRatio: 50,
		NamespaceWhitelist: map[string]struct{}{
			"ioi-system": {},
		},
		Interval: agent.ReportInterval{
			NetInterval:  1,
			DiskInterval: 1,
			RdtInterval:  10,
		},
		ProfileTime:         1,
		Disk_min_pod_in_bw:  5,
		Disk_min_pod_out_bw: 5,
		Net_min_pod_in_bw:   5,
		Net_min_pod_out_bw:  5,
	}

	type fields struct {
		NodeName string
	}
	type args struct {
		ad *corev1.ConfigMap
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *agent.AdminResourceConfig
	}{
		{
			name:   "test get admin config",
			fields: fields{NodeName: "ioi-1"},
			args:   args{ad: adp},
			want:   rsc,
		},
		{
			name:   "test get admin config",
			fields: fields{NodeName: "ioi-1"},
			args:   args{ad: adp_wrong},
			want:   rsc_wrong,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &AdminEngine{
				NodeName: tt.fields.NodeName,
			}
			got := e.GetAdminConfig(tt.args.ad)
			if !reflect.DeepEqual(got.DiskBePool, tt.want.DiskBePool) {
				t.Errorf("AdminEngine.GetAdminConfig() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got.Loglevel, tt.want.Loglevel) {
				t.Errorf("AdminEngine.GetAdminConfig() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got.Disk_min_pod_in_bw, tt.want.Disk_min_pod_in_bw) {
				t.Errorf("AdminEngine.GetAdminConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdminEngine_Uninitialize(t *testing.T) {
	type fields struct {
		NodeName string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "test uninitialize",
			fields: fields{
				NodeName: "ioi-1",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &AdminEngine{
				NodeName: tt.fields.NodeName,
			}
			if err := e.Uninitialize(); (err != nil) != tt.wantErr {
				t.Errorf("AdminEngine.Uninitialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdminEngine_AddAdminConfigData(t *testing.T) {
	type fields struct {
		NodeName string
	}
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test add admin config data",
			fields: fields{
				NodeName: "ioi-1",
			},
			args: args{
				obj: &corev1.ConfigMap{
					Data: map[string]string{
						"diskpools":    "bePool=20\nsysPool=20\n",
						"networkpools": "bePool=30\nsysPool=20\n",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &AdminEngine{
				NodeName: tt.fields.NodeName,
			}
			e.AddAdminConfigData(tt.args.obj)
		})
	}
}

func TestAdminEngine_UpdateAdminConfigData(t *testing.T) {
	type fields struct {
		NodeName string
	}
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test update admin config data",
			fields: fields{
				NodeName: "ioi-1",
			},
			args: args{
				oldObj: &corev1.ConfigMap{
					Data: map[string]string{
						"diskpools":    "bePool=20\nsysPool=100\n",
						"networkpools": "bePool=30\nsysPool=100\n",
					},
				},
				newObj: &corev1.ConfigMap{
					Data: map[string]string{
						"diskpools":    "bePool=20\nsysPool=100\n",
						"networkpools": "bePool=30\nsysPool=100\n",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &AdminEngine{
				NodeName: tt.fields.NodeName,
			}
			e.UpdateAdminConfigData(tt.args.oldObj, tt.args.newObj)
		})
	}
}
