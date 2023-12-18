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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

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
		{
			name: "rdt quantity annotation #1",
			args: args{
				annotations: map[string]string{
					utils.RDTQuantityConfigAnno: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"50MBps\",\"l3cache\":\"100MiB\"},\"class\":\"class1\"}",
				},
			},
			want: true,
		},
		{
			name: "rdt quantity annotation #2",
			args: args{
				annotations: map[string]string{
					utils.RDTQuantityConfigAnno: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"class\":\"class1\"}",
				},
			},
			want: true,
		},
		{
			name: "rdt quantity annotation #3",
			args: args{
				annotations: map[string]string{
					utils.RDTQuantityConfigAnno: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"}}",
				},
			},
			want: true,
		},
		{
			name: "rdt quantity annotation #4",
			args: args{
				annotations: map[string]string{
					utils.RDTQuantityConfigAnno: "",
				},
			},
			want: false,
		},
		{
			name: "rdt quantity annotation #5",
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

func fakeResourceCache(info NodeInfo) ioiresource.ExtendedCache {
	ec := ioiresource.NewExtendedCache()
	rs := &Resource{
		nodeName: "node1",
		info:     &info,
	}
	ec.SetExtendedResource("node1", rs)
	return ec
}

func TestHandle_CanAdmitPod(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		RWMutex    sync.RWMutex
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
		{
			name: "can admit pod #1",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(
						NodeInfo{
							BEScore:              100,
							L3CacheCapacity:      100,
							MBCapacity:           100,
							BEL3CapacityBaseline: 10,
							BEMBCapacityBaseline: 10,
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
					),
				},
			},
			args: args{
				nodeName: "node1",
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
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "can admit pod #2",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(
						NodeInfo{
							BEScore:              100,
							L3CacheCapacity:      100,
							MBCapacity:           100,
							BEL3CapacityBaseline: 10,
							BEMBCapacityBaseline: 10,
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
					),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &pb.IOResourceRequest{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class2",
						Request: &pb.Quantity{
							Inbps: 60,
						},
						Limit: &pb.Quantity{
							Inbps: 70,
						},
						RawRequest: &pb.Quantity{
							Inbps: 60,
						},
						RawLimit: &pb.Quantity{
							Inbps: 70,
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "can admit pod #3",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(
						NodeInfo{
							BEScore:              100,
							L3CacheCapacity:      100,
							MBCapacity:           100,
							BEL3CapacityBaseline: 10,
							BEMBCapacityBaseline: 10,
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
					),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &pb.IOResourceRequest{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "BE",
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "can not admit pod",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(
						NodeInfo{
							BEScore:              100,
							L3CacheCapacity:      100,
							MBCapacity:           100,
							BEL3CapacityBaseline: 10,
							BEMBCapacityBaseline: 10,
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
					),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &pb.IOResourceRequest{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class2",
						Request: &pb.Quantity{
							Inbps: 70,
						},
						Limit: &pb.Quantity{
							Inbps: 70,
						},
						RawRequest: &pb.Quantity{
							Inbps: 70,
						},
						RawLimit: &pb.Quantity{
							Inbps: 70,
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
				client:     tt.fields.client,
				RWMutex:    tt.fields.RWMutex,
			}
			got, _, err := h.CanAdmitPod(tt.args.nodeName, tt.args.rawRequest)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle.CanAdmitPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Handle.CanAdmitPod() got = %v, want %v", got, tt.want)
			}
			// if !reflect.DeepEqual(got1, tt.want1) {
			// 	t.Errorf("Handle.CanAdmitPod() got1 = %v, want %v", got1, tt.want1)
			// }
		})
	}
}

func TestHandle_NodePressureRatio(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		RWMutex    sync.RWMutex
	}
	type args struct {
		request  interface{}
		nodeInfo interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    float64
		wantErr bool
	}{
		{
			name: "node pressure ratio #1",
			args: args{
				request: &pb.IOResourceRequest{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class2",
						Request: &pb.Quantity{
							Inbps: 70,
						},
						Limit: &pb.Quantity{
							Inbps: 70,
						},
						RawRequest: &pb.Quantity{
							Inbps: 70,
						},
						RawLimit: &pb.Quantity{
							Inbps: 70,
						},
					},
				},
				nodeInfo: &NodeInfo{
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
			},
			want:    62.5,
			wantErr: false,
		},
		{
			name: "node pressure ratio #2",
			args: args{
				request: &pb.IOResourceRequest{
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "BE",
					},
				},
				nodeInfo: &NodeInfo{
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
			},
			want:    100,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				RWMutex:    tt.fields.RWMutex,
			}
			got, err := h.NodePressureRatio(tt.args.request, tt.args.nodeInfo)
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
		client     kubernetes.Interface
		RWMutex    sync.RWMutex
	}
	type args struct {
		pod  *corev1.Pod
		node string
		init bool
	}
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]*pb.IOResourceRequest
		want1   interface{}
		wantErr bool
	}{
		{
			name: "get request by annotation #1",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.RDTQuantityConfigAnno: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"50MBps\",\"l3cache\":\"100MiB\"},\"class\":\"class1\"}",
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"class1": {
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class1",
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
			},
			wantErr: false,
		},
		{
			name: "get request by annotation #2",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.RDTQuantityConfigAnno: "{\"requests\":{\"mb\":\"3GBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"5GBps\",\"l3cache\":\"100MiB\"},\"class\":\"class1\"}",
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"class1": {
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class1",
						Request: &pb.Quantity{
							Inbps: 40,
						},
						Limit: &pb.Quantity{
							Inbps: 100,
						},
						RawRequest: &pb.Quantity{
							Inbps: 3072,
						},
						RawLimit: &pb.Quantity{
							Inbps: 5120,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "get request by annotation #3",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.RDTQuantityConfigAnno: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"l3cache\":\"100MiB\"},\"class\":\"class1\"}",
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"class1": {
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "class1",
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
							Inbps: 30,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "get request by annotation #4",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.RDTQuantityConfigAnno: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"50MBps\",\"l3cache\":\"100MiB\"}}",
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"pod1": {
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "pod1",
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
			},
			wantErr: false,
		},
		{
			name: "get request by annotation #5",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.RDTQuantityConfigAnno: "{\"class\":\"class1\"}",
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"BE": {
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName:    "BE",
						Request:    &pb.Quantity{},
						Limit:      &pb.Quantity{},
						RawRequest: &pb.Quantity{},
						RawLimit:   &pb.Quantity{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "get request by annotation #6",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.RDTQuantityConfigAnno: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"}}",
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"pod1": {
					IoType:       pb.IOType_RDTQuantity,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "pod1",
						Request: &pb.Quantity{
							Inbps: 40,
						},
						Limit: &pb.Quantity{
							Inbps: 40,
						},
						RawRequest: &pb.Quantity{
							Inbps: 30,
						},
						RawLimit: &pb.Quantity{
							Inbps: 30,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "get request by annotation #7",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
				client: fakeKubeClient,
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.RDTQuantityConfigAnno: "{\"requests\":{\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"50MBps\",\"l3cache\":\"100MiB\"},\"class\":\"class1\"}",
						},
					},
				},
				node: "node1",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				RWMutex:    tt.fields.RWMutex,
			}
			got, got1, err := h.GetRequestByAnnotation(tt.args.pod, tt.args.node, tt.args.init)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle.GetRequestByAnnotation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Handle.GetRequestByAnnotation() got1 = %v, want %v", got1, tt.want1)
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
		// TODO: Add test cases.
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
			name:   "node has label rdtQuantity",
			fields: fields{},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"ioisolation": "rdtQuantity-disk",
						},
					},
				},
			},
			want: true,
		},
		{
			name:   "node has label rdtQuantity",
			fields: fields{},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"ioisolation": "rdtQuantity",
						},
					},
				},
			},
			want: true,
		},
		{
			name:   "node without label rdtQuantity",
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
		want   map[string]*classInfo
	}{
		{
			name: "init node status #1",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
			},
			args: args{
				nodeName: "node1",
				rc: v1.ResourceConfigSpec{
					Devices: map[string]v1.Device{},
				},
			},
			want: map[string]*classInfo{
				"BE": {
					Requests: rdtResource{
						l3: 40,
						mb: 50,
					},
					Limits: rdtResource{
						l3: 40,
						mb: 50,
					},
					PodList: []string{},
				},
				"class1": {
					Requests: rdtResource{
						l3: 30,
						mb: 20,
					},
					Limits: rdtResource{
						l3: 50,
						mb: 40,
					},
					PodList: []string{"pod1"},
				},
				"class2": {
					Requests: rdtResource{
						l3: 30,
						mb: 30,
					},
					Limits: rdtResource{
						l3: 50,
						mb: 50,
					},
					PodList: []string{"pod2"},
				},
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
											Inbps: 20,
										},
										RawLimit: &pb.Quantity{
											Inbps: 40,
										},
									},
								},
							},
						},
						"pod2": {
							PodName: "pod2",
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_RDTQuantity,
									WorkloadType: utils.GaIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "class2",
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
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			h.InitNodeStatus(tt.args.nodeName, tt.args.rc)
			got := tt.fields.HandleBase.EC.GetExtendedResource(tt.args.nodeName)
			classes := got.(*Resource).info.classes
			if !reflect.DeepEqual(classes, tt.want) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", classes, tt.want)
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
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		want    *NodeInfo
	}{
		{
			name: "fill in cache",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: ioiresource.NewExtendedCache(),
				},
				client: fakeKubeClient,
			},
			args: args{
				nodeName: "node1",
				rc: v1.ResourceConfigSpec{
					Devices: map[string]v1.Device{
						"l3": {
							Name:       "l3",
							DefaultIn:  20,
							CapacityIn: 100,
						},
						"mb": {
							Name:       "mb",
							DefaultIn:  20,
							CapacityIn: 200,
						},
					},
				},
			},
			wantErr: false,
			want: &NodeInfo{
				BEScore:              100,
				L3CacheCapacity:      100,
				MBCapacity:           200,
				BEL3CapacityBaseline: 20,
				BEMBCapacityBaseline: 20,
				classes: map[string]*classInfo{
					"BE": {
						Requests: rdtResource{
							l3: 100,
							mb: 200,
						},
						Limits: rdtResource{
							l3: 100,
							mb: 200,
						},
						PodList: []string{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
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
			info := got.(*Resource).info
			if !reflect.DeepEqual(info, tt.want) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", info, tt.want)
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
					EC: fakeResourceCache(
						NodeInfo{
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
					),
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
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
			},
			args: args{
				nodeName: "node1",
				nodeIoBw: &v1.NodeIOStatusStatus{
					ObservedGeneration: 1,
					AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
						v1.RDTQuantity: {
							DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
								"BE": {
									IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
										v1.WorkloadGA: {
											Pressure: 90,
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
					EC: fakeResourceCache(NodeInfo{
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
					}),
				},
			},
			args: args{
				nodeName: "node1",
				nodeIoBw: &v1.NodeIOStatusStatus{
					ObservedGeneration: 0,
					AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
						v1.RDTQuantity: {
							DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
								"BE": {
									IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
										v1.WorkloadGA: {
											Pressure: 90,
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
