/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package aggregator

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	fakeioiclientset "sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned/fake"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/api/nodeagent"
	testutils "sigs.k8s.io/IOIsolation/test/utils"
)

func Test_verifyEndpoint(t *testing.T) {
	type args struct {
		ip   string
		port string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "ip in wrong format",
			args: args{
				ip:   "10.22.55.99.3",
				port: "1",
			},
			want: false,
		},
		{
			name: "ip in wrong format",
			args: args{
				ip:   "10.259.55.99",
				port: "1",
			},
			want: false,
		},
		{
			name: "port in wrong format",
			args: args{
				ip:   "10.25.55.99",
				port: "666666",
			},
			want: false,
		},
		{
			name: "right format",
			args: args{
				ip:   "10.25.55.99",
				port: "66",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := verifyEndpoint(tt.args.ip, tt.args.port); got != tt.want {
				t.Errorf("verifyEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIoisolationAggregator_updateNodeIOStatus(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		// Mutex                                 sync.Mutex
	}
	type args struct {
		ctx context.Context
		obj interface{}
	}
	fakeClient := fakeioiclientset.NewSimpleClientset()
	err := testutils.MakeNodeIOStatus(fakeClient, &v1.NodeIOStatusSpec{
		NodeName:     "node1",
		Generation:   3,
		ReservedPods: map[string]v1.PodRequest{},
	},
		&v1.NodeIOStatusStatus{
			ObservedGeneration: 2,
			AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
				v1.BlockIO: {
					DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
						"dev1": {
							IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
								v1.WorkloadGA: {Pressure: 1},
								v1.WorkloadBE: {Total: 500},
							},
						},
					},
				},
			},
		})
	if err != nil {
		t.Error(err)
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "get resource and update status",
			fields: fields{
				client: fakeClient,
				nodeInfo4Sched: map[string]*v1.NodeIOStatusStatus{
					"node1": {
						ObservedGeneration: 2,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
											v1.WorkloadGA: {Pressure: 1},
											v1.WorkloadBE: {Total: 500},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				obj: "node1",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				// Mutex:                                 tt.fields.Mutex,
			}
			if err := aggr.updateNodeIOStatus(tt.args.ctx, tt.args.obj); (err != nil) != tt.wantErr {
				t.Errorf("IoisolationAggregator.updateNodeIOStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIoisolationAggregator_updatePodList(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		// Mutex                                 sync.Mutex
	}
	type args struct {
		ctx context.Context
		obj interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "send podlist with success",
			fields: fields{
				nodeInfo4Agent: map[string]*NodeInfo4Agent{},
			},
			args: args{
				ctx: context.TODO(),
				obj: &nodeagent.UpdateReservedPodsRequest{
					NodeName: "node1",
					ReservedPods: &aggregator.PodList{
						Generation: 0,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "send podlist out of date",
			fields: fields{
				nodeInfo4Agent: map[string]*NodeInfo4Agent{},
				client:         nil,
			},
			args: args{
				ctx: context.TODO(),
				obj: &nodeagent.UpdateReservedPodsRequest{
					NodeName: "node1",
					ReservedPods: &aggregator.PodList{
						Generation: -1,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				// Mutex:                                 tt.fields.Mutex,
			}
			aggr.NodeStaticIOInfoCrAdd(&v1.NodeStaticIOInfo{
				Spec: v1.NodeStaticIOInfoSpec{
					NodeName: "node1",
					EndPoint: "10.22.33.44:55",
				},
			})
			aggr.nodeInfo4Agent["node1"].client = nil
			if err := aggr.updatePodList(tt.args.ctx, tt.args.obj); (err != nil) != tt.wantErr {
				t.Errorf("IoisolationAggregator.updatePodList() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_parseEndPoint(t *testing.T) {
	type args struct {
		addr string
	}
	tests := []struct {
		name    string
		args    args
		want    *Endpoint
		wantErr bool
	}{
		{
			name: "ip in wrong format",
			args: args{
				addr: "10.22.55.99.3:1",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "ip in wrong format",
			args: args{
				addr: "10.259.55.99:1",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "port in wrong format",
			args: args{
				addr: "10.25.55.99:666666",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "right format",
			args: args{
				addr: "10.25.55.99:10",
			},
			want: &Endpoint{
				IP:   "10.25.55.99",
				Port: "10",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEndPoint(tt.args.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEndPoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseEndPoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIoisolationAggregator_WatchNodeInfo(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		// Mutex                                 sync.Mutex
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "implement list-watch",
			fields: fields{
				client: fakeioiclientset.NewSimpleClientset(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				// Mutex:                                 tt.fields.Mutex,
			}
			if err := aggr.WatchNodeInfo(); (err != nil) != tt.wantErr {
				t.Errorf("IoisolationAggregator.WatchNodeInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIoisolationAggregator_AddNodeIOStatusCR(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		// Mutex                                 sync.Mutex
	}
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "Add node io status CR",
			fields: fields{
				ReservedPodCh:  make(chan *v1.NodeIOStatusSpec, 3),
				nodeInfo4Sched: make(map[string]*v1.NodeIOStatusStatus),
				nodeInfo4Agent: make(map[string]*NodeInfo4Agent),
			},
			args: args{
				obj: &v1.NodeIOStatus{
					Spec: v1.NodeIOStatusSpec{
						NodeName: "node1",
					},
					Status: v1.NodeIOStatusStatus{
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"678-456": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
											v1.WorkloadGA: {},
											v1.WorkloadBE: {},
										},
									},
								},
							},
						},
					},
				},
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				// Mutex:                                 tt.fields.Mutex,
			}
			aggr.AddNodeIOStatusCR(tt.args.obj)
			data := len(aggr.nodeInfo4Sched["node1"].AllocatableBandwidth[v1.BlockIO].DeviceIOStatus)
			if !reflect.DeepEqual(data, tt.want) {
				t.Errorf("AddNodeIOStatusCR() = %v, want %v", data, tt.want)
			}
		})
	}
}

func TestIoisolationAggregator_UpdateNodeIOStatusCR(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		// Mutex                                 sync.Mutex
	}
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "Add new resource in node io status CR",
			fields: fields{
				ReservedPodCh: make(chan *v1.NodeIOStatusSpec, 3),
				nodeInfo4Sched: map[string]*v1.NodeIOStatusStatus{
					"node1": {
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				oldObj: &v1.NodeIOStatus{
					Spec: v1.NodeIOStatusSpec{
						NodeName: "node1",
					},
					Status: v1.NodeIOStatusStatus{
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
								},
							},
						},
					},
				},
				newObj: &v1.NodeIOStatus{
					Spec: v1.NodeIOStatusSpec{
						NodeName: "node1",
					},
					Status: v1.NodeIOStatusStatus{
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
								},
							},
							v1.NetworkIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"nic1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
								},
							},
						},
					},
				},
			},
			want: 2,
		},
		{
			name: "Add new disk in node io status CR",
			fields: fields{
				ReservedPodCh: make(chan *v1.NodeIOStatusSpec, 3),
				nodeInfo4Sched: map[string]*v1.NodeIOStatusStatus{
					"node1": {
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				oldObj: &v1.NodeIOStatus{
					Spec: v1.NodeIOStatusSpec{
						NodeName: "node1",
					},
					Status: v1.NodeIOStatusStatus{
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
								},
							},
						},
					},
				},
				newObj: &v1.NodeIOStatus{
					Spec: v1.NodeIOStatusSpec{
						NodeName: "node1",
					},
					Status: v1.NodeIOStatusStatus{
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
									"dev2": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
								},
							},
						},
					},
				},
			},
			want: 1,
		},
		{
			name: "Delete disk in node io status CR",
			fields: fields{
				ReservedPodCh: make(chan *v1.NodeIOStatusSpec, 3),
				nodeInfo4Sched: map[string]*v1.NodeIOStatusStatus{
					"node1": {
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				oldObj: &v1.NodeIOStatus{
					Spec: v1.NodeIOStatusSpec{
						NodeName: "node1",
					},
					Status: v1.NodeIOStatusStatus{
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{},
									},
								},
							},
						},
					},
				},
				newObj: &v1.NodeIOStatus{
					Spec: v1.NodeIOStatusSpec{
						NodeName: "node1",
					},
					Status: v1.NodeIOStatusStatus{
						ObservedGeneration: 1,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{},
							},
						},
					},
				},
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				// Mutex:                                 tt.fields.Mutex,
			}
			aggr.UpdateNodeIOStatusCR(tt.args.oldObj, tt.args.newObj)
			klog.Info(aggr.nodeInfo4Sched["node1"])
			data := len(aggr.nodeInfo4Sched["node1"].AllocatableBandwidth)
			if !reflect.DeepEqual(data, tt.want) {
				t.Errorf("UpdateNodeIOStatusCR() = %v, want %v", data, tt.want)
			}
		})
	}
}

