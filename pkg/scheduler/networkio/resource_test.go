/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package networkio

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

func TestResource_AddPod(t *testing.T) {
	type fields struct {
		nodeName string
		info     *NodeInfo
		ch       resource.CacheHandle
		// RWMutex  sync.RWMutex
	}
	type args struct {
		pod        *v1.Pod
		rawRequest interface{}
		client     kubernetes.Interface
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "add pod",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					MasterNic: "00.00.00.00.00.00",
					NICsStatus: map[string]*NICInfo{
						"00.00.00.00.00.00": {
							GA: &utils.IOPoolStatus{
								Total: 100,
								In:    50,
								Out:   50,
							},
							BE: &utils.IOPoolStatus{
								Num: 1,
							},
						},
					},
				},
				ch: &Handle{
					HandleBase: ioiresource.HandleBase{
						EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}),
					},
				},
			},
			args: args{
				pod: &v1.Pod{},
				rawRequest: map[string]*aggregator.IOResourceRequest{
					"00.00.00.00.00.00": {
						IoType:       aggregator.IOType_NetworkIO,
						WorkloadType: utils.GaIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "00.00.00.00.00.00",
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
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &Resource{
				nodeName: tt.fields.nodeName,
				info:     tt.fields.info,
				ch:       tt.fields.ch,
				// RWMutex:  tt.fields.RWMutex,
			}
			_, got1, err := ps.AddPod(tt.args.pod, tt.args.rawRequest, tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resource.AddPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got1 != tt.want {
				t.Errorf("Resource.AddPod() got1 = %v, want %v", got1, tt.want)
			}
		})
	}
}

func TestResource_RemovePod(t *testing.T) {
	type fields struct {
		nodeName string
		info     *NodeInfo
		ch       resource.CacheHandle
		// RWMutex  sync.RWMutex
	}
	type args struct {
		pod    *v1.Pod
		client kubernetes.Interface
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "remove pod",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					MasterNic: "00.00.00.00.00.00",
					NICsStatus: map[string]*NICInfo{
						"00.00.00.00.00.00": {
							GA: &utils.IOPoolStatus{
								Total: 100,
								In:    50,
								Out:   50,
							},
							BE: &utils.IOPoolStatus{
								Num: 1,
							},
						},
					},
				},
				ch: &Handle{
					HandleBase: ioiresource.HandleBase{
						EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}),
					},
				},
			},
			args: args{
				pod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID:  "pod1",
						Name: "pod1-name",
					},
				},
				client: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		ioiresource.IoiContext = &ioiresource.ResourceIOContext{
			Reservedpod: map[string]*pb.PodList{
				"node1": {
					PodList: map[string]*pb.PodRequest{
						"pod1": {
							PodName: "pod1-name",
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_NetworkIO,
									WorkloadType: utils.GaIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "00.00.00.00.00.00",
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
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			ps := &Resource{
				nodeName: tt.fields.nodeName,
				info:     tt.fields.info,
				ch:       tt.fields.ch,
				// RWMutex:  tt.fields.RWMutex,
			}
			if err := ps.RemovePod(tt.args.pod, tt.args.client); (err != nil) != tt.wantErr {
				t.Errorf("Resource.RemovePod() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
