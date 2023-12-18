/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package scheduler

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	rt "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
	"sigs.k8s.io/IOIsolation/api/config"
	ioiv1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/networkio"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	common "sigs.k8s.io/IOIsolation/pkg/scheduler/util"
	tu "sigs.k8s.io/IOIsolation/test/utils"
)

func TestGetResources(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    []common.ResourceType
		wantErr string
	}{
		{
			name:    "resource type is not set",
			args:    "",
			want:    []common.ResourceType{},
			wantErr: "resource type is unset",
		},
		{
			name:    "unknown resource type",
			args:    "XYZ",
			want:    []common.ResourceType{},
			wantErr: "unknown resource type",
		},
		{
			name:    "resource type is BlockIO",
			args:    "BlockIO",
			want:    []common.ResourceType{common.BlockIO},
			wantErr: "",
		},
		{
			name:    "resource type is NetworkIO",
			args:    "NetworkIO",
			want:    []common.ResourceType{common.NetworkIO},
			wantErr: "",
		},
		{
			name:    "resource type is RDT",
			args:    "RDT",
			want:    []common.ResourceType{common.RDT},
			wantErr: "",
		},
		{
			name:    "resource type is All",
			args:    "All",
			want:    []common.ResourceType{common.BlockIO, common.NetworkIO, common.RDT},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getResources(tt.args)
			if err != nil {
				tu.ExpectError(t, err, tt.wantErr)
			} else {
				if !reflect.DeepEqual(got, tt.want) {
					t.Fatalf("getResource got data %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	type fields struct {
		args runtime.Object // plugin args
		opts []rt.Option    // framework handle options
	}
	client := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	opts := []rt.Option{
		rt.WithClientSet(client),
		rt.WithInformerFactory(informerFactory),
		rt.WithKubeConfig(&rest.Config{}),
	}

	tests := []struct {
		name    string
		fields  *fields
		wantErr string
	}{
		{
			name: "nil plugin args",
			fields: &fields{
				args: nil,
				opts: opts,
			},
			wantErr: "want args to be of type *ResourceIOArgs",
		},
		{
			name: "Wrong plugin args type",
			fields: &fields{
				args: &ioiv1.NodeIOStatus{},
				opts: []rt.Option{},
			},
			wantErr: "want args to be of type *ResourceIOArgs",
		},
		{
			name: "Unspecified score strategy",
			fields: &fields{
				args: &config.ResourceIOArgs{
					ResourceType: "BlockIO",
				},
				opts: opts,
			},
			wantErr: "unknown score strategy",
		},
		{
			name: "Unknown score strategy",
			fields: &fields{
				args: &config.ResourceIOArgs{
					ResourceType:  "BlockIO",
					ScoreStrategy: "ABC",
				},
				opts: opts,
			},
			wantErr: "unknown score strategy",
		},
		{
			name: "success",
			fields: &fields{
				args: &config.ResourceIOArgs{
					ResourceType:  "NetworkIO",
					ScoreStrategy: "MostAllocated",
				},
				opts: opts,
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		eh := &resource.IOEventHandler{}
		patches := gomonkey.ApplyMethod(reflect.TypeOf(eh), "BuildEvtHandler", func(_ *resource.IOEventHandler, cxt context.Context) error {
			return nil
		})
		defer patches.Reset()
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			fh, err := rt.NewFramework(ctx, nil, nil, tt.fields.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := New(context.Background(), tt.fields.args, fh); err != nil {
				tu.ExpectError(t, err, tt.wantErr)
			}
		})
	}
}

func TestClearStateData(t *testing.T) {
	type fields struct {
		state  *framework.CycleState
		client kubernetes.Interface
		rhName string
	}
	rh := []string{"BlockIOHandle", "NetworkIOHandle"}
	handle2Delete := "BlockIOHandle"

	// create nodes
	c := fake.NewSimpleClientset()
	nodes, err := tu.MakeNodes(c, "N", 3)
	if err != nil {
		t.Fatal("failed to create node: ", err)
	}

	// add data in cycle state
	cs := &framework.CycleState{}
	for _, h := range rh {
		for _, n := range nodes {
			cs.Write(framework.StateKey(stateKeyPrefix+h+n.Name), &stateData{})
		}
	}

	tests := []struct {
		name   string
		fields *fields
	}{
		{
			name: fmt.Sprintf("Remove %v's state data for all nodes", handle2Delete),
			fields: &fields{
				state:  cs,
				client: c,
				rhName: handle2Delete,
			},
		},
		{
			name: "Remove state data for all nodes with empty handle name",
			fields: &fields{
				state:  cs,
				client: c,
				rhName: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearStateData(tt.fields.state, tt.fields.client, tt.fields.rhName)
			for _, n := range nodes {
				got, err := getStateData(tt.fields.state, stateKeyPrefix+handle2Delete+n.Name)
				if got != nil {
					t.Fatalf("failed to clear all nodes‘ state data for handle %v: %v, err: %v", handle2Delete, got, err)
				}
			}
		})
	}
}

func TestGetStateData(t *testing.T) {
	type fields struct {
		state *framework.CycleState
		key   string
	}
	rh := []string{"BlockIOHandle", "NetworkIOHandle"}
	handle2Get := "BlockIOHandle"

	// create nodes
	c := fake.NewSimpleClientset()
	nodes, err := tu.MakeNodes(c, "N", 3)
	if err != nil {
		t.Fatal("failed to create node: ", err)
	}

	// add data in cycle state
	cs := &framework.CycleState{}
	for _, h := range rh {
		for _, n := range nodes {
			cs.Write(framework.StateKey(stateKeyPrefix+h+n.Name), &stateData{})
		}
	}

	tests := []struct {
		name    string
		fields  *fields
		wantErr string
	}{
		{
			name: "empty key",
			fields: &fields{
				state: cs,
				key:   "",
			},
			wantErr: "not found",
		},
		{
			name: "wrong key",
			fields: &fields{
				state: cs,
				key:   "XYZ",
			},
			wantErr: "not found",
		},
		{
			name: "success",
			fields: &fields{
				state: cs,
				key:   stateKeyPrefix + handle2Get + nodes[0].Name,
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := getStateData(tt.fields.state, tt.fields.key); err != nil {
				tu.ExpectError(t, err, tt.wantErr)
			}
		})
	}
}

func TestDeleteStateData(t *testing.T) {
	type fields struct {
		state *framework.CycleState
		key   string
	}
	rh := []string{"BlockIOHandle", "NetworkIOHandle"}
	handle2Get := "BlockIOHandle"

	// create nodes
	c := fake.NewSimpleClientset()
	nodes, err := tu.MakeNodes(c, "N", 3)
	if err != nil {
		t.Fatal("failed to create node: ", err)
	}

	// add data in cycle state
	cs := &framework.CycleState{}
	for _, h := range rh {
		for _, n := range nodes {
			cs.Write(framework.StateKey(stateKeyPrefix+h+n.Name), &stateData{})
		}
	}

	tests := []struct {
		name   string
		fields *fields
	}{
		{
			name: "not existing key",
			fields: &fields{
				state: cs,
				key:   "ABC",
			},
		},
		{
			name: "success",
			fields: &fields{
				state: cs,
				key:   stateKeyPrefix + handle2Get + nodes[0].Name,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteStateData(tt.fields.state, tt.fields.key)
			if got, err := getStateData(cs, tt.fields.key); got != nil {
				t.Fatalf("data %v has not been removed: %v ", got, err)
			}
		})
	}
}

func fakeNetworeResourceConfigSpec(devs map[string]ioiv1.Device) *ioiv1.ResourceConfigSpec {
	return &ioiv1.ResourceConfigSpec{
		GAPool:  80,
		BEPool:  20,
		Devices: devs,
	}
}

func TestFilter(t *testing.T) {
	type fields struct {
		state *framework.CycleState
		pod   *v1.Pod
		ni    *framework.NodeInfo
	}

	// init resource handle
	rhs := make(map[string]resource.Handle)
	rhs["NetworkIO"] = networkio.New()
	c := fake.NewSimpleClientset()
	err := rhs["NetworkIO"].Run(resource.NewExtendedCache(), nil, c)
	if err != nil {
		t.Fatal(err)
	}
	s := map[string]ioiv1.Device{
		"50:7c:6f:06:d7:8d": {
			Name:          "ens160",
			Type:          ioiv1.DefaultDevice,
			CapacityIn:    40,
			CapacityOut:   40,
			CapacityTotal: 80,
			DefaultIn:     5,
			DefaultOut:    5,
		},
	}

	if err := rhs["NetworkIO"].(resource.CacheHandle).AddCacheNodeInfo("IOINode", *fakeNetworeResourceConfigSpec(s)); err != nil {
		t.Fatal("failed to add node cache: ", err)
	}
	if err := rhs["NetworkIO"].(resource.CacheHandle).AddCacheNodeInfo("IOINodeNoDev", *fakeNetworeResourceConfigSpec(nil)); err != nil {
		t.Fatal("failed to add node cache: ", err)
	}

	// node with ioi label and registered in cache
	nodeIOI := st.MakeNode().Name("IOINode").Label(agent.IoiLabel, "net").Obj()
	niIOI := framework.NewNodeInfo()
	niIOI.SetNode(nodeIOI)

	// node with ioi label but not registered in cache
	nodeIOIUnregistered := st.MakeNode().Name("IOINodeUnreg").Label(agent.IoiLabel, "net").Obj()
	niIOIN := framework.NewNodeInfo()
	niIOIN.SetNode(nodeIOIUnregistered)

	// node without ioi label
	nodeNoIOI := st.MakeNode().Name("NonIOINode").Obj()
	niNIOI := framework.NewNodeInfo()
	niNIOI.SetNode(nodeNoIOI)

	// node with ioi label and registered in cache but without device info
	nodeIOINoDev := st.MakeNode().Name("IOINodeNoDev").Label(agent.IoiLabel, "net").Obj()
	niNDev := framework.NewNodeInfo()
	niNDev.SetNode(nodeIOINoDev)

	// add data in cycle state
	// for _, h := range rh {
	// 	for _, n := range nodes {
	// 		cs.Write(framework.StateKey(stateKeyPrefix+h+n.Name), &stateData{})
	// 	}
	// }
	resource.IoiContext = &resource.ResourceIOContext{
		NsWhiteList: common.GetNamespaceWhiteList(fake.NewSimpleClientset()),
		Reservedpod: make(map[string]*pb.PodList),
		ClaimDevMap: make(map[string]*resource.StorageInfo),
	}

	tests := []struct {
		name   string
		fields *fields
		want   *framework.Status
		wantCS *stateData
	}{
		{
			name: "Pod namespace in whitelist",
			fields: &fields{
				state: nil,
				pod:   st.MakePod().Name("pod1").Namespace(utils.KubesystemNamespace).Obj(),
				ni:    &framework.NodeInfo{},
			},
			wantCS: nil,
			want:   framework.NewStatus(framework.Success),
		},
		{
			name: "Node status has not been reported",
			fields: &fields{
				state: &framework.CycleState{},
				pod:   st.MakePod().Name("pod2").Namespace("default").Obj(),
				ni:    niIOIN,
			},
			wantCS: nil,
			want:   framework.NewStatus(framework.Success),
		},
		{
			name: "Node without IOI cannot schedule critical pods",
			fields: &fields{
				state: nil,
				pod: st.MakePod().Name("pod3").Namespace("default").Annotations(map[string]string{
					utils.NetworkIOConfigAnno: "{\"requests\":{\"ingress\":\"50M\",\"egress\":\"50M/4k\"}}",
				}).Obj(),
				ni: niNIOI,
			},
			want:   framework.NewStatus(framework.Unschedulable, fmt.Sprintf("node %v without ioisolation support cannot schedule GA/BT workload", niNIOI.Node().Name)),
			wantCS: nil,
		},
		{
			name: "AdmitPod Error",
			fields: &fields{
				state: &framework.CycleState{},
				pod: st.MakePod().Name("pod5").Namespace("default").Annotations(map[string]string{
					utils.NetworkIOConfigAnno: "{\"requests\":{\"ingress\":\"50M\",\"egress\":\"50M/4k\"}}",
				}).Obj(),
				ni: niNDev,
			},
			want:   framework.NewStatus(framework.Unschedulable, "network io resource does not contain device info: "),
			wantCS: nil,
		},
		{
			name: "CanAdmitPod Error",
			fields: &fields{
				state: &framework.CycleState{},
				pod: st.MakePod().Name("pod6").Namespace("default").Annotations(map[string]string{
					utils.NetworkIOConfigAnno: "{\"requests\":{\"ingress\":\"50M\",\"egress\":\"50M\"}}",
				}).Obj(),
				ni: niIOI,
			},
			want:   framework.NewStatus(framework.Unschedulable, fmt.Sprintf("insufficient NetworkIOHandle total IO resource on node %v", niIOI.Node().Name)),
			wantCS: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := &ResourceIO{
				rhs:    rhs,
				scorer: nil,
				client: c,
			}
			if got := rs.Filter(context.TODO(), tt.fields.state, tt.fields.pod, tt.fields.ni); !got.Equal(tt.want) {
				t.Fatalf("filter got data %v, want %v", got, tt.want)
				if tt.wantCS != nil {
					if got, err := getStateData(tt.fields.state, stateKeyPrefix+rhs["NetworkIO"].Name()+tt.fields.ni.Node().Name); err != nil || !reflect.DeepEqual(tt.wantCS, got) {
						t.Fatalf("stateData after Filter got data %v, want %v", got, tt.want)
					}
				}
			}
		})
	}

}

func TestResourceIO_Score(t *testing.T) {
	type fields struct {
		rhs    map[string]resource.Handle
		scorer map[string]Scorer
		client kubernetes.Interface
	}
	type args struct {
		ctx      context.Context
		state    *framework.CycleState
		pod      *v1.Pod
		nodeName string
	}
	// init resource handle
	rhs := make(map[string]resource.Handle)
	rhs["NetworkIO"] = networkio.New()
	c := fake.NewSimpleClientset()
	err := rhs["NetworkIO"].Run(resource.NewExtendedCache(), nil, c)
	if err != nil {
		t.Fatal(err)
	}
	s := map[string]ioiv1.Device{
		"50:7c:6f:06:d7:8d": {
			Name:          "ens160",
			Type:          ioiv1.DefaultDevice,
			CapacityIn:    40,
			CapacityOut:   40,
			CapacityTotal: 80,
			DefaultIn:     5,
			DefaultOut:    5,
		},
	}

	if err := rhs["NetworkIO"].(resource.CacheHandle).AddCacheNodeInfo("IOINode", *fakeNetworeResourceConfigSpec(s)); err != nil {
		t.Fatal("failed to add node cache: ", err)
	}
	if err := rhs["NetworkIO"].(resource.CacheHandle).AddCacheNodeInfo("IOINodeNoDev", *fakeNetworeResourceConfigSpec(nil)); err != nil {
		t.Fatal("failed to add node cache: ", err)
	}

	resource.IoiContext = &resource.ResourceIOContext{
		NsWhiteList: common.GetNamespaceWhiteList(fake.NewSimpleClientset()),
		Reservedpod: make(map[string]*pb.PodList),
		ClaimDevMap: make(map[string]*resource.StorageInfo),
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int64
		want1  *framework.Status
	}{
		{
			name: "Pod namespace in whitelist",
			args: args{
				pod: st.MakePod().Name("pod1").Namespace(utils.KubesystemNamespace).Obj(),
			},
			want:  100,
			want1: framework.NewStatus(framework.Success),
		},
		{
			name: "Pod namespace not in whitelist",
			fields: fields{
				rhs:    rhs,
				scorer: nil,
			},
			args: args{
				pod: st.MakePod().Name("pod2").Namespace("default").Obj(),
				ctx: context.TODO(),
			},
			want:  100,
			want1: framework.NewStatus(framework.Success),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches1 := gomonkey.ApplyFunc(getStateData, func(cs *framework.CycleState, key string) (*stateData, error) {
				return &stateData{
					nodeSupportIOI: false,
				}, nil
			})
			defer patches1.Reset()
			ps := &ResourceIO{
				rhs:    tt.fields.rhs,
				scorer: tt.fields.scorer,
				client: tt.fields.client,
			}
			got, got1 := ps.Score(tt.args.ctx, tt.args.state, tt.args.pod, tt.args.nodeName)
			if got != tt.want {
				t.Errorf("ResourceIO.Score() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ResourceIO.Score() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestResourceIO_Reserve(t *testing.T) {
	type fields struct {
		rhs    map[string]resource.Handle
		scorer map[string]Scorer
		client kubernetes.Interface
	}
	type args struct {
		ctx      context.Context
		state    *framework.CycleState
		pod      *v1.Pod
		nodeName string
	}
	// init resource handle
	rhs := make(map[string]resource.Handle)
	rhs["NetworkIO"] = networkio.New()
	c := fake.NewSimpleClientset()
	err := rhs["NetworkIO"].Run(resource.NewExtendedCache(), nil, c)
	if err != nil {
		t.Fatal(err)
	}
	s := map[string]ioiv1.Device{
		"50:7c:6f:06:d7:8d": {
			Name:          "ens160",
			Type:          ioiv1.DefaultDevice,
			CapacityIn:    40,
			CapacityOut:   40,
			CapacityTotal: 80,
			DefaultIn:     5,
			DefaultOut:    5,
		},
	}

	if err := rhs["NetworkIO"].(resource.CacheHandle).AddCacheNodeInfo("IOINode", *fakeNetworeResourceConfigSpec(s)); err != nil {
		t.Fatal("failed to add node cache: ", err)
	}
	if err := rhs["NetworkIO"].(resource.CacheHandle).AddCacheNodeInfo("IOINodeNoDev", *fakeNetworeResourceConfigSpec(nil)); err != nil {
		t.Fatal("failed to add node cache: ", err)
	}
	resource.IoiContext = &resource.ResourceIOContext{
		NsWhiteList: common.GetNamespaceWhiteList(fake.NewSimpleClientset()),
		Reservedpod: make(map[string]*pb.PodList),
		ClaimDevMap: make(map[string]*resource.StorageInfo),
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *framework.Status
	}{
		{
			name: "Pod namespace not in whitelist",
			fields: fields{
				rhs:    rhs,
				client: c,
			},
			args: args{
				pod:      st.MakePod().Name("pod2").Namespace("default").Obj(),
				ctx:      context.TODO(),
				nodeName: "IOINode",
			},
			want: framework.NewStatus(framework.Success, ""),
		},
	}
	for _, tt := range tests {
		patches1 := gomonkey.ApplyFunc(getStateData, func(cs *framework.CycleState, key string) (*stateData, error) {
			return &stateData{
				nodeSupportIOI: false,
				request:        map[string]*aggregator.IOResourceRequest{},
			}, nil
		})
		defer patches1.Reset()
		patches2 := gomonkey.ApplyFunc(clearStateData, func(state *framework.CycleState, client kubernetes.Interface, rhName string) {})
		defer patches2.Reset()
		t.Run(tt.name, func(t *testing.T) {
			ps := &ResourceIO{
				rhs:    tt.fields.rhs,
				scorer: tt.fields.scorer,
				client: tt.fields.client,
			}
			if got := ps.Reserve(tt.args.ctx, tt.args.state, tt.args.pod, tt.args.nodeName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResourceIO.Reserve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceIO_Unreserve(t *testing.T) {
	type fields struct {
		rhs    map[string]resource.Handle
		scorer map[string]Scorer
		client kubernetes.Interface
	}
	type args struct {
		ctx      context.Context
		state    *framework.CycleState
		pod      *v1.Pod
		nodeName string
	}
	// init resource handle
	rhs := make(map[string]resource.Handle)
	rhs["NetworkIO"] = networkio.New()
	c := fake.NewSimpleClientset()
	err := rhs["NetworkIO"].Run(resource.NewExtendedCache(), nil, c)
	if err != nil {
		t.Fatal(err)
	}
	s := map[string]ioiv1.Device{
		"50:7c:6f:06:d7:8d": {
			Name:          "ens160",
			Type:          ioiv1.DefaultDevice,
			CapacityIn:    40,
			CapacityOut:   40,
			CapacityTotal: 80,
			DefaultIn:     5,
			DefaultOut:    5,
		},
	}

	if err := rhs["NetworkIO"].(resource.CacheHandle).AddCacheNodeInfo("IOINode", *fakeNetworeResourceConfigSpec(s)); err != nil {
		t.Fatal("failed to add node cache: ", err)
	}
	resource.IoiContext = &resource.ResourceIOContext{
		NsWhiteList: common.GetNamespaceWhiteList(fake.NewSimpleClientset()),
		Reservedpod: make(map[string]*pb.PodList),
		ClaimDevMap: make(map[string]*resource.StorageInfo),
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "unreserve pod",
			fields: fields{
				rhs:    rhs,
				client: c,
			},
			args: args{
				pod:      st.MakePod().Name("pod2").Namespace("default").Obj(),
				ctx:      context.TODO(),
				nodeName: "IOINode",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &ResourceIO{
				rhs:    tt.fields.rhs,
				scorer: tt.fields.scorer,
				client: tt.fields.client,
			}
			ps.Unreserve(tt.args.ctx, tt.args.state, tt.args.pod, tt.args.nodeName)
		})
	}
}
