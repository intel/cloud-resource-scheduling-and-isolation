/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package networkio

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

func fakeResourceCache(ga, be *utils.IOPoolStatus) ioiresource.ExtendedCache {
	ec := ioiresource.NewExtendedCache()
	rs := &Resource{
		nodeName: "node1",
		info: &NodeInfo{
			MasterNic: "00.00.00.00.00.00",
			NICsStatus: map[string]*NICInfo{
				"00.00.00.00.00.00": {
					DefaultSpeed: 10,
					GA:           ga,
					BE:           be,
				},
			},
		},
	}
	ec.SetExtendedResource("node1", rs)
	return ec
}
func TestHandle_parsePodNetworkIOAnnotations(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
		// RWMutex    sync.RWMutex
	}
	type args struct {
		podAnno PodAnnoThroughput
		info    *NodeInfo
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *pb.IOResourceRequest
	}{
		// TODO: Add test cases.
		{
			name: "GA case #1",
			args: args{
				podAnno: PodAnnoThroughput{
					Requests: NetIOConfig{
						Ingress: "100M",
						Egress:  "100M",
					},
					Limits: NetIOConfig{
						Ingress: "100M",
						Egress:  "100M",
					},
				},
				info: &NodeInfo{
					MasterNic: "00.00.00.00.00.00",
					NICsStatus: map[string]*NICInfo{
						"00.00.00.00.00.00": {
							DefaultSpeed: 10,
						},
					},
				},
			},
			want: &pb.IOResourceRequest{
				IoType:       pb.IOType_NetworkIO,
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
		{
			name: "BT case #1",
			args: args{
				podAnno: PodAnnoThroughput{
					Requests: NetIOConfig{
						Ingress: "100M",
						Egress:  "100M",
					},
					Limits: NetIOConfig{
						Ingress: "200M",
						Egress:  "200M",
					},
				},
				info: &NodeInfo{
					MasterNic: "00.00.00.00.00.00",
					NICsStatus: map[string]*NICInfo{
						"00.00.00.00.00.00": {
							DefaultSpeed: 10,
						},
					},
				},
			},
			want: &pb.IOResourceRequest{
				IoType:       pb.IOType_NetworkIO,
				WorkloadType: utils.GaIndex,
				DevRequest: &pb.DeviceRequirement{
					DevName: "00.00.00.00.00.00",
					Request: &pb.Quantity{
						Inbps:  100,
						Outbps: 100,
					},
					Limit: &pb.Quantity{
						Inbps:  200,
						Outbps: 200,
					},
				},
			},
		},
		{
			name: "BE case #1",
			args: args{
				podAnno: PodAnnoThroughput{},
				info: &NodeInfo{
					MasterNic:  "00.00.00.00.00.00",
					NICsStatus: nil,
				},
			},
			want: &pb.IOResourceRequest{
				IoType:       pb.IOType_NetworkIO,
				WorkloadType: utils.BeIndex,
				DevRequest: &pb.DeviceRequirement{
					DevName: "00.00.00.00.00.00",
					Request: &pb.Quantity{},
					Limit:   &pb.Quantity{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
				// RWMutex:    tt.fields.RWMutex,
			}
			if got := h.parsePodNetworkIOAnnotations(tt.args.podAnno, tt.args.info); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle.parsePodNetworkIOAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			name: "network io annotation #1",
			args: args{
				annotations: map[string]string{
					utils.NetworkIOConfigAnno: "{\"requests\":{\"ingress\":\"50M\",\"egress\":\"50M\"},\"limits\":{\"ingress\":\"50M\",\"egress\":\"50M\"}}",
				},
			},
			want: true,
		},
		{
			name: "network io annotation #2",
			args: args{
				annotations: map[string]string{
					utils.NetworkIOConfigAnno: "{\"requests\":{\"ingress\":\"50M\",\"egress\":\"50M/4k\"}}",
				},
			},
			want: true,
		},
		{
			name: "network io annotation #3",
			args: args{
				annotations: map[string]string{
					utils.NetworkIOConfigAnno: "",
				},
			},
			want: false,
		},
		{
			name: "network io annotation #4",
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

func TestHandle_CanAdmitPod(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
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
		{
			name: "GA pod can schedule",
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
					}),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: map[string]*pb.IOResourceRequest{
					"00.00.00.00.00.00": {
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
			want:    true,
			wantErr: false,
		},
		{
			name: "BE pod can schedule",
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
					}),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: map[string]*pb.IOResourceRequest{
					"00.00.00.00.00.00": {
						IoType:       pb.IOType_NetworkIO,
						WorkloadType: utils.BeIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "00.00.00.00.00.00",
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "BE pod can't schedule",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 10,
						In:    5,
						Out:   5,
					}),
				},
			},
			args: args{
				nodeName: "node1",
				rawRequest: map[string]*pb.IOResourceRequest{
					"00.00.00.00.00.00": {
						IoType:       pb.IOType_NetworkIO,
						WorkloadType: utils.BeIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "00.00.00.00.00.00",
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
			args: args{
				request: map[string]*pb.IOResourceRequest{
					"00.00.00.00.00.00": {
						IoType:       pb.IOType_NetworkIO,
						WorkloadType: utils.BeIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "00.00.00.00.00.00",
						},
					},
				},
				pool: map[string]map[string]*utils.IOPoolStatus{
					"00.00.00.00.00.00": {
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
		{
			name: "request device number is 0",
			args: args{
				request: map[string]*pb.IOResourceRequest{},
				pool: map[string]map[string]*utils.IOPoolStatus{
					"00.00.00.00.00.00": {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handle{
				HandleBase: tt.fields.HandleBase,
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
		wantErr bool
	}{
		{
			name: "GA pod",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 10,
						In:    5,
						Out:   5,
					}),
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.NetworkIOConfigAnno: "{\"requests\":{\"ingress\":\"50M\",\"egress\":\"50M\"},\"limits\":{\"ingress\":\"50M\",\"egress\":\"50M\"}}",
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"00.00.00.00.00.00": {
					IoType:       pb.IOType_NetworkIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "00.00.00.00.00.00",
						Request: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
						Limit: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "BT pod #1",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 10,
						In:    5,
						Out:   5,
					}),
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.NetworkIOConfigAnno: "{\"limits\":{\"ingress\":\"50M\",\"egress\":\"50M\"}}",
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"00.00.00.00.00.00": {
					IoType:       pb.IOType_NetworkIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "00.00.00.00.00.00",
						Request: &pb.Quantity{
							Inbps:  10,
							Outbps: 10,
						},
						Limit: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "BT pod #2",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 10,
						In:    5,
						Out:   5,
					}),
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.NetworkIOConfigAnno: "{\"requests\":{\"ingress\":\"50M\",\"egress\":\"50M\"}}",
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"00.00.00.00.00.00": {
					IoType:       pb.IOType_NetworkIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "00.00.00.00.00.00",
						Request: &pb.Quantity{
							Inbps:  50,
							Outbps: 50,
						},
						Limit: &pb.Quantity{
							Inbps:  utils.MAX_BT_LIMIT,
							Outbps: utils.MAX_BT_LIMIT,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "BE pod #1",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 10,
						In:    5,
						Out:   5,
					}),
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
						Annotations: map[string]string{
							utils.NetworkIOConfigAnno: "{\"limit\":{\"ingress\":\"50M\",\"egress\":\"50M\"}}}}", //false format
						},
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"00.00.00.00.00.00": {
					IoType:       pb.IOType_NetworkIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "00.00.00.00.00.00",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "BE pod #2",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 10,
						In:    5,
						Out:   5,
					}),
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
					},
				},
				node: "node1",
			},
			want: map[string]*pb.IOResourceRequest{
				"00.00.00.00.00.00": {
					IoType:       pb.IOType_NetworkIO,
					WorkloadType: utils.BeIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "00.00.00.00.00.00",
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
				// RWMutex:    tt.fields.RWMutex,
			}
			got, _, err := h.GetRequestByAnnotation(tt.args.pod, tt.args.node, tt.args.init)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle.GetRequestByAnnotation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandle_InitBENum(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
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
		{
			name: "calcultate BE num",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 10,
						In:    5,
						Out:   5,
						Num:   0,
					}),
				},
			},
			args: args{
				nodeName: "node1",
				rc: v1.ResourceConfigSpec{
					Devices: map[string]v1.Device{
						"00.00.00.00.00.00": {},
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
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_NetworkIO,
									WorkloadType: utils.BeIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "00.00.00.00.00.00",
									},
								},
							},
						},
						"pod2": {
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_NetworkIO,
									WorkloadType: utils.BeIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "00.00.00.00.00.00",
									},
								},
							},
						},
						"pod3": {
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_NetworkIO,
									WorkloadType: utils.GaIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "00.00.00.00.00.00",
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
				// RWMutex:    tt.fields.RWMutex,
			}
			h.InitBENum(tt.args.nodeName, tt.args.rc)
			got := tt.fields.HandleBase.EC.GetExtendedResource(tt.args.nodeName)
			be := got.(*Resource).info.NICsStatus["00.00.00.00.00.00"].BE.Num
			if !reflect.DeepEqual(be, tt.want) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", be, tt.want)
			}
		})
	}
}

