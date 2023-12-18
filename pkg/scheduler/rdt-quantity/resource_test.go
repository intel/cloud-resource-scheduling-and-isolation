/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdtquantity

import (
	"reflect"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

func TestResource_AddPod(t *testing.T) {
	type fields struct {
		nodeName string
		info     *NodeInfo
		ch       resource.CacheHandle
		RWMutex  sync.RWMutex
	}
	type args struct {
		pod        *v1.Pod
		rawRequest interface{}
		client     kubernetes.Interface
	}
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*pb.IOResourceRequest
		want1   bool
		wantErr bool
		want2   map[string]*classInfo
	}{
		{
			name: "add pod #1",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					BEScore:              100,
					L3CacheCapacity:      100,
					MBCapacity:           100,
					BEL3CapacityBaseline: 20,
					BEMBCapacityBaseline: 20,
					classes: map[string]*classInfo{
						"BE": {
							Requests: rdtResource{
								l3: 100,
								mb: 100,
							},
							Limits: rdtResource{
								l3: 100,
								mb: 100,
							},
							PodList: []string{},
						},
					},
				},
				ch: &Handle{
					HandleBase: resource.HandleBase{
						EC: fakeResourceCache(NodeInfo{}),
					},
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						UID:       "pod1",
					},
				},
				rawRequest: &pb.IOResourceRequest{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class1",
						Request: &pb.Quantity{
							Inbps: 30,
						},
						Limit: &pb.Quantity{
							Inbps: 50,
						},
						RawRequest: &pb.Quantity{
							Inbps: 30,
						},
						RawLimit: &pb.Quantity{
							Inbps: 50,
						},
					},
				},
				client: fakeKubeClient,
			},
			want: []*pb.IOResourceRequest{
				{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class1",
						Request: &pb.Quantity{
							Inbps: 30,
						},
						Limit: &pb.Quantity{
							Inbps: 50,
						},
						RawRequest: &pb.Quantity{
							Inbps: 30,
						},
						RawLimit: &pb.Quantity{
							Inbps: 50,
						},
					},
				},
			},
			want1:   true,
			wantErr: false,
			want2: map[string]*classInfo{
				"BE": {
					Requests: rdtResource{
						l3: 70,
						mb: 70,
					},
					Limits: rdtResource{
						l3: 70,
						mb: 70,
					},
					PodList: []string{},
				},
				"class1": {
					Requests: rdtResource{
						l3: 30,
						mb: 30,
					},
					Limits: rdtResource{
						l3: 50,
						mb: 50,
					},
					PodList: []string{"pod1"},
				},
			},
		},
		{
			name: "add pod #2",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					BEScore:              100,
					L3CacheCapacity:      100,
					MBCapacity:           100,
					BEL3CapacityBaseline: 20,
					BEMBCapacityBaseline: 20,
					classes: map[string]*classInfo{
						"BE": {
							Requests: rdtResource{
								l3: 100,
								mb: 100,
							},
							Limits: rdtResource{
								l3: 100,
								mb: 100,
							},
							PodList: []string{},
						},
					},
				},
				ch: &Handle{
					HandleBase: resource.HandleBase{
						EC: fakeResourceCache(NodeInfo{}),
					},
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						UID:       "pod1",
					},
				},
				rawRequest: &pb.IOResourceRequest{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "BE",
					},
				},
				client: fakeKubeClient,
			},
			want: []*pb.IOResourceRequest{
				{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "BE",
					},
				},
			},
			want1:   true,
			wantErr: false,
			want2: map[string]*classInfo{
				"BE": {
					Requests: rdtResource{
						l3: 100,
						mb: 100,
					},
					Limits: rdtResource{
						l3: 100,
						mb: 100,
					},
					PodList: []string{"pod1"},
				},
			},
		},
		{
			name: "add pod #3",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					BEScore:              100,
					L3CacheCapacity:      100,
					MBCapacity:           100,
					BEL3CapacityBaseline: 20,
					BEMBCapacityBaseline: 20,
					classes: map[string]*classInfo{
						"BE": {
							Requests: rdtResource{
								l3: 70,
								mb: 70,
							},
							Limits: rdtResource{
								l3: 70,
								mb: 70,
							},
							PodList: []string{},
						},
						"class1": {
							Requests: rdtResource{
								l3: 30,
								mb: 30,
							},
							Limits: rdtResource{
								l3: 50,
								mb: 50,
							},
							PodList: []string{"pod1"},
						},
					},
				},
				ch: &Handle{
					HandleBase: resource.HandleBase{
						EC: fakeResourceCache(NodeInfo{}),
					},
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod2",
						Namespace: "default",
						UID:       "pod2",
					},
				},
				rawRequest: &pb.IOResourceRequest{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class1",
						Request: &pb.Quantity{
							Inbps: 30,
						},
						Limit: &pb.Quantity{
							Inbps: 50,
						},
						RawRequest: &pb.Quantity{
							Inbps: 30,
						},
						RawLimit: &pb.Quantity{
							Inbps: 50,
						},
					},
				},
				client: fakeKubeClient,
			},
			want: []*pb.IOResourceRequest{
				{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class1",
						Request: &pb.Quantity{
							Inbps: 30,
						},
						Limit: &pb.Quantity{
							Inbps: 50,
						},
						RawRequest: &pb.Quantity{
							Inbps: 30,
						},
						RawLimit: &pb.Quantity{
							Inbps: 50,
						},
					},
				},
			},
			want1:   true,
			wantErr: false,
			want2: map[string]*classInfo{
				"BE": {
					Requests: rdtResource{
						l3: 70,
						mb: 70,
					},
					Limits: rdtResource{
						l3: 70,
						mb: 70,
					},
					PodList: []string{},
				},
				"class1": {
					Requests: rdtResource{
						l3: 30,
						mb: 30,
					},
					Limits: rdtResource{
						l3: 50,
						mb: 50,
					},
					PodList: []string{"pod1", "pod2"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &Resource{
				nodeName: tt.fields.nodeName,
				info:     tt.fields.info,
				ch:       tt.fields.ch,
				RWMutex:  tt.fields.RWMutex,
			}
			got, got1, err := ps.AddPod(tt.args.pod, tt.args.rawRequest, tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resource.AddPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Resource.AddPod() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Resource.AddPod() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(ps.info.classes, tt.want2) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", ps.info.classes, tt.want2)
			}
		})
	}
}

