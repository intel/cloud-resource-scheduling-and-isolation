/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package resource

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned/fake"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	cm "sigs.k8s.io/IOIsolation/pkg/scheduler/util"
	tu "sigs.k8s.io/IOIsolation/test/utils"
)

func TestResourceIOContext_AddPod(t *testing.T) {
	type fields struct {
		Client         kubernetes.Interface
		VClient        versioned.Interface
		Reservedpod    map[string]*pb.PodList
		NsWhiteList    []string
		ClaimDevMap    map[string]*StorageInfo
		queue          workqueue.RateLimitingInterface
		lastUpdatedGen map[string]int64
		// Mutex          sync.Mutex
	}
	type args struct {
		ctx      context.Context
		reqlist  *pb.PodRequest
		pod      *corev1.Pod
		nodeName string
	}
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Second)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "add pod",
			fields: fields{
				Reservedpod: map[string]*pb.PodList{
					"node1": {
						PodList: make(map[string]*pb.PodRequest),
					},
				},
				lastUpdatedGen: map[string]int64{
					"node1": -1,
				},
				queue: workqueue.NewNamedRateLimitingQueue(rateLimiter, "xx"),
			},
			args: args{
				ctx: context.TODO(),
				reqlist: &pb.PodRequest{
					PodName:      "pod1",
					PodNamespace: "default",
					Request: []*pb.IOResourceRequest{
						{
							IoType:       pb.IOType_DiskIO,
							WorkloadType: utils.GaIndex,
							DevRequest: &pb.DeviceRequirement{
								DevName: "dev1",
								Request: &pb.Quantity{
									Inbps:  50,
									Outbps: 50,
								},
							},
						},
					},
				},
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID: "pod1",
					},
				},
				nodeName: "node1",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ResourceIOContext{
				Client:         tt.fields.Client,
				VClient:        tt.fields.VClient,
				Reservedpod:    tt.fields.Reservedpod,
				NsWhiteList:    tt.fields.NsWhiteList,
				ClaimDevMap:    tt.fields.ClaimDevMap,
				queue:          tt.fields.queue,
				lastUpdatedGen: tt.fields.lastUpdatedGen,
				// Mutex:          tt.fields.Mutex,
			}
			if err := c.AddPod(tt.args.ctx, tt.args.reqlist, tt.args.pod, tt.args.nodeName); (err != nil) != tt.wantErr {
				t.Errorf("ResourceIOContext.AddPod() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResourceIOContext_RemovePod(t *testing.T) {
	type fields struct {
		Client         kubernetes.Interface
		VClient        versioned.Interface
		Reservedpod    map[string]*pb.PodList
		NsWhiteList    []string
		ClaimDevMap    map[string]*StorageInfo
		queue          workqueue.RateLimitingInterface
		lastUpdatedGen map[string]int64
		// Mutex          sync.Mutex
	}
	type args struct {
		ctx      context.Context
		pod      *corev1.Pod
		nodeName string
	}
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Second)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "delete pod",
			fields: fields{
				Reservedpod: map[string]*pb.PodList{
					"node1": {
						PodList: map[string]*pb.PodRequest{
							"pod1": {
								PodName: "pod1-name",
							},
						},
					},
				},
				lastUpdatedGen: map[string]int64{
					"node1": -1,
				},
				queue: workqueue.NewNamedRateLimitingQueue(rateLimiter, "xx"),
			},
			args: args{
				ctx: context.TODO(),
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID: "pod1",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
					},
				},
				nodeName: "node1",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ResourceIOContext{
				Client:         tt.fields.Client,
				VClient:        tt.fields.VClient,
				Reservedpod:    tt.fields.Reservedpod,
				NsWhiteList:    tt.fields.NsWhiteList,
				ClaimDevMap:    tt.fields.ClaimDevMap,
				queue:          tt.fields.queue,
				lastUpdatedGen: tt.fields.lastUpdatedGen,
				// Mutex:          tt.fields.Mutex,
			}
			if err := c.RemovePod(tt.args.ctx, tt.args.pod, tt.args.nodeName); (err != nil) != tt.wantErr {
				t.Errorf("ResourceIOContext.RemovePod() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResourceIOContext_InNamespaceWhiteList(t *testing.T) {
	type fields struct {
		NsWhiteList []string
	}
	type args struct {
		ns string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Found",
			fields: fields{
				NsWhiteList: []string{"default"},
			},
			args: args{
				ns: "default",
			},
			want: true,
		},
		{
			name: "Not found",
			fields: fields{
				NsWhiteList: []string{"default"},
			},
			args: args{
				ns: "ioi-system",
			},
			want: false,
		},
		{
			name: "Not found with empty white list",
			fields: fields{
				NsWhiteList: []string{},
			},
			args: args{
				ns: "ioi-system",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ResourceIOContext{
				NsWhiteList: tt.fields.NsWhiteList,
			}
			if got := c.InNamespaceWhiteList(tt.args.ns); got != tt.want {
				t.Errorf("ResourceIOContext.InNamespaceWhiteList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceIOContext_RemoveNode(t *testing.T) {
	type fields struct {
		Reservedpod map[string]*pb.PodList
	}
	type args struct {
		node string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Remove an existing node",
			fields: fields{
				Reservedpod: map[string]*pb.PodList{
					"nodeA": nil,
				},
			},
			args: args{
				node: "nodeA",
			},
		},
		{
			name: "Remove a non-existing node",
			fields: fields{
				Reservedpod: map[string]*pb.PodList{
					"nodeA": nil,
				},
			},
			args: args{
				node: "nodeB",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ResourceIOContext{
				Reservedpod: tt.fields.Reservedpod,
			}
			c.RemoveNode(tt.args.node)
			if _, ok := c.Reservedpod[tt.args.node]; ok {
				t.Errorf("ResourceIOContext.RemoveNode() failed, node %s still exists", tt.args.node)
			}
		})
	}
}

func TestResourceIOContext_SetPodListGen(t *testing.T) {
	type fields struct {
		Reservedpod map[string]*pb.PodList
	}
	type args struct {
		node string
		gen  int64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Success with nil ",
			fields: fields{
				Reservedpod: map[string]*pb.PodList{
					"nodeA": nil,
				},
			},
			args: args{
				node: "nodeA",
				gen:  0,
			},
			wantErr: false,
		},
		{
			name: "Success",
			fields: fields{
				Reservedpod: map[string]*pb.PodList{
					"nodeA": {},
				},
			},
			args: args{
				node: "nodeA",
				gen:  0,
			},
			wantErr: false,
		},
		{
			name: "Non existing node",
			fields: fields{
				Reservedpod: map[string]*pb.PodList{
					"nodeA": {},
				},
			},
			args: args{
				node: "nodeB",
				gen:  0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ResourceIOContext{
				Reservedpod: tt.fields.Reservedpod,
			}
			if err := c.SetPodListGen(tt.args.node, tt.args.gen); (err != nil) != tt.wantErr {
				t.Errorf("ResourceIOContext.SetPodListGen() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewContext(t *testing.T) {
	ctx := context.Background()
	fw, err := runtime.NewFramework(ctx,
		nil,
		nil,
		runtime.WithKubeConfig(&rest.Config{}),
		runtime.WithClientSet(kubernetes.NewForConfigOrDie(&rest.Config{})),
	)
	if err != nil {
		t.Fatalf("failed to create framework: %v", err)
	}
	type args struct {
		rl workqueue.RateLimiter
		wl []string
		h  framework.Handle
	}
	tests := []struct {
		name    string
		args    args
		want    *ResourceIOContext
		wantErr bool
	}{
		{
			name: "rate limiter is nil",
			args: args{
				rl: nil,
				wl: []string{"default"},
				h:  nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				rl: workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Second),
				wl: []string{"default"},
				h:  fw,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewContext(tt.args.rl, tt.args.wl, tt.args.h)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestResourceIOContext_updateContext(t *testing.T) {
	c := fakekubeclientset.NewSimpleClientset()
	tPod := "pod1"
	tNode := "nodeA"
	tPodObj, err := tu.MakePod(c, st.MakePod().Name(tPod).Namespace("default").Obj())
	if err != nil {
		t.Fatal(err)
	}
	vc := fake.NewSimpleClientset()
	if err = cm.CreateNodeIOStatus(vc, &v1.NodeIOStatusSpec{
		NodeName:   tNode,
		Generation: 0,
	}); err != nil {
		t.Fatal(err)
	}
	type fields struct {
		Client         kubernetes.Interface
		VClient        versioned.Interface
		Reservedpod    map[string]*pb.PodList
		NsWhiteList    []string
		ClaimDevMap    map[string]*StorageInfo
		queue          workqueue.RateLimitingInterface
		lastUpdatedGen map[string]int64
	}
	type args struct {
		spec *v1.NodeIOStatusSpec
		p    *corev1.Pod
		req  *pb.PodRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "nil nodename in spec",
			fields: fields{
				Client:  c,
				VClient: vc,
			},
			args: args{
				spec: &v1.NodeIOStatusSpec{},
			},
			wantErr: false,
		},
		{
			name: "empty nodename in spec",
			fields: fields{
				Client:  c,
				VClient: vc,
			},
			args: args{
				spec: &v1.NodeIOStatusSpec{
					NodeName: "",
				},
			},
			wantErr: false,
		},
		{
			name: "old generation",
			fields: fields{
				Client:  c,
				VClient: vc,
				lastUpdatedGen: map[string]int64{
					"nodeA": 1,
				},
			},
			args: args{
				spec: &v1.NodeIOStatusSpec{
					NodeName:   "nodeA",
					Generation: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "pod info is unset",
			fields: fields{
				Client:  c,
				VClient: vc,
				lastUpdatedGen: map[string]int64{
					"nodeA": 1,
				},
			},
			args: args{
				spec: &v1.NodeIOStatusSpec{
					NodeName:   "nodeA",
					Generation: 2,
				},
				req: &pb.PodRequest{
					PodName:      "podA",
					PodNamespace: "default",
					Request: []*pb.IOResourceRequest{
						{
							IoType:       pb.IOType_DiskIO,
							WorkloadType: utils.GaIndex,
							DevRequest: &pb.DeviceRequirement{
								DevName: "dev1",
								Limit: &pb.Quantity{
									Inbps:  100,
									Outbps: 100,
								},
								RawLimit: &pb.Quantity{
									Inbps:   50,
									Outbps:  50,
									Iniops:  5000,
									Outiops: 5000,
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "add pod annotation",
			fields: fields{
				Client:  c,
				VClient: vc,
				lastUpdatedGen: map[string]int64{
					"nodeA": 1,
				},
			},
			args: args{
				spec: &v1.NodeIOStatusSpec{
					NodeName:   "nodeA",
					Generation: 2,
				},
				p: tPodObj,
				req: &pb.PodRequest{
					PodName:      "podA",
					PodNamespace: "default",
					Request: []*pb.IOResourceRequest{
						{
							IoType:       pb.IOType_DiskIO,
							WorkloadType: utils.GaIndex,
							DevRequest: &pb.DeviceRequirement{
								DevName: "dev1",
								Limit: &pb.Quantity{
									Inbps:  100,
									Outbps: 100,
								},
								RawLimit: &pb.Quantity{
									Inbps:   50,
									Outbps:  50,
									Iniops:  5000,
									Outiops: 5000,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "delete pod annotation",
			fields: fields{
				Client:  c,
				VClient: vc,
				lastUpdatedGen: map[string]int64{
					"nodeA": 1,
				},
			},
			args: args{
				spec: &v1.NodeIOStatusSpec{
					NodeName:   "nodeA",
					Generation: 2,
				},
				p:   tPodObj,
				req: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := &ResourceIOContext{
				Client:         tt.fields.Client,
				VClient:        tt.fields.VClient,
				Reservedpod:    tt.fields.Reservedpod,
				NsWhiteList:    tt.fields.NsWhiteList,
				ClaimDevMap:    tt.fields.ClaimDevMap,
				queue:          tt.fields.queue,
				lastUpdatedGen: tt.fields.lastUpdatedGen,
			}
			if err := c.updateContext(ctx, tt.args.spec, tt.args.p, tt.args.req); (err != nil) != tt.wantErr {
				t.Errorf("ResourceIOContext.updateContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestResourceIOContext_RunWorkerQueue(t *testing.T) {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Second)
	type fields struct {
		queue workqueue.RateLimitingInterface
	}
	type args struct {
		item *syncContext
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "success",
			fields: fields{
				queue: workqueue.NewNamedRateLimitingQueue(rateLimiter, "yy"),
			},
			args: args{
				item: &syncContext{
					spec: &v1.NodeIOStatusSpec{
						NodeName:   "nodeA",
						Generation: 2,
					},
					pod: nil,
					req: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			c := &ResourceIOContext{
				queue: tt.fields.queue,
			}
			go c.RunWorkerQueue(ctx)
			c.queue.Add(tt.args.item)
			time.Sleep(2 * time.Second)
		})
	}

}
