/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package blockio

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	testutils "sigs.k8s.io/IOIsolation/test/utils"
)

// import (
// 	"reflect"
// 	"testing"

// 	utils "sigs.k8s.io/IOIsolation/pkg"
// 	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
// 	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
// )

var ReadRatio []utils.MappingRatio = utils.ParseMappingRatio(map[string]float64{
	"1k": 10, "4k": 10, "8k": 10, "16k": 10, "32k": 1,
})

var WriteRatio []utils.MappingRatio = utils.ParseMappingRatio(map[string]float64{
	"1k": 10, "4k": 10, "8k": 10, "16k": 10, "32k": 1,
})

func TestHandle_CriticalPod(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		// RWMutex    sync.RWMutex
	}
	type args struct {
		annotations map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		// TODO: Add test cases.
		{
			name: "block io annotation #1",
			args: args{
				annotations: map[string]string{
					utils.BlockIOConfigAnno: "{\"dev1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
				},
			},
			want: true,
		},
		{
			name: "block io annotation #2",
			args: args{
				annotations: map[string]string{
					utils.BlockIOConfigAnno: "{\"dev1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
				},
			},
			want: true,
		},
		{
			name: "block io annotation #3",
			args: args{
				annotations: map[string]string{
					utils.BlockIOConfigAnno: "",
				},
			},
			want: false,
		},
		{
			name: "block io annotation #4",
			args: args{
				annotations: map[string]string{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				// RWMutex:    tt.fields.RWMutex,
			}
			if got := h.CriticalPod(tt.args.annotations); got != tt.want {
				t.Errorf("Handle.CriticalPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandle_getVolumeRequestFromAnno(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		// pvcLister  corelisters.PersistentVolumeClaimLister
		// cmLister   corelisters.ConfigMapLister
		client kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		pvcMapping map[string]string
		anno       utils.PodAnnoThroughput
		info       *NodeInfo
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]*pb.IOResourceRequest
	}{
		{
			name: "1 GA typed volume in same disk",
			args: args{
				pvcMapping: map[string]string{
					"volume1": "disk1",
				},
				anno: utils.PodAnnoThroughput{
					"volume1": utils.VolumeAnnoThroughput{
						Requests: utils.BlockIOConfig{
							Rbps: "50Mi",
							Wbps: "50Mi",
						},
						Limits: utils.BlockIOConfig{
							Rbps: "50Mi",
							Wbps: "50Mi",
						},
					},
				},
				info: &NodeInfo{
					DisksStatus: map[string]*DiskInfo{
						"disk1": {
							ReadRatio:  ReadRatio,
							WriteRatio: WriteRatio,
						},
					},
				},
			},
			want: map[string]*pb.IOResourceRequest{
				"disk1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "disk1",
						Request: &pb.Quantity{
							Inbps:  500,
							Outbps: 500,
						},
						Limit: &pb.Quantity{
							Inbps:  500,
							Outbps: 500,
						},
						RawRequest: &pb.Quantity{
							Inbps:   50,
							Outbps:  50,
							Iniops:  13107,
							Outiops: 13107,
						},
						RawLimit: &pb.Quantity{
							Inbps:   50,
							Outbps:  50,
							Iniops:  13107,
							Outiops: 13107,
						},
					},
				},
			},
		},
		{
			name: "2 GA typed volume in same disk",
			args: args{
				pvcMapping: map[string]string{
					"volume1": "disk1",
					"volume2": "disk1",
				},
				anno: utils.PodAnnoThroughput{
					"volume1": utils.VolumeAnnoThroughput{
						Requests: utils.BlockIOConfig{
							Rbps: "50Mi/4k",
							Wbps: "50Mi/4k",
						},
						Limits: utils.BlockIOConfig{
							Rbps: "50Mi/4k",
							Wbps: "50Mi/4k",
						},
					},
					"volume2": utils.VolumeAnnoThroughput{
						Requests: utils.BlockIOConfig{
							Rbps: "50Mi/32k",
							Wbps: "50Mi/32k",
						},
						Limits: utils.BlockIOConfig{
							Rbps: "50Mi/32k",
							Wbps: "50Mi/32k",
						},
					},
				},
				info: &NodeInfo{
					DisksStatus: map[string]*DiskInfo{
						"disk1": {
							ReadRatio:  ReadRatio,
							WriteRatio: WriteRatio,
						},
					},
				},
			},
			want: map[string]*pb.IOResourceRequest{
				"disk1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "disk1",
						Request: &pb.Quantity{
							Inbps:  550,
							Outbps: 550,
						},
						Limit: &pb.Quantity{
							Inbps:  550,
							Outbps: 550,
						},
						RawRequest: &pb.Quantity{
							Inbps:   100,
							Outbps:  100,
							Iniops:  13107 + 1638,
							Outiops: 13107 + 1638,
						},
						RawLimit: &pb.Quantity{
							Inbps:   100,
							Outbps:  100,
							Iniops:  13107 + 1638,
							Outiops: 13107 + 1638,
						},
					},
				},
			},
		},

		{
			name: "2 BT typed volume in different disk",
			args: args{
				pvcMapping: map[string]string{
					"volume1": "disk1",
					"volume2": "disk2",
				},
				anno: utils.PodAnnoThroughput{
					"volume1": utils.VolumeAnnoThroughput{
						Requests: utils.BlockIOConfig{
							Rbps: "50Mi/32k",
							Wbps: "50Mi/32k",
						},
						Limits: utils.BlockIOConfig{
							Rbps: "100Mi/32k",
							Wbps: "100Mi/32k",
						},
					},
					"volume2": utils.VolumeAnnoThroughput{
						Requests: utils.BlockIOConfig{
							Rbps: "50Mi/32k",
							Wbps: "50Mi/32k",
						},
						Limits: utils.BlockIOConfig{
							Rbps: "100Mi/32k",
							Wbps: "100Mi/32k",
						},
					},
				},
				info: &NodeInfo{
					DisksStatus: map[string]*DiskInfo{
						"disk1": {
							ReadRatio:  ReadRatio,
							WriteRatio: WriteRatio,
						},
						"disk2": {
							ReadRatio:  ReadRatio,
							WriteRatio: WriteRatio,
						},
					},
				},
			},
			want: map[string]*pb.IOResourceRequest{
				"disk1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "disk1",
						Request: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
						Limit: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						RawRequest: &pb.Quantity{
							Inbps:   50,
							Outbps:  50,
							Iniops:  1638,
							Outiops: 1638,
						},
						RawLimit: &pb.Quantity{
							Inbps:   100,
							Outbps:  100,
							Iniops:  3276,
							Outiops: 3276,
						},
					},
				},
				"disk2": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "disk2",
						Request: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
						Limit: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						RawRequest: &pb.Quantity{
							Inbps:   50,
							Outbps:  50,
							Iniops:  1638,
							Outiops: 1638,
						},
						RawLimit: &pb.Quantity{
							Inbps:   100,
							Outbps:  100,
							Iniops:  3276,
							Outiops: 3276,
						},
					},
				},
			},
		},

		{
			name: "BT typed volume and GA typed volume in same disk",
			args: args{
				pvcMapping: map[string]string{
					"volume1": "disk1",
					"volume2": "disk1",
				},
				anno: utils.PodAnnoThroughput{
					"volume1": utils.VolumeAnnoThroughput{
						Requests: utils.BlockIOConfig{
							Rbps: "50Mi/32k",
							Wbps: "50Mi/32k",
						},
						Limits: utils.BlockIOConfig{
							Rbps: "100Mi/32k",
							Wbps: "100Mi/32k",
						},
					},
					"volume2": utils.VolumeAnnoThroughput{
						Requests: utils.BlockIOConfig{
							Rbps: "50Mi/32k",
							Wbps: "50Mi/32k",
						},
						Limits: utils.BlockIOConfig{
							Rbps: "50Mi/32k",
							Wbps: "50Mi/32k",
						},
					},
				},
				info: &NodeInfo{
					DisksStatus: map[string]*DiskInfo{
						"disk1": {
							ReadRatio:  ReadRatio,
							WriteRatio: WriteRatio,
						},
					},
				},
			},
			want: map[string]*pb.IOResourceRequest{
				"disk1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "disk1",
						Request: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						Limit: &pb.Quantity{
							Inbps:  150,
							Outbps: 150,
						},
						RawRequest: &pb.Quantity{
							Inbps:   100,
							Outbps:  100,
							Iniops:  3276,
							Outiops: 3276,
						},
						RawLimit: &pb.Quantity{
							Inbps:   150,
							Outbps:  150,
							Iniops:  4914,
							Outiops: 4914,
						},
					},
				},
			},
		},
		{
			name: "2 BE typed volume in same disk",
			args: args{
				pvcMapping: map[string]string{
					"volume1": "disk1",
					"volume2": "disk1",
				},
				anno: utils.PodAnnoThroughput{},
				info: &NodeInfo{
					DisksStatus: map[string]*DiskInfo{
						"disk1": {
							ReadRatio:      ReadRatio,
							WriteRatio:     WriteRatio,
							BEDefaultRead:  10,
							BEDefaultWrite: 10,
						},
					},
				},
			},
			want: map[string]*pb.IOResourceRequest{
				"disk1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "disk1",
					},
				},
			},
		},
		{
			name: "2 BE typed volume in different disks",
			args: args{
				pvcMapping: map[string]string{
					"volume1": "disk1",
					"volume2": "disk2",
				},
				anno: utils.PodAnnoThroughput{},
				info: &NodeInfo{
					DisksStatus: map[string]*DiskInfo{
						"disk1": {
							ReadRatio:      ReadRatio,
							WriteRatio:     WriteRatio,
							BEDefaultRead:  10,
							BEDefaultWrite: 10,
						},
						"disk2": {
							ReadRatio:      ReadRatio,
							WriteRatio:     WriteRatio,
							BEDefaultRead:  20,
							BEDefaultWrite: 20,
						},
					},
				},
			},
			want: map[string]*pb.IOResourceRequest{
				"disk1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "disk1",
					},
				},
				"disk2": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "disk2",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				// pvcLister:  tt.fields.pvcLister,
				// cmLister:   tt.fields.cmLister,
				client: tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			if got := h.getVolumeRequestFromAnno(tt.args.pvcMapping, tt.args.anno, tt.args.info); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle.getVolumeRequestFromAnno() = %v, want %v", got, tt.want)
			}
		})
	}
}
func mapToString(m map[string]*pb.IOResourceRequest) string {
	var res string
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		res += fmt.Sprintf("%s:%v\n", k, v)
	}
	return res
}