func TestResource_AdmitPod(t *testing.T) {
	type fields struct {
		nodeName string
		info     *NodeInfo
		ch       resource.CacheHandle
		// RWMutex  sync.RWMutex
	}
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.IOResourceRequest
		wantErr bool
	}{
		{
			name: "Admit new pod",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					BEScore:              100,
					L3CacheCapacity:      100,
					MBCapacity:           100,
					BEL3CapacityBaseline: 20,
					BEMBCapacityBaseline: 20,
					classes: map[string]*classInfo{
						"BE": {
							Requests: rdtResource{
								l3: 70,
								mb: 70,
							},
							Limits: rdtResource{
								l3: 70,
								mb: 70,
							},
							PodList: []string{},
						},
						"class1": {
							Requests: rdtResource{
								l3: 30,
								mb: 30,
							},
							Limits: rdtResource{
								l3: 50,
								mb: 50,
							},
							PodList: []string{"pod1"},
						},
					},
				},
				ch: &Handle{
					HandleBase: resource.HandleBase{
						EC: fakeResourceCache(NodeInfo{}),
					},
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.RDTQuantityConfigAnno: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"50MBps\",\"l3cache\":\"100MiB\"},\"class\":\"class2\"}",
						},
					},
				},
			},
			want: &pb.IOResourceRequest{
				IoType:       pb.IOType_RDTQuantity,
				WorkloadType: utils.GaIndex,
				DevRequest: &pb.DeviceRequirement{
					DevName: "class2",
					Request: &pb.Quantity{
						Inbps: 40,
					},
					Limit: &pb.Quantity{
						Inbps: 100,
					},
					RawRequest: &pb.Quantity{
						Inbps: 30,
					},
					RawLimit: &pb.Quantity{
						Inbps: 50,
					},
				},
			},
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
			got, err := ps.AdmitPod(tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resource.AdmitPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			st := got.(*pb.IOResourceRequest)
			if !reflect.DeepEqual(st, tt.want) {
				t.Errorf("Resource.AdmitPod() = %v, want %v", st, tt.want)
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
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()
	type args struct {
		pod    *corev1.Pod
		client kubernetes.Interface
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		want    map[string]*classInfo
	}{
		{
			name: "remove pod #1",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					BEScore:              100,
					L3CacheCapacity:      100,
					MBCapacity:           100,
					BEL3CapacityBaseline: 20,
					BEMBCapacityBaseline: 20,
					classes: map[string]*classInfo{
						"BE": {
							Requests: rdtResource{
								l3: 70,
								mb: 70,
							},
							Limits: rdtResource{
								l3: 70,
								mb: 70,
							},
							PodList: []string{},
						},
						"class1": {
							Requests: rdtResource{
								l3: 30,
								mb: 30,
							},
							Limits: rdtResource{
								l3: 50,
								mb: 50,
							},
							PodList: []string{"pod1"},
						},
					},
				},
				ch: &Handle{
					HandleBase: resource.HandleBase{
						EC: fakeResourceCache(NodeInfo{}),
					},
				},
			},
			args: args{
				client: fakeKubeClient,
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID:       "pod1",
						Name:      "pod1",
						Namespace: "default",
					},
				},
			},
			wantErr: false,
			want: map[string]*classInfo{
				"BE": {
					Requests: rdtResource{
						l3: 100,
						mb: 100,
					},
					Limits: rdtResource{
						l3: 100,
						mb: 100,
					},
					PodList: []string{},
				},
			},
		},
		{
			name: "remove pod #2",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					BEScore:              100,
					L3CacheCapacity:      100,
					MBCapacity:           100,
					BEL3CapacityBaseline: 20,
					BEMBCapacityBaseline: 20,
					classes: map[string]*classInfo{
						"BE": {
							Requests: rdtResource{
								l3: 70,
								mb: 70,
							},
							Limits: rdtResource{
								l3: 70,
								mb: 70,
							},
							PodList: []string{},
						},
						"class1": {
							Requests: rdtResource{
								l3: 30,
								mb: 30,
							},
							Limits: rdtResource{
								l3: 50,
								mb: 50,
							},
							PodList: []string{"pod1", "pod2"},
						},
					},
				},
				ch: &Handle{
					HandleBase: resource.HandleBase{
						EC: fakeResourceCache(NodeInfo{}),
					},
				},
			},
			args: args{
				client: fakeKubeClient,
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID:       "pod2",
						Name:      "pod2",
						Namespace: "default",
					},
				},
			},
			wantErr: false,
			want: map[string]*classInfo{
				"BE": {
					Requests: rdtResource{
						l3: 70,
						mb: 70,
					},
					Limits: rdtResource{
						l3: 70,
						mb: 70,
					},
					PodList: []string{},
				},
				"class1": {
					Requests: rdtResource{
						l3: 30,
						mb: 30,
					},
					Limits: rdtResource{
						l3: 50,
						mb: 50,
					},
					PodList: []string{"pod1"},
				},
			},
		},
	}
	for _, tt := range tests {
		resource.IoiContext = &resource.ResourceIOContext{
			Reservedpod: map[string]*pb.PodList{
				"node1": {
					PodList: map[string]*pb.PodRequest{
						"pod1": {
							PodName: "pod1",
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_RDTQuantity,
									WorkloadType: utils.GaIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "class1",
										Request: &pb.Quantity{
											Inbps: 30,
										},
										Limit: &pb.Quantity{
											Inbps: 50,
										},
										RawRequest: &pb.Quantity{
											Inbps: 30,
										},
										RawLimit: &pb.Quantity{
											Inbps: 50,
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
			if !reflect.DeepEqual(ps.info.classes, tt.want) {
				t.Errorf("Resource.RemovePod() got = %v, want %v", ps.info.classes, tt.want)
			}
		})
	}
}