func TestInitClient(t *testing.T) {
	type args struct {
		cacheInfo *NodeInfo4Agent
		nodeInfo  *v1.NodeStaticIOInfo
		mtls      bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "init client without mtls",
			args: args{
				cacheInfo: &NodeInfo4Agent{},
				nodeInfo: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						EndPoint: "100.111.222.33:7777",
					},
				},
				mtls: false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		patches := gomonkey.ApplyFunc(grpc.Dial, func(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
			return &grpc.ClientConn{}, nil
		})
		defer patches.Reset()
		t.Run(tt.name, func(t *testing.T) {
			if err := InitClient(tt.args.cacheInfo, tt.args.nodeInfo, tt.args.mtls); (err != nil) != tt.wantErr {
				t.Errorf("InitClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIoisolationAggregator_UpdateNodeStaticIOInfoCR(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		// Mutex                                 sync.Mutex
	}
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "init node",
			fields: fields{
				nodeInfo4Agent: map[string]*NodeInfo4Agent{},
				mtls:           false,
			},
			args: args{
				oldObj: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						NodeName: "node1",
					},
				},
				newObj: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						NodeName: "node1",
						EndPoint: "100.22.33.44:55",
					},
				},
			},
			want: true,
		},
		{
			name: "change endpoint",
			fields: fields{
				nodeInfo4Agent: map[string]*NodeInfo4Agent{},
				mtls:           false,
			},
			args: args{
				oldObj: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						NodeName: "node1",
						EndPoint: "100.22.33.44:56",
					},
				},
				newObj: &v1.NodeStaticIOInfo{
					Spec: v1.NodeStaticIOInfoSpec{
						NodeName: "node1",
						EndPoint: "100.22.33.44:55",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				// Mutex:                                 tt.fields.Mutex,
			}
			aggr.NodeStaticIOInfoCrUpdate(tt.args.oldObj, tt.args.newObj)
			got := aggr.nodeInfo4Agent["node1"].client
			if tt.want != (got != nil) {
				t.Errorf("UpdateNodeStaticIOInfoCR() client = %v, wantErr %v", got, tt.want)
			}
		})
	}
}