func Test_addPodRequest(t *testing.T) {
	type args struct {
		m   map[string]*pb.IOResourceRequest
		req map[string]*pb.IOResourceRequest
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "GA merge to GA in same device",
			args: args{
				m: map[string]*pb.IOResourceRequest{
					"dev1": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.GaIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
							Request: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
							Limit: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
							RawRequest: &pb.Quantity{
								Inbps:   100,
								Outbps:  100,
								Iniops:  100,
								Outiops: 100,
							},
							RawLimit: &pb.Quantity{
								Inbps:   100,
								Outbps:  100,
								Iniops:  100,
								Outiops: 100,
							},
						},
					},
				},
				req: map[string]*pb.IOResourceRequest{
					"dev1": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.GaIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
							Request: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
							Limit: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
							RawRequest: &pb.Quantity{
								Inbps:   100,
								Outbps:  100,
								Iniops:  100,
								Outiops: 100,
							},
							RawLimit: &pb.Quantity{
								Inbps:   100,
								Outbps:  100,
								Iniops:  100,
								Outiops: 100,
							},
						},
					},
				},
			},
			want: mapToString(map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  200,
							Outbps: 200,
						},
						Limit: &pb.Quantity{
							Inbps:  200,
							Outbps: 200,
						},
						RawRequest: &pb.Quantity{
							Inbps:   200,
							Outbps:  200,
							Iniops:  200,
							Outiops: 200,
						},
						RawLimit: &pb.Quantity{
							Inbps:   200,
							Outbps:  200,
							Iniops:  200,
							Outiops: 200,
						},
					},
				},
			}),
		},
		{
			name: "BE merge to GA in same device",
			args: args{
				m: map[string]*pb.IOResourceRequest{
					"dev1": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.GaIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
							Request: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
							Limit: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
							RawRequest: &pb.Quantity{
								Inbps:   100,
								Outbps:  100,
								Iniops:  100,
								Outiops: 100,
							},
							RawLimit: &pb.Quantity{
								Inbps:   100,
								Outbps:  100,
								Iniops:  100,
								Outiops: 100,
							},
						},
					},
				},
				req: map[string]*pb.IOResourceRequest{
					"dev1": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.BeIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
						},
					},
				},
			},
			want: mapToString(map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						Limit: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						RawRequest: &pb.Quantity{
							Inbps:   100,
							Outbps:  100,
							Iniops:  100,
							Outiops: 100,
						},
						RawLimit: &pb.Quantity{
							Inbps:   100,
							Outbps:  100,
							Iniops:  100,
							Outiops: 100,
						},
					},
				},
			}),
		},
		{
			name: "BT merge to BE in same device",
			args: args{
				req: map[string]*pb.IOResourceRequest{
					"dev1": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.GaIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
							Request: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
							Limit: &pb.Quantity{
								Inbps:  utils.MAX_BT_LIMIT,
								Outbps: utils.MAX_BT_LIMIT,
							},
							RawRequest: &pb.Quantity{
								Inbps:   100,
								Outbps:  100,
								Iniops:  100,
								Outiops: 100,
							},
							RawLimit: &pb.Quantity{
								Inbps:   utils.MAX_BT_LIMIT,
								Outbps:  utils.MAX_BT_LIMIT,
								Iniops:  100,
								Outiops: 100,
							},
						},
					},
				},
				m: map[string]*pb.IOResourceRequest{
					"dev1": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.BeIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
						},
					},
				},
			},
			want: mapToString(map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						Limit: &pb.Quantity{
							Inbps:  utils.MAX_BT_LIMIT,
							Outbps: utils.MAX_BT_LIMIT,
						},
						RawRequest: &pb.Quantity{
							Inbps:   100,
							Outbps:  100,
							Iniops:  100,
							Outiops: 100,
						},
						RawLimit: &pb.Quantity{
							Inbps:   utils.MAX_BT_LIMIT,
							Outbps:  utils.MAX_BT_LIMIT,
							Iniops:  100,
							Outiops: 100,
						},
					},
				},
			}),
		},
		{
			name: "BE merge to BE in same device",
			args: args{
				req: map[string]*pb.IOResourceRequest{
					"dev1": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.BeIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
						},
					},
				},
				m: map[string]*pb.IOResourceRequest{
					"dev1": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.BeIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
						},
					},
				},
			},
			want: mapToString(map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
					},
				},
			}),
		},
		{
			name: "BT merge to BE in different device",
			args: args{
				req: map[string]*pb.IOResourceRequest{
					"dev1": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.GaIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
							Request: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
							Limit: &pb.Quantity{
								Inbps:  utils.MAX_BT_LIMIT,
								Outbps: utils.MAX_BT_LIMIT,
							},
							RawRequest: &pb.Quantity{
								Inbps:   100,
								Outbps:  100,
								Iniops:  100,
								Outiops: 100,
							},
							RawLimit: &pb.Quantity{
								Inbps:   utils.MAX_BT_LIMIT,
								Outbps:  utils.MAX_BT_LIMIT,
								Iniops:  100,
								Outiops: 100,
							},
						},
					},
				},
				m: map[string]*pb.IOResourceRequest{
					"dev2": {
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.BeIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev2",
						},
					},
				},
			},
			want: mapToString(map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						Limit: &pb.Quantity{
							Inbps:  utils.MAX_BT_LIMIT,
							Outbps: utils.MAX_BT_LIMIT,
						},
						RawRequest: &pb.Quantity{
							Inbps:   100,
							Outbps:  100,
							Iniops:  100,
							Outiops: 100,
						},
						RawLimit: &pb.Quantity{
							Inbps:   utils.MAX_BT_LIMIT,
							Outbps:  utils.MAX_BT_LIMIT,
							Iniops:  100,
							Outiops: 100,
						},
					},
				},
				"dev2": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev2",
					},
				},
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addPodRequest(tt.args.m, tt.args.req)
			// klog.Info(tt.args.m)
			got := mapToString(tt.args.m)
			if got != tt.want {
				t.Errorf("Handle.addPodRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func fakeResourceCache(ga, be *utils.IOPoolStatus, reqDisk int64) ioiresource.ExtendedCache {
	ec := ioiresource.NewExtendedCache()
	rs := &Resource{
		nodeName: "node1",
		info: &NodeInfo{
			EmptyDirDevice: "dev1",
			DisksStatus: map[string]*DiskInfo{
				"dev1": {
					ReadRatio:          ReadRatio,
					WriteRatio:         WriteRatio,
					BEDefaultRead:      5,
					BEDefaultWrite:     5,
					Capacity:           100,
					GA:                 ga,
					BE:                 be,
					RequestedDiskSpace: reqDisk,
				},
				// "dev3": {
				// 	ReadRatio:          ReadRatio,
				// 	WriteRatio:         WriteRatio,
				// 	BEDefaultRead:      5,
				// 	BEDefaultWrite:     5,
				// 	Capacity:           100,
				// 	GA:                 ga,
				// 	BE:                 be,
				// 	RequestedDiskSpace: reqDisk,
				// },
			},
		},
	}
	ec.SetExtendedResource("node1", rs)
	return ec
}

func TestHandle_CanAdmitPod(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		// pvcLister  corelisters.PersistentVolumeClaimLister
		// cmLister   corelisters.ConfigMapLister
		client kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		nodeName   string
		rawRequest interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "pod without storage request can schedule",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, 0),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &RequestInfo{
					ResourceRequest: map[string]*utils.IOResourceRequest{
						"dev1": {
							IORequest: &pb.IOResourceRequest{
								IoType:       pb.IOType_DiskIO,
								WorkloadType: utils.GaIndex,
								DevRequest: &pb.DeviceRequirement{
									DevName: "dev1",
									Request: &pb.Quantity{
										Inbps:  10,
										Outbps: 10,
									},
									Limit: &pb.Quantity{
										Inbps:  10,
										Outbps: 10,
									},
								},
							},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},

		{
			name: "pod without storage request can't schedule",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, 0),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &RequestInfo{
					ResourceRequest: map[string]*utils.IOResourceRequest{
						"dev1": {
							IORequest: &pb.IOResourceRequest{
								IoType:       pb.IOType_DiskIO,
								WorkloadType: utils.GaIndex,
								DevRequest: &pb.DeviceRequirement{
									DevName: "dev1",
									Request: &pb.Quantity{
										Inbps:  100,
										Outbps: 100,
									},
									Limit: &pb.Quantity{
										Inbps:  100,
										Outbps: 100,
									},
								},
							},
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},

		{
			name: "pod with storage request can schedule",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, 0),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &RequestInfo{
					ResourceRequest: map[string]*utils.IOResourceRequest{
						"dev1": {
							IORequest: &pb.IOResourceRequest{
								IoType:       pb.IOType_DiskIO,
								WorkloadType: utils.GaIndex,
								DevRequest: &pb.DeviceRequirement{
									DevName: "dev1",
									Request: &pb.Quantity{
										Inbps:  10,
										Outbps: 10,
									},
									Limit: &pb.Quantity{
										Inbps:  10,
										Outbps: 10,
									},
								},
							},
							StorageRequest: 10,
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},

		{
			name: "pod with storage request can't schedule",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, 80),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &RequestInfo{
					ResourceRequest: map[string]*utils.IOResourceRequest{
						"dev1": {
							IORequest: &pb.IOResourceRequest{
								IoType:       pb.IOType_DiskIO,
								WorkloadType: utils.GaIndex,
								DevRequest: &pb.DeviceRequirement{
									DevName: "dev1",
									Request: &pb.Quantity{
										Inbps:  100,
										Outbps: 100,
									},
									Limit: &pb.Quantity{
										Inbps:  100,
										Outbps: 100,
									},
								},
							},
							StorageRequest: 90,
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				// pvcLister:  tt.fields.pvcLister,
				// cmLister:   tt.fields.cmLister,
				client: tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			got, _, err := h.CanAdmitPod(tt.args.nodeName, tt.args.rawRequest)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle.CanAdmitPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Handle.CanAdmitPod() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandle_NodePressureRatio(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		// pvcLister  corelisters.PersistentVolumeClaimLister
		// cmLister   corelisters.ConfigMapLister
		client kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		request interface{}
		pool    interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    float64
		wantErr bool
	}{
		{
			name: "request device number not 0",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, 0),
				},
			},
			args: args{
				request: &RequestInfo{
					ResourceRequest: map[string]*utils.IOResourceRequest{
						"dev1": {
							IORequest: &pb.IOResourceRequest{
								IoType:       pb.IOType_DiskIO,
								WorkloadType: utils.GaIndex,
								DevRequest: &pb.DeviceRequirement{
									DevName: "dev1",
									Request: &pb.Quantity{
										Inbps:  10,
										Outbps: 10,
									},
									Limit: &pb.Quantity{
										Inbps:  10,
										Outbps: 10,
									},
								},
							},
						},
					},
				},
				pool: map[string]map[string]*utils.IOPoolStatus{
					"dev1": {
						utils.GA_Type: {
							Total: 100,
							In:    50,
							Out:   50,
						},
					},
				},
			},
			want:    float64((float64(10)/float64(50) + float64(10)/float64(50) + float64(10)/float64(50)) / float64(3.0)),
			wantErr: false,
		},

		{
			name: "request device number is 0",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, 0),
				},
			},
			args: args{
				request: &RequestInfo{
					ResourceRequest: map[string]*utils.IOResourceRequest{},
				},
				pool: map[string]map[string]*utils.IOPoolStatus{
					"dev1": {
						utils.GA_Type: {
							Total: 100,
							In:    50,
							Out:   50,
						},
					},
				},
			},
			want:    0.5,
			wantErr: false,
		},
		{
			name: "be pod request",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, 0),
				},
			},
			args: args{
				request: &RequestInfo{
					ResourceRequest: map[string]*utils.IOResourceRequest{
						"dev1": {
							IORequest: &pb.IOResourceRequest{
								IoType:       pb.IOType_DiskIO,
								WorkloadType: utils.BeIndex,
							},
						},
					},
				},
				pool: map[string]map[string]*utils.IOPoolStatus{
					"dev1": {
						utils.GA_Type: {
							Total: 100,
							In:    50,
							Out:   50,
						},
					},
				},
			},
			want:    0,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				// pvcLister:  tt.fields.pvcLister,
				// cmLister:   tt.fields.cmLister,
				client: tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			got, err := h.NodePressureRatio(tt.args.request, tt.args.pool)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle.NodePressureRatio() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Handle.NodePressureRatio() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandle_GetRequestByAnnotation(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		// pvcLister  corelisters.PersistentVolumeClaimLister
		// cmLister   corelisters.ConfigMapLister
		client kubernetes.Interface
		packer BinPacker
		// RWMutex    sync.RWMutex
	}
	type args struct {
		pod  *corev1.Pod
		node string
		init bool
	}
	scName := "sc1"
	ctx := context.Background()
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()

	if err := testutils.MakeStorageClass(fakeKubeClient, ctx, scName, "localstorage.csi.k8s.io"); err != nil {
		t.Error(err)
	}
	if err := testutils.MakePVC(fakeKubeClient, ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc1",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("10"),
				},
			},
		},
	}); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]*pb.IOResourceRequest
		wantErr bool
	}{
		{
			name: "emptyDir attached pod running when scheduler init, annotation wrong format",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{{\"vol1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
				node: "node1",
				init: true,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "emptyDir attached pod running when scheduler init, cannot find volume in annotation",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{{\"vol2\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
				node: "node1",
				init: true,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "emptyDir attached pod running when scheduler init",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{\"vol1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}},\"vol2\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
							{
								Name: "vol2",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
				node: "node1",
				init: true,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						Limit: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						RawRequest: &pb.Quantity{
							Inbps:   10,
							Outbps:  10,
							Iniops:  2620,
							Outiops: 2620,
						},
						RawLimit: &pb.Quantity{
							Inbps:   10,
							Outbps:  10,
							Iniops:  2620,
							Outiops: 2620,
						},
					},
				},
			},
			wantErr: false,
		},

		{
			name: "pvc attached pod running when scheduler init",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{\"vol1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc1",
									},
								},
							},
						},
					},
				},
				node: "node1",
				init: true,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
						Limit: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
						RawRequest: &pb.Quantity{
							Inbps:   5,
							Outbps:  5,
							Iniops:  1310,
							Outiops: 1310,
						},
						RawLimit: &pb.Quantity{
							Inbps:   5,
							Outbps:  5,
							Iniops:  1310,
							Outiops: 1310,
						},
					},
				},
			},
			wantErr: false,
		},

		{
			name: "scheduling new emptyDir attached pod",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
				packer: NewFirstFitPacker(),
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{\"vol1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}},\"vol2\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
							{
								Name: "vol2",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
				node: "node1",
				init: false,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						Limit: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						RawRequest: &pb.Quantity{
							Inbps:   10,
							Outbps:  10,
							Iniops:  2620,
							Outiops: 2620,
						},
						RawLimit: &pb.Quantity{
							Inbps:   10,
							Outbps:  10,
							Iniops:  2620,
							Outiops: 2620,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "scheduling new emptyDir attached pod, annotation wrong format",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
				packer: NewFirstFitPacker(),
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{{\"vol1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
				node: "node1",
				init: false,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
					},
				},
			},
			wantErr: false,
		},

		{
			name: "scheduling new pvc attached pod",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 2000,
						In:    1000,
						Out:   1000,
					}, &utils.IOPoolStatus{
						Total: 2000,
						In:    1000,
						Out:   1000,
					}, 0),
				},
				client: fakeKubeClient,
				packer: NewFirstFitPacker(),
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{\"vol1\":{\"requests\":{\"rbps\":\"50Mi/4k\",\"wbps\":\"50Mi/4k\"},\"limits\":{\"rbps\":\"50Mi/4k\",\"wbps\":\"50Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc1",
									},
								},
							},
						},
					},
				},
				node: "node1",
				init: false,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  500,
							Outbps: 500,
						},
						Limit: &pb.Quantity{
							Inbps:  500,
							Outbps: 500,
						},
						RawRequest: &pb.Quantity{
							Inbps:   50,
							Outbps:  50,
							Iniops:  13107,
							Outiops: 13107,
						},
						RawLimit: &pb.Quantity{
							Inbps:   50,
							Outbps:  50,
							Iniops:  13107,
							Outiops: 13107,
						},
					},
				},
			},
			wantErr: false,
		},

		{
			name: "scheduling new pvc and emptyDir attached pod",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
				packer: NewFirstFitPacker(),
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{\"vol1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}},\"vol2\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc1",
									},
								},
							},
							{
								Name: "vol2",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
				node: "node1",
				init: false,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						Limit: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						RawRequest: &pb.Quantity{
							Inbps:   10,
							Outbps:  10,
							Iniops:  2620,
							Outiops: 2620,
						},
						RawLimit: &pb.Quantity{
							Inbps:   10,
							Outbps:  10,
							Iniops:  2620,
							Outiops: 2620,
						},
					},
				},
			},
			wantErr: false,
		},

		{
			name: "scheduling new pvc and emptyDir attached pod not in same QoS #1",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
				packer: NewFirstFitPacker(),
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{\"vol1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}},\"vol2\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"10Mi/4k\",\"wbps\":\"10Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc1",
									},
								},
							},
							{
								Name: "vol2",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
				node: "node1",
				init: false,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  100,
							Outbps: 100,
						},
						Limit: &pb.Quantity{
							Inbps:  150,
							Outbps: 150,
						},
						RawRequest: &pb.Quantity{
							Inbps:   10,
							Outbps:  10,
							Iniops:  2620,
							Outiops: 2620,
						},
						RawLimit: &pb.Quantity{
							Inbps:   15,
							Outbps:  15,
							Iniops:  3931,
							Outiops: 3931,
						},
					},
				},
			},
			wantErr: false,
		},

		{
			name: "scheduling new pvc and emptyDir attached pod not in same QoS #2",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
				packer: NewFirstFitPacker(),
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.BlockIOConfigAnno: "{\"vol1\":{\"requests\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"},\"limits\":{\"rbps\":\"5Mi/4k\",\"wbps\":\"5Mi/4k\"}}}",
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc1",
									},
								},
							},
							{
								Name: "vol2",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
				node: "node1",
				init: false,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
						Limit: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
						RawRequest: &pb.Quantity{
							Inbps:   5,
							Outbps:  5,
							Iniops:  1310,
							Outiops: 1310,
						},
						RawLimit: &pb.Quantity{
							Inbps:   5,
							Outbps:  5,
							Iniops:  1310,
							Outiops: 1310,
						},
					},
				},
			},
			wantErr: false,
		},

		{
			name: "scheduling new pod without volume declaration",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pod1",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{},
					},
				},
				node: "node1",
				init: false,
			},
			want: map[string]*pb.IOResourceRequest{
				"dev1": {
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		ioiresource.IoiContext = &ioiresource.ResourceIOContext{
			Reservedpod: make(map[string]*pb.PodList),
			ClaimDevMap: map[string]*ioiresource.StorageInfo{
				"pvc1.default": {
					DevID:            "dev1",
					RequestedStorage: 10,
					NodeName:         "node1",
				},
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				packer:     tt.fields.packer,
			}
			got, _, err := h.GetRequestByAnnotation(tt.args.pod, tt.args.node, tt.args.init)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle.GetRequestByAnnotation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(mapToString(got), mapToString(tt.want)) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandle_NodeHasIoiLabel(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		node *corev1.Node
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "node has label all",
			fields: fields{},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"ioisolation": "all",
						},
					},
				},
			},
			want: true,
		},
		{
			name:   "node has label disk",
			fields: fields{},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"ioisolation": "rdt-disk",
						},
					},
				},
			},
			want: true,
		},
		{
			name:   "node has label disk",
			fields: fields{},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"ioisolation": "disk",
						},
					},
				},
			},
			want: true,
		},
		{
			name:   "node without label disk",
			fields: fields{},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"ioisolation": "net",
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			if got := h.NodeHasIoiLabel(tt.args.node); got != tt.want {
				t.Errorf("Handle.NodeHasIoiLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandle_InitBENum(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		nodeName string
		rc       v1.ResourceConfigSpec
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		// TODO: Add test cases.
		{
			name: "get initial be pod number",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, &utils.IOPoolStatus{
						Total: 200,
						In:    100,
						Out:   100,
					}, 0),
				},
			},
			args: args{
				nodeName: "node1",
				rc: v1.ResourceConfigSpec{
					Devices: map[string]v1.Device{
						"dev1": {},
					},
				},
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		ioiresource.IoiContext = &ioiresource.ResourceIOContext{
			Reservedpod: map[string]*pb.PodList{
				"node1": {
					PodList: map[string]*pb.PodRequest{
						"pod1": {
							PodName: "pod1",
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_DiskIO,
									WorkloadType: utils.BeIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "dev1",
									},
								},
							},
						},
						"pod2": {
							PodName: "pod2",
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_DiskIO,
									WorkloadType: utils.BeIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "dev1",
									},
								},
							},
						},
						"pod3": {
							PodName: "pod3",
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_DiskIO,
									WorkloadType: utils.GaIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "dev1",
									},
								},
							},
						},
					},
				},
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			h.InitBENum(tt.args.nodeName, tt.args.rc)
			got := tt.fields.HandleBase.EC.GetExtendedResource(tt.args.nodeName)
			be := got.(*Resource).info.DisksStatus["dev1"].BE.Num
			if !reflect.DeepEqual(be, tt.want) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", be, tt.want)
			}
		})
	}
}

