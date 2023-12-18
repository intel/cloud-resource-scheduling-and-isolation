/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

func TestHandle_CriticalPod(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
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
			name: "rdt annotation #1",
			args: args{
				annotations: map[string]string{
					utils.RDTConfigAnno: "class1",
				},
			},
			want: true,
		},
		{
			name: "rdt annotation #2",
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
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			if got := h.CriticalPod(tt.args.annotations); got != tt.want {
				t.Errorf("Handle.CriticalPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func fakeResourceCache(rdtClassStatus map[string]float64) ioiresource.ExtendedCache {
	ec := ioiresource.NewExtendedCache()
	rs := &Resource{
		nodeName: "node1",
		info: &NodeInfo{
			RDTClassStatus: rdtClassStatus,
		},
	}
	ec.SetExtendedResource("node1", rs)
	return ec
}

func TestHandle_CanAdmitPod(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
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
		want1   interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "can admit pod",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(map[string]float64{
						"class1": 100,
						"class2": 80,
						"class3": 60,
					}),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &RequestInfo{
					RDTClass: "class1",
				},
			},
			want: true,
			want1: map[string]float64{
				"class1": 100,
				"class2": 80,
				"class3": 60,
			},
			wantErr: false,
		},
		{
			name: "pod without rdt annotation",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(map[string]float64{
						"class1": 100,
						"class2": 80,
						"class3": 60,
					}),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &RequestInfo{
					RDTClass: "",
				},
			},
			want: true,
			want1: map[string]float64{
				"class1": 100,
				"class2": 80,
				"class3": 60,
			},
			wantErr: false,
		},
		{
			name: "There is no rdt class of pod in node",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(map[string]float64{
						"class1": 100,
						"class2": 80,
						"class3": 60,
					}),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: &RequestInfo{
					RDTClass: "class4",
				},
			},
			want:    false,
			want1:   nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			got, got1, err := h.CanAdmitPod(tt.args.nodeName, tt.args.rawRequest)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle.CanAdmitPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Handle.CanAdmitPod() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Handle.CanAdmitPod() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestHandle_NodePressureRatio(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		client     kubernetes.Interface
		// RWMutex    sync.RWMutex
	}
	type args struct {
		request     interface{}
		classStatus interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    float64
		wantErr bool
	}{
		{
			name: "get node pressure ratio",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(map[string]float64{
						"class1": 100,
						"class2": 80,
						"class3": 60,
					}),
				},
			},
			args: args{
				request: &RequestInfo{
					RDTClass: "class3",
				},
				classStatus: map[string]float64{
					"class1": 100,
					"class2": 80,
					"class3": 60,
				},
			},
			want:    0.6,
			wantErr: false,
		},
		{
			name: "pod without rdt class",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(map[string]float64{
						"class1": 100,
						"class2": 80,
						"class3": 60,
					}),
				},
			},
			args: args{
				request: &RequestInfo{
					RDTClass: "",
				},
				classStatus: map[string]float64{
					"class1": 100,
					"class2": 80,
					"class3": 60,
				},
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "There is no rdt class of pod in node",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(map[string]float64{
						"class1": 100,
						"class2": 80,
						"class3": 60,
					}),
				},
			},
			args: args{
				request: &RequestInfo{
					RDTClass: "class4",
				},
				classStatus: map[string]float64{
					"class1": 100,
					"class2": 80,
					"class3": 60,
				},
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			got, err := h.NodePressureRatio(tt.args.request, tt.args.classStatus)
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
		// RWMutex    sync.RWMutex
	}
	type args struct {
		pod  *corev1.Pod
		node string
		init bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]*pb.IOResourceRequest
		want1   interface{}
		wantErr bool
	}{
		{
			name: "emptyDir attached pod running when scheduler init, annotation wrong format",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
						Annotations: map[string]string{
							utils.RDTConfigAnno: "class1",
						},
					},
				},
			},
			want:    nil,
			want1:   "class1",
			wantErr: false,
		},
		{
			name: "emptyDir attached pod running when scheduler init, annotation wrong format",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
					},
				},
			},
			want:    nil,
			want1:   "",
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
			name:   "node has label rdt",
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
			name:   "node has label rdt",
			fields: fields{},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"ioisolation": "rdt",
						},
					},
				},
			},
			want: true,
		},
		{
			name:   "node without label rdt",
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
		want   map[string]float64
	}{
		// TODO: Add test cases.
		{
			name: "get initial rdt class status cache",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(make(map[string]float64)),
				},
			},
			args: args{
				nodeName: "node1",
				rc: v1.ResourceConfigSpec{
					Devices: map[string]v1.Device{
						"class1": {},
						"class2": {},
						"class3": {},
					},
				},
			},
			want: map[string]float64{
				"class1": 0,
				"class2": 0,
				"class3": 0,
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
			h.InitNodeStatus(tt.args.nodeName, tt.args.rc)
			got := tt.fields.HandleBase.EC.GetExtendedResource(tt.args.nodeName)
			status := got.(*Resource).info.RDTClassStatus
			if !reflect.DeepEqual(status, tt.want) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", status, tt.want)
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
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		want    map[string]float64
	}{
		// TODO: Add test cases.
		{
			name: "fill in cache",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(nil),
				},
			},
			args: args{
				nodeName: "node1",
				rc: v1.ResourceConfigSpec{
					Devices: map[string]v1.Device{
						"class1": {},
						"class2": {},
						"class3": {},
					},
				},
			},
			wantErr: false,
			want: map[string]float64{
				"class1": 0,
				"class2": 0,
				"class3": 0,
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
			info := got.(*Resource).info.RDTClassStatus
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
		// TODO: Add test cases.
		{
			name: "delete node cache",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(make(map[string]float64)),
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
		n        string
		nodeIoBw *v1.NodeIOStatusStatus
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "fill in cache",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(map[string]float64{
						"class1": 0,
						"class2": 0,
						"class3": 0,
					}),
				},
			},
			args: args{
				n: "node1",
				nodeIoBw: &v1.NodeIOStatusStatus{
					AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
						v1.WorkloadIOType(v1.RDT): {
							DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
								"class1": {
									IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
										v1.WorkloadGA: {
											Pressure: 100,
										},
									},
								},
								"class2": {
									IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
										v1.WorkloadGA: {
											Pressure: 80,
										},
									},
								},
								"class3": {
									IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
										v1.WorkloadGA: {
											Pressure: 10,
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
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				client:     tt.fields.client,
				// RWMutex:    tt.fields.RWMutex,
			}
			if err := h.UpdateCacheNodeStatus(tt.args.n, tt.args.nodeIoBw); (err != nil) != tt.wantErr {
				t.Errorf("Handle.UpdateCacheNodeStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