func TestHandle_InitNodeStatus(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
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
			name: "get initial cache",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}),
				},
			},
			args: args{
				nodeName: "node1",
				rc: v1.ResourceConfigSpec{
					BEPool: 20,
					Devices: map[string]v1.Device{
						"00.00.00.00.00.00": {
							Name:          "00.00.00.00.00.00",
							Type:          v1.DefaultDevice,
							CapacityIn:    1000,
							CapacityOut:   1000,
							CapacityTotal: 2000,
						},
					},
				},
			},
			want1: &utils.IOPoolStatus{
				Total: 1400,
				In:    700,
				Out:   700,
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
									IoType:       pb.IOType_NetworkIO,
									WorkloadType: utils.GaIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "00.00.00.00.00.00",
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
									IoType:       pb.IOType_NetworkIO,
									WorkloadType: utils.BeIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "00.00.00.00.00.00",
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
				// RWMutex:    tt.fields.RWMutex,
			}
			h.InitNodeStatus(tt.args.nodeName, tt.args.rc)
			got := tt.fields.HandleBase.EC.GetExtendedResource(tt.args.nodeName)
			ga := got.(*Resource).info.NICsStatus["00.00.00.00.00.00"].GA
			be := got.(*Resource).info.NICsStatus["00.00.00.00.00.00"].BE
			if !reflect.DeepEqual(ga, tt.want1) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", ga, tt.want1)
			}
			if !reflect.DeepEqual(be, tt.want2) {
				t.Errorf("Handle.GetRequestByAnnotation() got = %v, want %v", be, tt.want2)
			}
		})
	}
}