func TestHandle_InitNodeStatus(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		nodeName string
		rc       v1.ResourceConfigSpec
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want1  *utils.IOPoolStatus
		want2  *utils.IOPoolStatus
	}{
		{
			name: "get initial ga cache",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
				},
			},
			args: args{
				nodeName: "node1",
				rc: v1.ResourceConfigSpec{
					Devices: map[string]v1.Device{
						"dev1": {
							Name:          "dev1",
							Type:          v1.DefaultDevice,
							CapacityIn:    1000,
							CapacityOut:   1000,
							CapacityTotal: 2000,
						},
					},
				},
			},
			want1: &utils.IOPoolStatus{
				Total: 1800,
				In:    900,
				Out:   900,
			},
			want2: &utils.IOPoolStatus{
				Num: 1,
			},
		},
	}
	for _, tt := range tests {
		ioiresource.IoiContext = &ioiresource.ResourceIOContext{
			Reservedpod: map[string]*pb.PodList{
				"node1": {
					PodList: map[string]*pb.PodRequest{
						"pod1": {
							PodName: "pod1",
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_DiskIO,
									WorkloadType: utils.GaIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "dev1",
										Request: &pb.Quantity{
											Inbps:  100,
											Outbps: 100,
										},
									},
								},
							},
						},
						"pod2": {
							PodName: "pod2",
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_DiskIO,
									WorkloadType: utils.BeIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "dev1",
										Request: &pb.Quantity{
											Inbps:  100,
											Outbps: 100,
										},
									},
								},
							},
						},
					},
				},
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			h.InitNodeStatus(tt.args.nodeName, tt.args.rc)
			got := tt.fields.HandleBase.EC.GetExtendedResource(tt.args.nodeName)
			ga := got.(*Resource).info.DisksStatus["dev1"].GA
			be := got.(*Resource).info.DisksStatus["dev1"].BE
			if !reflect.DeepEqual(ga, tt.want1) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", ga, tt.want1)
			}
			if !reflect.DeepEqual(be, tt.want2) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", be, tt.want2)
			}
		})
	}
}