func Test_ip2FQDN(t *testing.T) {
	type args struct {
		ipaddr string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "ip to fqdn",
			args: args{
				ipaddr: "100.22.33.44",
			},
			want: "100-22-33-44.ioi-system.pod.cluster.local",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ip2FQDN(tt.args.ipaddr); got != tt.want {
				t.Errorf("ip2FQDN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIoisolationAggregator_WatchConfigMap(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		// Mutex                                 sync.Mutex
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "implement list-watch",
			fields: fields{
				coreclient: fake.NewSimpleClientset(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		utils.INF = 3
		utils.DBG = 4
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				// Mutex:                                 tt.fields.Mutex,
			}
			if err := aggr.WatchConfigMap(); (err != nil) != tt.wantErr {
				t.Errorf("IoisolationAggregator.WatchConfigMap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIoisolationAggregator_AddAdminConfigData(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		// Mutex                                 sync.Mutex
	}
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantinfo  string
		wantdebug string
	}{
		{
			name: "Info and Debug are unset",
			args: args{
				obj: &corev1.ConfigMap{
					Data: map[string]string{
						"loglevel": "",
					},
				},
			},
			wantinfo:  "3",
			wantdebug: "4",
		},
		{
			name: "Info set to true",
			args: args{
				obj: &corev1.ConfigMap{
					Data: map[string]string{
						"loglevel": "level=info\n",
					},
				},
			},
			wantinfo:  "1",
			wantdebug: "4",
		},
	}
	for _, tt := range tests {
		utils.INF = 3
		utils.DBG = 4
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				// Mutex:                                 tt.fields.Mutex,
			}
			aggr.AddAdminConfigData(tt.args.obj)
			if !reflect.DeepEqual(utils.INF.String(), tt.wantinfo) {
				t.Errorf("utils.INF %v, want %v", utils.INF, tt.wantinfo)
			}
			if !reflect.DeepEqual(utils.DBG.String(), tt.wantdebug) {
				t.Errorf("utils.DBG %v, want %v", utils.DBG, tt.wantdebug)
			}
		})
	}
}

func TestIoisolationAggregator_UpdateAdminConfigData(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		// Mutex                                 sync.Mutex
	}
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantinfo  string
		wantdebug string
	}{
		{
			name: "Info and Debug are unset",
			args: args{
				newObj: &corev1.ConfigMap{
					Data: map[string]string{
						"loglevel": "",
					},
				},
			},
			wantinfo:  "3",
			wantdebug: "4",
		},
		{
			name: "Debug set to true",
			args: args{
				newObj: &corev1.ConfigMap{
					Data: map[string]string{
						"loglevel": "level=debug\n",
					},
				},
			},
			wantinfo:  "1",
			wantdebug: "2",
		},
	}
	for _, tt := range tests {
		utils.INF = 3
		utils.DBG = 4
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				// Mutex:                                 tt.fields.Mutex,
			}
			aggr.UpdateAdminConfigData(tt.args.oldObj, tt.args.newObj)
			aggr.AddAdminConfigData(tt.args.newObj)
			if !reflect.DeepEqual(utils.INF.String(), tt.wantinfo) {
				t.Errorf("utils.INF %v, want %v", utils.INF, tt.wantinfo)
			}
			if !reflect.DeepEqual(utils.DBG.String(), tt.wantdebug) {
				t.Errorf("utils.DBG %v, want %v", utils.DBG, tt.wantdebug)
			}
		})
	}
}

