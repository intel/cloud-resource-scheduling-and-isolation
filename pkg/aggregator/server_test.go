/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package aggregator

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/api/nodeagent"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	tu "sigs.k8s.io/IOIsolation/test/utils"
)

func TestIoisolationAggregator_GetPodList(t *testing.T) {
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
		gen   int64
		pods  map[string]v1.PodRequest
		added map[string]v1.PodRequest
	}
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()
	_, err := tu.MakePod(fakeKubeClient, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pod1",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
	})
	if err != nil {
		t.Error(err)
	}
	_, err = tu.MakePod(fakeKubeClient, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod2",
			Namespace: "default",
			Annotations: map[string]string{
				"ioi.intel.com/allocated-io": "{\"1\": {\"dev1\":{\"qos\": 0,\"request\": {\"inbps\": 500,\"outbps\": 500},\"limit\": {\"inbps\": 500,\"outbps\": 500}}}}",
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
		want    *aggregator.PodList
		wantErr bool
	}{
		{
			name: "test get pod list from added pod list",
			fields: fields{
				coreclient: fakeKubeClient,
			},
			args: args{
				gen: 2,
				pods: map[string]v1.PodRequest{
					"pod1": {
						Name:      "pod1",
						Namespace: "default",
					},
					"pod2": {
						Name:      "pod2",
						Namespace: "default",
					},
				},
				added: map[string]v1.PodRequest{
					"pod2": {
						Name:      "pod2",
						Namespace: "default",
					},
				},
			},
			want: &aggregator.PodList{
				Generation: 2,
				PodList: map[string]*aggregator.PodRequest{
					"pod1": nil,
					"pod2": {
						PodName:      "pod2",
						PodNamespace: "default",
						Request: []*aggregator.IOResourceRequest{
							{
								IoType:       aggregator.IOType_DiskIO,
								WorkloadType: utils.GaIndex,
								DevRequest: &aggregator.DeviceRequirement{
									DevName: "dev1",
									Request: &aggregator.Quantity{
										Inbps:  500,
										Outbps: 500,
									},
									Limit: &aggregator.Quantity{
										Inbps:  500,
										Outbps: 500,
									},
									RawRequest: &aggregator.Quantity{},
									RawLimit:   &aggregator.Quantity{},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},

		{
			name: "test get pod list from added pod list, both pods",
			fields: fields{
				coreclient: fakeKubeClient,
			},
			args: args{
				gen: 2,
				pods: map[string]v1.PodRequest{
					"pod1": {
						Name:      "pod1",
						Namespace: "default",
					},
					"pod2": {
						Name:      "pod2",
						Namespace: "default",
					},
				},
				added: map[string]v1.PodRequest{
					"pod1": {
						Name:      "pod1",
						Namespace: "default",
					},
					"pod2": {
						Name:      "pod2",
						Namespace: "default",
					},
				},
			},
			want: &aggregator.PodList{
				Generation: 2,
				PodList: map[string]*aggregator.PodRequest{
					"pod1": nil,
					"pod2": {
						PodName:      "pod2",
						PodNamespace: "default",
						Request: []*aggregator.IOResourceRequest{
							{
								IoType:       aggregator.IOType_DiskIO,
								WorkloadType: utils.GaIndex,
								DevRequest: &aggregator.DeviceRequirement{
									DevName: "dev1",
									Request: &aggregator.Quantity{
										Inbps:  500,
										Outbps: 500,
									},
									Limit: &aggregator.Quantity{
										Inbps:  500,
										Outbps: 500,
									},
									RawRequest: &aggregator.Quantity{},
									RawLimit:   &aggregator.Quantity{},
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
			got, err := aggr.GetPodList(tt.args.gen, tt.args.pods, tt.args.added)
			if (err != nil) != tt.wantErr {
				t.Errorf("IoisolationAggregator.GetPodList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want.String()) {
				t.Errorf("IoisolationAggregator.GetPodList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIoisolationAggregator_updateNodeIOStatusCache(t *testing.T) {
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
		req *aggregator.UpdateNodeIOStatusRequest
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   v1.NodeIOStatusStatus
	}{
		{
			name: "update status cache",
			fields: fields{
				nodeInfo4Sched: map[string]*v1.NodeIOStatusStatus{
					"node1": {
						ObservedGeneration: 2,
						AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
							v1.BlockIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"dev1": {
										IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
											v1.WorkloadGA: {},
											v1.WorkloadBE: {},
										},
									},
								},
							},
							v1.NetworkIO: {
								DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
									"nic1": {
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
			args: args{
				req: &aggregator.UpdateNodeIOStatusRequest{
					NodeName:           "node1",
					ObservedGeneration: 3,
					IoBw: []*aggregator.IOBandwidth{
						{
							IoType: aggregator.IOType_DiskIO,
							DeviceBwInfo: map[string]*aggregator.DeviceBandwidthInfo{
								"dev1": {
									Name: "dev1",
									Status: map[string]*aggregator.IOStatus{
										"GA": {Pressure: 1},
										"BE": {Pressure: 1},
									},
								},
							},
						},
						{
							IoType: aggregator.IOType_NetworkIO,
							DeviceBwInfo: map[string]*aggregator.DeviceBandwidthInfo{
								"nic1": {
									Name: "nic1",
									Status: map[string]*aggregator.IOStatus{
										"GA": {Pressure: 1},
										"BE": {Pressure: 1},
									},
								},
							},
						},
					},
				},
			},
			want: v1.NodeIOStatusStatus{
				ObservedGeneration: 3,
				AllocatableBandwidth: map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{
					v1.BlockIO: {
						DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
							"dev1": {
								IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
									v1.WorkloadGA: {Pressure: 1},
									v1.WorkloadBE: {Pressure: 1},
								},
							},
						},
					},
					v1.NetworkIO: {
						DeviceIOStatus: map[string]v1.IOAllocatableBandwidth{
							"nic1": {
								IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
									v1.WorkloadGA: {Pressure: 1},
									v1.WorkloadBE: {Pressure: 1},
								},
							},
						},
					},
				},
			},
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
			aggr.updateNodeIOStatusCache(tt.args.req)
			got := aggr.nodeInfo4Sched["node1"]
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("IoisolationAggregator.GetPodList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIoisolationAggregator_UpdateNodeIOStatus(t *testing.T) {
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
		ctx context.Context
		req *aggregator.UpdateNodeIOStatusRequest
	}
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Second)
	var conn *grpc.ClientConn
	client := nodeagent.NewNodeAgentClient(conn)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *aggregator.Empty
		wantErr bool
	}{
		{
			name: "test Update node status grpc server handler, generation out of date",
			fields: fields{
				direct:       false,
				NodeStatusCh: make(chan *aggregator.UpdateNodeIOStatusRequest, 2),
				nodeInfo4Sched: map[string]*v1.NodeIOStatusStatus{
					"node1": {},
				},
				queue: workqueue.NewNamedRateLimitingQueue(rateLimiter, "aggregator"),
				nodeInfo4Agent: map[string]*NodeInfo4Agent{
					"node1": {
						latestGeneration: 3,
						reservedPod: map[string]v1.PodRequest{
							"pod1": {
								Name:      "pod1",
								Namespace: "default",
							},
						},
						client: client,
					},
				},
				isLeader: true,
			},
			args: args{
				ctx: context.Background(),
				req: &aggregator.UpdateNodeIOStatusRequest{
					NodeName:           "node1",
					ObservedGeneration: 2,
					IoBw:               []*aggregator.IOBandwidth{},
				},
			},
			want:    &aggregator.Empty{},
			wantErr: true,
		},
		{
			name: "test Update node status grpc server handler, generation ok",
			fields: fields{
				direct:       false,
				NodeStatusCh: make(chan *aggregator.UpdateNodeIOStatusRequest, 2),
				nodeInfo4Sched: map[string]*v1.NodeIOStatusStatus{
					"node1": {},
				},
				queue: workqueue.NewNamedRateLimitingQueue(rateLimiter, "aggregator"),
				nodeInfo4Agent: map[string]*NodeInfo4Agent{
					"node1": {
						latestGeneration: 2,
						reservedPod: map[string]v1.PodRequest{
							"pod1": {
								Name:      "pod1",
								Namespace: "default",
							},
						},
						client: client,
					},
				},
				isLeader: true,
			},
			args: args{
				ctx: context.Background(),
				req: &aggregator.UpdateNodeIOStatusRequest{
					NodeName:           "node1",
					ObservedGeneration: 2,
					IoBw:               []*aggregator.IOBandwidth{},
				},
			},
			want:    &aggregator.Empty{},
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
				isLeader:                              tt.fields.isLeader,
				// Mutex:                                 tt.fields.Mutex,
			}
			patches := gomonkey.ApplyMethod(reflect.TypeOf(aggr), "GetPodList", func(_ *IoisolationAggregator, gen int64, pods map[string]v1.PodRequest, added map[string]v1.PodRequest) (*aggregator.PodList, error) {
				return &aggregator.PodList{}, nil
			})
			defer patches.Reset()
			got, err := aggr.UpdateNodeIOStatus(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("IoisolationAggregator.UpdateNodeIOStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IoisolationAggregator.UpdateNodeIOStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIoisolationAggregator_GetReservedPods(t *testing.T) {
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
		ctx context.Context
		req *aggregator.GetReservedPodsRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *aggregator.GetReservedPodsResponse
		wantErr bool
	}{
		{
			name: "test get reserved pod success",
			fields: fields{
				nodeInfo4Agent: map[string]*NodeInfo4Agent{
					"node1": {
						latestGeneration: 2,
						reservedPod: map[string]v1.PodRequest{
							"pod1": {},
						},
					},
				},
				isLeader: true,
			},
			args: args{
				ctx: context.Background(),
				req: &aggregator.GetReservedPodsRequest{
					NodeName: "node1",
				},
			},
			want: &aggregator.GetReservedPodsResponse{
				PodList: &aggregator.PodList{
					Generation: 2,
					PodList: map[string]*aggregator.PodRequest{
						"pod1": {},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "test get reserved pod fail",
			fields: fields{
				nodeInfo4Agent: map[string]*NodeInfo4Agent{
					"node1": {
						latestGeneration: 2,
						reservedPod: map[string]v1.PodRequest{
							"pod1": {},
						},
					},
				},
				isLeader: true,
			},
			args: args{
				ctx: context.Background(),
				req: &aggregator.GetReservedPodsRequest{
					NodeName: "node0",
				},
			},
			want:    &aggregator.GetReservedPodsResponse{},
			wantErr: true,
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
				isLeader:                              tt.fields.isLeader,
				// Mutex:                                 tt.fields.Mutex,
			}
			patches := gomonkey.ApplyMethod(reflect.TypeOf(aggr), "GetPodList", func(_ *IoisolationAggregator, gen int64, pods, added map[string]v1.PodRequest) (*aggregator.PodList, error) {
				return &aggregator.PodList{
					Generation: 2,
					PodList: map[string]*aggregator.PodRequest{
						"pod1": {},
					},
				}, nil
			})
			defer patches.Reset()
			got, err := aggr.GetReservedPods(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("IoisolationAggregator.GetReservedPods() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want.String()) {
				t.Errorf("IoisolationAggregator.GetReservedPods() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewIoisolationAggregator(t *testing.T) {
	type args struct {
		cfg *Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "new aggregator",
			args: args{
				cfg: &Config{
					IsMtls:         false,
					Direct:         true,
					UpdateInterval: 5 * time.Second,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			patches := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
				return &rest.Config{}, nil
			})

			defer patches.Reset()
			_, err := NewIoisolationAggregator(tt.args.cfg, &rest.Config{}, fakekubeclientset.NewSimpleClientset(), true)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewIoisolationAggregator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestIoisolationAggregator_SendReservedPodtoQueue(t *testing.T) {
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
		ctx context.Context
		c   chan os.Signal
	}
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Second)
	tests := []struct {
		name         string
		fields       fields
		args         args
		nodeToUpdate *v1.NodeIOStatusSpec
		want         int
	}{
		{
			name: "prepare pod list, add pod",
			fields: fields{
				ReservedPodCh: make(chan *v1.NodeIOStatusSpec, 3),
				queue:         workqueue.NewNamedRateLimitingQueue(rateLimiter, "aggregator"),
				nodeInfo4Agent: map[string]*NodeInfo4Agent{
					"node1": {
						latestGeneration: 3,
						reservedPod: map[string]v1.PodRequest{
							"pod1": {
								Name:      "pod1",
								Namespace: "default",
							},
						},
					},
				},
				isLeader: true,
			},
			args: args{
				ctx: context.TODO(),
				c:   make(chan os.Signal, 2),
			},
			nodeToUpdate: &v1.NodeIOStatusSpec{
				NodeName:   "node1",
				Generation: 4,
				ReservedPods: map[string]v1.PodRequest{
					"pod1": {
						Name:      "pod1",
						Namespace: "default",
					},
					"pod2": {
						Name:      "pod2",
						Namespace: "default",
					},
				},
			},
			want: 2,
		},
		{
			name: "prepare pod list, delete pod",
			fields: fields{
				ReservedPodCh: make(chan *v1.NodeIOStatusSpec, 3),
				queue:         workqueue.NewNamedRateLimitingQueue(rateLimiter, "aggregator"),
				nodeInfo4Agent: map[string]*NodeInfo4Agent{
					"node1": {
						latestGeneration: 3,
						reservedPod: map[string]v1.PodRequest{
							"pod1": {
								Name:      "pod1",
								Namespace: "default",
							},
							"pod2": {
								Name:      "pod2",
								Namespace: "default",
							},
						},
					},
				},
				isLeader: true,
			},
			args: args{
				ctx: context.TODO(),
				c:   make(chan os.Signal, 2),
			},
			nodeToUpdate: &v1.NodeIOStatusSpec{
				NodeName:   "node1",
				Generation: 4,
				ReservedPods: map[string]v1.PodRequest{
					"pod1": {
						Name:      "pod1",
						Namespace: "default",
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
				isLeader:                              tt.fields.isLeader,
				// Mutex:                                 tt.fields.Mutex,
			}
			patches1 := gomonkey.ApplyMethod(reflect.TypeOf(aggr), "GetPodList", func(_ *IoisolationAggregator, gen int64, pods, added map[string]v1.PodRequest) (*aggregator.PodList, error) {
				return &aggregator.PodList{
					Generation: 4,
					PodList: map[string]*aggregator.PodRequest{
						"pod1": nil,
						"pod2": {
							PodName:      "pod2",
							PodNamespace: "default",
							Request: []*aggregator.IOResourceRequest{
								{
									IoType: aggregator.IOType_DiskIO,
								},
							},
						},
					},
				}, nil
			})
			defer patches1.Reset()
			patches2 := gomonkey.ApplyPrivateMethod(reflect.TypeOf(aggr), "updatePodList", func(_ *IoisolationAggregator, ctx context.Context, obj interface{}) error {
				return nil
			})
			defer patches2.Reset()
			signal.Notify(tt.args.c, os.Interrupt)
			go aggr.StartWorkQueue(tt.args.ctx)
			go aggr.SendReservedPodtoQueue(tt.args.ctx, tt.args.c)
			time.Sleep(time.Second)
			aggr.ReservedPodCh <- tt.nodeToUpdate
			time.Sleep(time.Second)
			tt.args.c <- os.Interrupt
			aggr.queue.ShutDown()
			got := aggr.nodeInfo4Agent["node1"].reservedPod
			if len(got) != tt.want {
				t.Errorf("SendReservedPodtoQueue() got reservedPod = %v, want %v", got, tt.want)
				return
			}
		})
	}
}

func TestIoisolationAggregator_SendNodeStatustoQueue(t *testing.T) {
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
		c   chan os.Signal
	}
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Second)
	tests := []struct {
		name   string
		fields fields
		args   args
		update *aggregator.UpdateNodeIOStatusRequest
	}{
		{
			name: "test Send NodeStatus to Queue",
			fields: fields{
				NodeStatusCh: make(chan *aggregator.UpdateNodeIOStatusRequest, 3),
				queue:        workqueue.NewNamedRateLimitingQueue(rateLimiter, "aggregator"),
				timer:        time.NewTicker(2 * time.Second),
			},
			args: args{
				ctx: context.TODO(),
				c:   make(chan os.Signal, 2),
			},
			update: &aggregator.UpdateNodeIOStatusRequest{
				NodeName:           "node1",
				ObservedGeneration: 3,
			},
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
			patches1 := gomonkey.ApplyPrivateMethod(reflect.TypeOf(aggr), "updateNodeIOStatusCache", func(_ *IoisolationAggregator, req *aggregator.UpdateNodeIOStatusRequest) {
			})
			defer patches1.Reset()

			patches2 := gomonkey.ApplyPrivateMethod(reflect.TypeOf(aggr), "updateNodeIOStatus", func(_ *IoisolationAggregator, ctx context.Context, obj interface{}) error {
				return fmt.Errorf("")
			})
			defer patches2.Reset()
			signal.Notify(tt.args.c, os.Interrupt)
			go aggr.StartWorkQueue(tt.args.ctx)
			go aggr.SendNodeStatustoQueue(tt.args.ctx, tt.args.c)
			time.Sleep(time.Second)
			aggr.NodeStatusCh <- tt.update
			time.Sleep(2 * time.Second)
			tt.args.c <- os.Interrupt
			aggr.queue.ShutDown()
		})
	}
}