func TestHandle_AddCacheNodeInfo(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		nodeName string
		rc       v1.ResourceConfigSpec
	}
	scName := "sc1"
	mSuccess := map[string]string{
		fmt.Sprintf("%s.%s", "pvc1", "default"): "dev1",
	}
	ctx := context.Background()
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()

	if err := testutils.MakePVC(fakeKubeClient, ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc1",
			Namespace: "default",
			Annotations: map[string]string{
				"volume.kubernetes.io/storage-provisioner": "localstorage.csi.k8s.io",
				"volume.kubernetes.io/selected-node":       "node1",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("10"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}); err != nil {
		t.Error(err)
	}

	if err := testutils.MakeVolumeDeviceMapConfigMap(fakeKubeClient, ctx, "node1", mSuccess); err != nil {
		t.Error(err)
	}

	time.Sleep(time.Second)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		want    int64
	}{
		{
			name: "fill in cache",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
				},
				client: fakeKubeClient,
			},
			args: args{
				nodeName: "node1",
				rc: v1.ResourceConfigSpec{
					BEPool: 10,
					Devices: map[string]v1.Device{
						"dev1": {
							Name:       "dev1",
							Type:       v1.DefaultDevice,
							DefaultIn:  5,
							DefaultOut: 5,
							ReadRatio: map[string]float64{
								"1k": 10, "4k": 10, "8k": 10, "16k": 10, "32k": 1,
							},
							WriteRatio: map[string]float64{
								"1k": 10, "4k": 10, "8k": 10, "16k": 10, "32k": 1,
							},
							CapacityIn:    1000,
							CapacityOut:   1000,
							CapacityTotal: 2000,
							DiskSize:      "10",
						},
					},
				},
			},
			wantErr: false,
			want:    10,
		},
	}
	for _, tt := range tests {
		ioiresource.IoiContext = &ioiresource.ResourceIOContext{
			ClaimDevMap: map[string]*ioiresource.StorageInfo{},
		}
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			if err := h.AddCacheNodeInfo(tt.args.nodeName, tt.args.rc); (err != nil) != tt.wantErr {
				t.Errorf("Handle.AddCacheNodeInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := tt.fields.HandleBase.EC.GetExtendedResource(tt.args.nodeName)
			info := got.(*Resource).info.DisksStatus["dev1"].RequestedDiskSpace
			if !reflect.DeepEqual(info, tt.want) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", info, tt.want)
			}
		})
	}
}