func TestIoisolationAggregator_UpdateEndpointLabelToPod(t *testing.T) {
	type fields struct {
		UnimplementedIOAggregator4AgentServer aggregator.UnimplementedIOAggregator4AgentServer
		server4Agent                          *grpc.Server
		nodeInfo4Agent                        map[string]*NodeInfo4Agent
		nodeInfo4Sched                        map[string]*v1.NodeIOStatusStatus
		NodeStatusCh                          chan *aggregator.UpdateNodeIOStatusRequest
		ReservedPodCh                         chan *v1.NodeIOStatusSpec
		queue                                 workqueue.RateLimitingInterface
		timer                                 *time.Ticker
		client                                versioned.Interface
		coreclient                            kubernetes.Interface
		mtls                                  bool
		direct                                bool
		isLeader                              bool
		// Mutex                                 sync.Mutex
	}
	type args struct {
		add bool
	}
	client := fake.NewSimpleClientset()
	tests := []struct {
		name    string
		fields  fields
		args    args
		podName string
		wantErr bool
	}{
		{
			name: "add label to pod",
			fields: fields{
				coreclient: client,
			},
			args: args{
				add: true,
			},
			podName: "pod1",
			wantErr: false,
		},
		{
			name: "delete label to pod",
			fields: fields{
				coreclient: client,
			},
			args: args{
				add: false,
			},
			podName: "pod2",
			wantErr: false,
		},
	}
	_, err := testutils.MakePod(client, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "default",
		},
	})
	if err != nil {
		t.Error(err)
	}
	_, err = testutils.MakePod(client, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod2",
			Namespace: "default",
			Labels: map[string]string{
				"aggregator-role": "endpoint-aggregator",
			},
		},
	})
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggr := &IoisolationAggregator{
				UnimplementedIOAggregator4AgentServer: tt.fields.UnimplementedIOAggregator4AgentServer,
				server4Agent:                          tt.fields.server4Agent,
				nodeInfo4Agent:                        tt.fields.nodeInfo4Agent,
				nodeInfo4Sched:                        tt.fields.nodeInfo4Sched,
				NodeStatusCh:                          tt.fields.NodeStatusCh,
				ReservedPodCh:                         tt.fields.ReservedPodCh,
				queue:                                 tt.fields.queue,
				timer:                                 tt.fields.timer,
				client:                                tt.fields.client,
				coreclient:                            tt.fields.coreclient,
				mtls:                                  tt.fields.mtls,
				direct:                                tt.fields.direct,
				isLeader:                              tt.fields.isLeader,
				// Mutex:                                 tt.fields.Mutex,
			}
			os.Setenv("POD_NAME", tt.podName)
			os.Setenv("POD_NAMESPACE", "default")
			if err := aggr.UpdateEndpointLabelToPod(tt.args.add); (err != nil) != tt.wantErr {
				t.Errorf("IoisolationAggregator.UpdateEndpointLabelToPod() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