func TestHandle_NodeHasIoiLabel(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
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
			name:   "node has label net",
			fields: fields{},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"ioisolation": "rdt-net",
						},
					},
				},
			},
			want: true,
		},
		{
			name:   "node has label net",
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
							"ioisolation": "disk",
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
				// RWMutex:    tt.fields.RWMutex,
			}
			if got := h.NodeHasIoiLabel(tt.args.node); got != tt.want {
				t.Errorf("Handle.NodeHasIoiLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandle_UpdateCacheNodeStatus(t *testing.T) {
	type fields struct {
		HandleBase ioiresource.HandleBase
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
		{
			name: "update cache success",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 10,
						In:    5,
						Out:   5,
						Num:   0,
					}),
				},
			},
			args: args{
				n: "node1",
				nodeIoBw: &v1.NodeIOStatusStatus{
					ObservedGeneration: 1,
					AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
						v1.NetworkIO: {
							DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
								"00.00.00.00.00.00": {
									IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
										v1.WorkloadGA: {
											Total:    50,
											In:       25,
											Out:      25,
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
			name: "update cache fails",
			fields: fields{
				HandleBase: ioiresource.HandleBase{
					EC: fakeResourceCache(&utils.IOPoolStatus{
						Total: 100,
						In:    50,
						Out:   50,
					}, &utils.IOPoolStatus{
						Total: 10,
						In:    5,
						Out:   5,
						Num:   0,
					}),
				},
			},
			args: args{
				n: "node0",
				nodeIoBw: &v1.NodeIOStatusStatus{
					ObservedGeneration: 1,
					AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
						v1.NetworkIO: {
							DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
								"00.00.00.00.00.00": {
									IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
										v1.WorkloadGA: {
											Total:    50,
											In:       25,
											Out:      25,
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
			wantErr: true,
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
				// RWMutex:    tt.fields.RWMutex,
			}
			if err := h.UpdateCacheNodeStatus(tt.args.n, tt.args.nodeIoBw); (err != nil) != tt.wantErr {
				t.Errorf("Handle.UpdateCacheNodeStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