func TestHandle_UpdatePVC(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		pvc      *corev1.PersistentVolumeClaim
		nodename string
		devId    string
		storage  int64
		add      bool
	}
	// scName := "sc1"
	ctx := context.Background()
	mSuccess := map[string]string{
		fmt.Sprintf("%s.%s", "pvc1", "default"): "dev1",
	}
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()
	if err := testutils.MakeVolumeDeviceMapConfigMap(fakeKubeClient, ctx, "node1", mSuccess); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantst  int64
	}{
		// TODO: Add test cases.
		{
			name: "add pvc",
			fields: fields{
				client: fakeKubeClient,
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
				},
			},
			args: args{
				add:      true,
				storage:  10,
				devId:    "dev1",
				nodename: "node1",
				pvc:      &corev1.PersistentVolumeClaim{},
			},
			wantErr: false,
			wantst:  10,
		},
		{
			name: "delete pvc",
			fields: fields{
				client: fakeKubeClient,
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 10),
				},
			},
			args: args{
				add:      false,
				storage:  10,
				devId:    "dev1",
				nodename: "node1",
				pvc: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "default",
						Annotations: map[string]string{
							"volume.kubernetes.io/storage-provisioner": "localstorage.csi.k8s.io",
							"volume.kubernetes.io/selected-node":       "node1",
						},
					},
				},
			},
			wantErr: false,
			wantst:  0,
		},
	}
	for _, tt := range tests {
		ioiresource.IoiContext = &ioiresource.ResourceIOContext{
			ClaimDevMap: map[string]*ioiresource.StorageInfo{
				"pvc1.default": {
					DevID:            "dev1",
					RequestedStorage: 10,
					NodeName:         "node1",
				},
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			if err := h.UpdatePVC(tt.args.pvc, tt.args.nodename, tt.args.devId, tt.args.storage, tt.args.add); (err != nil) != tt.wantErr {
				t.Errorf("Handle.UpdatePVC() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := tt.fields.HandleBase.EC.GetExtendedResource(tt.args.nodename)
			info := got.(*Resource).info.DisksStatus["dev1"].RequestedDiskSpace
			if !reflect.DeepEqual(info, tt.wantst) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", info, tt.wantst)
			}
		})
	}
}

func TestHandle_DeleteCacheNodeInfo(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		nodeName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "delete node cache",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
				},
			},
			args: args{
				nodeName: "node1",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			if err := h.DeleteCacheNodeInfo(tt.args.nodeName); (err != nil) != tt.wantErr {
				t.Errorf("Handle.DeleteCacheNodeInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandle_UpdateCacheNodeStatus(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		nodeName string
		nodeIoBw *v1.NodeIOStatusStatus
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "generation ok to update status",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
				},
			},
			args: args{
				nodeName: "node1",
				nodeIoBw: &v1.NodeIOStatusStatus{
					ObservedGeneration: 1,
					AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
						v1.BlockIO: {
							DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
								"dev1": {
									IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
										v1.WorkloadGA: {
											Total:    100,
											In:       50,
											Out:      50,
											Pressure: 0,
										},
										v1.WorkloadBE: {
											Pressure: 0,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "generation outdated to update status",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
				},
			},
			args: args{
				nodeName: "node1",
				nodeIoBw: &v1.NodeIOStatusStatus{
					ObservedGeneration: 0,
					AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
						v1.BlockIO: {
							DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
								"dev1": {
									IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
										v1.WorkloadGA: {
											Total:    100,
											In:       50,
											Out:      50,
											Pressure: 0,
										},
										v1.WorkloadBE: {
											Pressure: 0,
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		ioiresource.IoiContext = &ioiresource.ResourceIOContext{
			Reservedpod: map[string]*pb.PodList{
				"node1": {
					Generation: 1,
				},
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			if err := h.UpdateCacheNodeStatus(tt.args.nodeName, tt.args.nodeIoBw); (err != nil) != tt.wantErr {
				t.Errorf("Handle.UpdateCacheNodeStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandle_UpdateDeviceNodeInfo(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		packer     BinPacker
		// RWMutex    sync.RWMutex
	}
	type args struct {
		node string
		old  *v1.ResourceConfigSpec
		new  *v1.ResourceConfigSpec
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]v1.Device
		want1   map[string]v1.Device
		wantErr bool
	}{
		{
			name: "add device info",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
				},
			},
			args: args{
				node: "node1",
				old: &v1.ResourceConfigSpec{
					BEPool: 20,
					Devices: map[string]v1.Device{
						"dev1": {
							Name: "dev1",
							Type: v1.DefaultDevice,
						},
					},
				},
				new: &v1.ResourceConfigSpec{
					BEPool: 20,
					Devices: map[string]v1.Device{
						"dev1": {
							Name: "dev1",
							Type: v1.DefaultDevice,
						},
						"dev2": {
							Name:          "dev2",
							CapacityIn:    1000,
							CapacityOut:   1000,
							CapacityTotal: 2000,
							DiskSize:      "10",
						},
					},
				},
			},
			want: map[string]v1.Device{
				"dev2": {
					Name:          "dev2",
					CapacityIn:    1000,
					CapacityOut:   1000,
					CapacityTotal: 2000,
					DiskSize:      "10",
				},
			},
			want1:   map[string]v1.Device{},
			wantErr: false,
		},
		{
			name: "delete device info",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
				},
			},
			args: args{
				node: "node1",
				old: &v1.ResourceConfigSpec{
					BEPool: 20,
					Devices: map[string]v1.Device{
						"dev1": {
							Name: "dev1",
							Type: v1.DefaultDevice,
						},
						"dev2": {
							Name:          "dev2",
							CapacityIn:    1000,
							CapacityOut:   1000,
							CapacityTotal: 2000,
							DiskSize:      "10",
						},
					},
				},
				new: &v1.ResourceConfigSpec{
					BEPool: 20,
					Devices: map[string]v1.Device{
						"dev1": {
							Name: "dev1",
							Type: v1.DefaultDevice,
						},
					},
				},
			},
			want1: map[string]v1.Device{
				"dev2": {
					Name:          "dev2",
					CapacityIn:    1000,
					CapacityOut:   1000,
					CapacityTotal: 2000,
					DiskSize:      "10",
				},
			},
			want:    map[string]v1.Device{},
			wantErr: false,
		},
		{
			name: "node not exist",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
				},
			},
			args: args{
				node: "node2",
				old: &v1.ResourceConfigSpec{
					BEPool: 20,
					Devices: map[string]v1.Device{
						"dev1": {
							Name: "dev1",
							Type: v1.DefaultDevice,
						},
						"dev2": {
							Name:          "dev2",
							CapacityIn:    1000,
							CapacityOut:   1000,
							CapacityTotal: 2000,
							DiskSize:      "10",
						},
					},
				},
				new: &v1.ResourceConfigSpec{
					BEPool: 20,
					Devices: map[string]v1.Device{
						"dev1": {
							Name: "dev1",
							Type: v1.DefaultDevice,
						},
					},
				},
			},
			want1:   nil,
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				packer:     tt.fields.packer,
				// RWMutex:    tt.fields.RWMutex,
			}
			got, got1, err := h.UpdateDeviceNodeInfo(tt.args.node, tt.args.old, tt.args.new)
			// rs := tt.fields.HandleBase.EC.GetExtendedResource(tt.args.node)
			// info := rs.(*Resource).info.DisksStatus
			// klog.Info("New disk status:", info)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle.UpdateDeviceNodeInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle.UpdateDeviceNodeInfo() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Handle.UpdateDeviceNodeInfo() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
