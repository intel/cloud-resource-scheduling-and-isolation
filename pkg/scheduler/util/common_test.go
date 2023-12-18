/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
	crv1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned/fake"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	tu "sigs.k8s.io/IOIsolation/test/utils"
)

func TestIsUnderPressure(t *testing.T) {
	tests := []struct {
		name string
		args float64
		want bool
	}{
		{
			name: "not under pressure",
			args: 0,
			want: false,
		},
		{
			name: "under pressure",
			args: 1,
			want: true,
		},
		{
			name: "under pressure 1",
			args: 100,
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnderPressure(tt.args); got != tt.want {
				t.Errorf("IsUnderPressure got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemovePVCsFromVolDiskCm(t *testing.T) {
	nodeName := "testNode"
	pvcName := "test"
	pvcNS := "default"
	type fields struct {
		nodeName string
		pvc      *v1.PersistentVolumeClaim
		client   kubernetes.Interface
	}

	fakeKubeClient := fakekubeclientset.NewSimpleClientset()

	m := map[string]string{
		fmt.Sprintf("%s.%s", pvcName, pvcNS): "",
		"test1.default":                      "",
	}
	if err := tu.MakeVolumeDeviceMapConfigMap(fakeKubeClient, context.TODO(), nodeName, m); err != nil {
		t.Fatalf("failed to create fake volume device mapping configmap")
	}

	tests := []struct {
		name    string
		args    *fields
		want    map[string]string
		wantErr string
	}{
		{
			name: "ConfigMap for node fakeNode does not exist",
			args: &fields{
				nodeName: "fakeNode",
				pvc:      nil,
				client:   fakeKubeClient,
			},
			want:    nil,
			wantErr: "configMap for volume device binding does not exists",
		},
		{
			name: "Failed to reach API server",
			args: &fields{
				nodeName: "fakeNode",
				pvc:      nil,
				client:   nil,
			},
			want:    nil,
			wantErr: "failed to get ConfigMap volume-device-map",
		},
		{
			name: "success",
			args: &fields{
				nodeName: nodeName,
				pvc: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: pvcNS,
					},
				},
				client: fakeKubeClient,
			},
			want: map[string]string{
				"test1.default": "",
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemovePVCsFromVolDiskCm(tt.args.client, tt.args.pvc, tt.args.nodeName); err != nil {
				tu.ExpectError(t, err, tt.wantErr)
				if tt.want != nil {
					cm, err := GetVolumeDeviceMap(fakeKubeClient, nodeName)
					if err != nil {
						t.Fatalf(err.Error())
					}
					if !reflect.DeepEqual(cm.Data, tt.want) {
						t.Fatalf("RemovePVCsFromVolDiskCm got data %v, want %v", cm.Data, tt.want)
					}
				}
			}
		})
	}
}

func TestRemoveNodeFromVolDiskCm(t *testing.T) {
	nodeName := "testNode"

	type fields struct {
		nodeName string
		client   kubernetes.Interface
	}

	fakeKubeClient := fakekubeclientset.NewSimpleClientset()

	m := map[string]string{
		"test1.default": "",
		"test2.default": "",
	}
	if err := tu.MakeVolumeDeviceMapConfigMap(fakeKubeClient, context.TODO(), nodeName, m); err != nil {
		t.Fatalf("failed to create fake volume device mapping configmap")
	}

	tests := []struct {
		name    string
		args    *fields
		want    map[string]string
		wantErr string
	}{
		{
			name: "ConfigMap for node fakeNode does not exist",
			args: &fields{
				nodeName: "fakeNode",
				client:   fakeKubeClient,
			},
			want:    nil,
			wantErr: "configMap for volume device binding does not exists",
		},
		{
			name: "Failed to reach API server",
			args: &fields{
				nodeName: "fakeNode",
				client:   nil,
			},
			want:    nil,
			wantErr: "failed to get ConfigMap volume-device-map",
		},
		{
			name: "success",
			args: &fields{
				nodeName: nodeName,
				client:   fakeKubeClient,
			},
			want:    nil,
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemoveNodeFromVolDiskCm(tt.args.client, tt.args.nodeName); err != nil {
				tu.ExpectError(t, err, tt.wantErr)
				if tt.want != nil {
					cm, err := GetVolumeDeviceMap(fakeKubeClient, nodeName)
					if err != nil {
						t.Fatalf(err.Error())
					}
					if !reflect.DeepEqual(cm.Data, tt.want) {
						t.Fatalf("RemoveNodeFromVolDiskCm got data %v, want %v", cm.Data, tt.want)
					}
				}
			}
		})
	}
}

func TestCreateNodeIOStatus(t *testing.T) {
	vc := fake.NewSimpleClientset()
	type fields struct {
		client versioned.Interface
		spec   *crv1.NodeIOStatusSpec
	}
	nodename := "test"
	spec := &crv1.NodeIOStatusSpec{
		NodeName:   nodename,
		Generation: 0,
		ReservedPods: map[string]crv1.PodRequest{
			"poduid": {
				Name:      "pod1",
				Namespace: "default",
			},
		},
	}

	tests := []struct {
		name    string
		args    *fields
		wantErr string
	}{
		{
			name: "success",
			args: &fields{
				client: vc,
				spec:   spec,
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateNodeIOStatus(tt.args.client, tt.args.spec); err != nil {
				tu.ExpectError(t, err, tt.wantErr)
			}
		})
	}
}

func TestUpdateNodeIOStatusSpec(t *testing.T) {
	vc := fake.NewSimpleClientset()
	type fields struct {
		client versioned.Interface
		n      string
		spec   *crv1.NodeIOStatusSpec
	}
	nodename := "test"
	spec := &crv1.NodeIOStatusSpec{
		NodeName:   nodename,
		Generation: 0,
		ReservedPods: map[string]crv1.PodRequest{
			"poduid": {
				Name:      "pod1",
				Namespace: "default",
			},
		},
	}
	increGen := spec.DeepCopy()
	increGen.Generation += 1

	addPod := spec.DeepCopy()
	addPod.Generation += 1
	addPod.ReservedPods["pod2uid"] = crv1.PodRequest{
		Name:      "pod2",
		Namespace: "default",
	}

	tests := []struct {
		name    string
		args    *fields
		wantErr string
	}{
		{
			name: "NodeIOStatus does not exist",
			args: &fields{
				client: vc,
				n:      "abc",
				spec:   nil,
			},
			wantErr: "not found",
		},
		{
			name: "increment generation",
			args: &fields{
				client: vc,
				n:      nodename,
				spec:   increGen,
			},
			wantErr: "",
		},
		{
			name: "add pod ",
			args: &fields{
				client: vc,
				n:      nodename,
				spec:   addPod,
			},
			wantErr: "",
		},
	}
	if err := CreateNodeIOStatus(vc, spec); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UpdateNodeIOStatusSpec(tt.args.client, tt.args.n, tt.args.spec); err != nil {
				tu.ExpectError(t, err, tt.wantErr)
			}
		})
	}
}

func TestUpdateNodeIOStatusInType(t *testing.T) {
	vc := fake.NewSimpleClientset()
	type fields struct {
		client   versioned.Interface
		n        string
		toUpdate map[crv1.WorkloadIOType]crv1.DeviceAllocatableBandwidth
	}
	nodename := "test"
	spec := &crv1.NodeIOStatusSpec{
		NodeName:   nodename,
		Generation: 0,
		ReservedPods: map[string]crv1.PodRequest{
			"poduid": {
				Name:      "pod1",
				Namespace: "default",
			},
		},
	}

	tests := []struct {
		name    string
		args    *fields
		wantErr string
	}{
		{
			name: "NodeIOStatus does not exist",
			args: &fields{
				client:   vc,
				n:        "abc",
				toUpdate: nil,
			},
			wantErr: "not found",
		},
		{
			name: "success",
			args: &fields{
				client: vc,
				n:      nodename,
				toUpdate: map[crv1.WorkloadIOType]crv1.DeviceAllocatableBandwidth{
					crv1.BlockIO: {
						DeviceIOStatus: map[string]crv1.IOAllocatableBandwidth{
							"devid": {
								IOPoolStatus: map[crv1.WorkloadQoSType]crv1.IOStatus{
									crv1.WorkloadGA: {},
									crv1.WorkloadBE: {},
								},
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "duplicate",
			args: &fields{
				client: vc,
				n:      nodename,
				toUpdate: map[crv1.WorkloadIOType]crv1.DeviceAllocatableBandwidth{
					crv1.BlockIO: {
						DeviceIOStatus: map[string]crv1.IOAllocatableBandwidth{
							"devid": {
								IOPoolStatus: map[crv1.WorkloadQoSType]crv1.IOStatus{
									crv1.WorkloadGA: {},
									crv1.WorkloadBE: {},
								},
							},
						},
					},
				},
			},
			wantErr: "",
		},
	}
	if err := CreateNodeIOStatus(vc, spec); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UpdateNodeIOStatusInType(tt.args.client, tt.args.n, tt.args.toUpdate); err != nil {
				tu.ExpectError(t, err, tt.wantErr)
			}
		})
	}
}

func TestUpdateNodeIOStatusInDevice(t *testing.T) {
	client := fake.NewSimpleClientset()
	nodename := "test"
	spec := &crv1.NodeIOStatusSpec{
		NodeName:   nodename,
		Generation: 2,
		ReservedPods: map[string]crv1.PodRequest{
			"poduid": {
				Name:      "pod1",
				Namespace: "default",
			},
		},
	}
	status := &crv1.NodeIOStatusStatus{
		ObservedGeneration: 2,
		AllocatableBandwidth: map[crv1.WorkloadIOType]crv1.DeviceAllocatableBandwidth{
			crv1.BlockIO: {
				DeviceIOStatus: map[string]crv1.IOAllocatableBandwidth{
					"device2": {
						IOPoolStatus: map[crv1.WorkloadQoSType]crv1.IOStatus{
							crv1.WorkloadGA: {},
							crv1.WorkloadBE: {},
						},
					},
				},
			},
		},
	}
	adds := map[string]crv1.IOAllocatableBandwidth{
		"device1": {
			IOPoolStatus: map[crv1.WorkloadQoSType]crv1.IOStatus{
				crv1.WorkloadGA: {},
				crv1.WorkloadBE: {},
			},
		},
	}
	dels := map[string]crv1.IOAllocatableBandwidth{
		"device2": {},
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success",
			wantErr: false,
		},
	}
	if err := tu.MakeNodeIOStatus(client, spec, status); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UpdateNodeIOStatusInDevice(client, nodename, crv1.BlockIO, adds, dels)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateNodeIOStatusInDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestComparePodList(t *testing.T) {
	type fields struct {
		pl1 map[string]crv1.PodRequest
		pl2 map[string]crv1.PodRequest
	}
	tests := []struct {
		name string
		args *fields
		want bool
	}{
		{
			name: "two nil pod lists",
			args: &fields{
				pl1: nil,
				pl2: nil,
			},
			want: true,
		},
		{
			name: "one nil pod lists",
			args: &fields{
				pl1: nil,
				pl2: map[string]crv1.PodRequest{
					"pod2uid": {},
				},
			},
			want: false,
		},
		{
			name: "equal pod lists",
			args: &fields{
				pl1: map[string]crv1.PodRequest{
					"pod2uid": {},
					"pod1uid": {},
				},
				pl2: map[string]crv1.PodRequest{
					"pod2uid": {},
					"pod1uid": {},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComparePodList(tt.args.pl1, tt.args.pl2); got != tt.want {
				t.Errorf("ComparePodList got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddOrUpdateAnnoOnPod(t *testing.T) {
	c := fakekubeclientset.NewSimpleClientset()
	podname := "pod1"
	pod, err := tu.MakePod(c, st.MakePod().Name(podname).Namespace("default").Obj())
	if err != nil {
		t.Fatal(err)
	}
	type fields struct {
		p    string
		anno map[string]string
	}
	tests := []struct {
		name    string
		args    *fields
		wantErr string
	}{
		{
			name: "nil annotation",
			args: &fields{
				p:    podname,
				anno: nil,
			},
			wantErr: "the annotation is unset",
		},
		{
			name: "empty annotation",
			args: &fields{
				p:    podname,
				anno: map[string]string{},
			},
			wantErr: "the annotation is unset",
		},
		{
			name: "add annotation",
			args: &fields{
				p: podname,
				anno: map[string]string{
					"anno1": "val1",
				},
			},
			wantErr: "",
		},
		{
			name: "update annotation",
			args: &fields{
				p: podname,
				anno: map[string]string{
					"anno1": "val123",
				},
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := AddOrUpdateAnnoOnPod(ctx, c, pod, tt.args.anno); err != nil {
				tu.ExpectError(t, err, tt.wantErr)
			}
		})
	}
}

func TestDeleteAnnoOnPod(t *testing.T) {
	c := fakekubeclientset.NewSimpleClientset()
	podname := "pod1"
	pod, err := tu.MakePod(c, st.MakePod().Name(podname).Namespace("default").Annotations(map[string]string{"anno1": "val1"}).Obj())
	if err != nil {
		t.Fatal(err)
	}
	type fields struct {
		p   *v1.Pod
		key string
	}
	tests := []struct {
		name    string
		args    *fields
		wantErr string
	}{
		{
			name: "empty annotation key",
			args: &fields{
				p:   &v1.Pod{},
				key: "",
			},
			wantErr: "annotation key is unset",
		},
		{
			name: "empty pod",
			args: &fields{
				p:   &v1.Pod{},
				key: "123",
			},
			wantErr: "pod info is unset",
		},
		{
			name: "delete annotation",
			args: &fields{
				p:   pod,
				key: "anno1",
			},
			wantErr: "",
		},
		{
			name: "delete same annotation again",
			args: &fields{
				p:   pod,
				key: "anno1",
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := DeleteAnnoOnPod(ctx, c, tt.args.p, tt.args.key); err != nil {
				tu.ExpectError(t, err, tt.wantErr)
			}
		})
	}
}

func TestPodRequest2String(t *testing.T) {
	tests := []struct {
		name    string
		args    *pb.PodRequest
		want    string
		wantErr string
	}{
		{
			name:    "empty pod request",
			args:    nil,
			wantErr: "empty request",
		},
		{
			name: "empty IO resource requests",
			args: &pb.PodRequest{
				Request: nil,
			},
			wantErr: "empty request",
		},
		{
			name: "success with request only",
			args: &pb.PodRequest{
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
							RawRequest: &pb.Quantity{
								Inbps:   50,
								Outbps:  50,
								Iniops:  5000,
								Outiops: 5000,
							},
						},
					},
				},
			},
			want:    "{\"1\":{\"dev1\":{\"qos\":0,\"request\":{\"inbps\":100,\"outbps\":100},\"limit\":{},\"rawRequest\":{\"inbps\":50,\"outbps\":50,\"iniops\":5000,\"outiops\":5000},\"rawLimit\":{}}}}",
			wantErr: "",
		},
		{
			name: "success with limit only",
			args: &pb.PodRequest{
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
			want:    "{\"1\":{\"dev1\":{\"qos\":0,\"request\":{},\"limit\":{\"inbps\":100,\"outbps\":100},\"rawRequest\":{},\"rawLimit\":{\"inbps\":50,\"outbps\":50,\"iniops\":5000,\"outiops\":5000}}}}",
			wantErr: "",
		},
		{
			name: "success with be",
			args: &pb.PodRequest{
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
			want:    "{\"1\":{\"dev1\":{\"qos\":2,\"request\":{},\"limit\":{},\"rawRequest\":{},\"rawLimit\":{}}}}",
			wantErr: "",
		},
		{
			name: "empty device requirement",
			args: &pb.PodRequest{
				Request: []*pb.IOResourceRequest{},
			},
			wantErr: "the device requirement is empty",
		},
		{
			name: "network: success with request only",
			args: &pb.PodRequest{
				Request: []*pb.IOResourceRequest{
					{
						IoType:       pb.IOType_NetworkIO,
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
			want:    "{\"2\":{\"dev1\":{\"qos\":0,\"request\":{\"inbps\":100,\"outbps\":100},\"limit\":{},\"rawRequest\":{},\"rawLimit\":{}}}}",
			wantErr: "",
		},
		{
			name: "network and disk: success with request only",
			args: &pb.PodRequest{
				Request: []*pb.IOResourceRequest{
					{
						IoType:       pb.IOType_NetworkIO,
						WorkloadType: utils.GaIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
							Request: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
						},
					},
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
			want:    "{\"1\":{\"dev1\":{\"qos\":0,\"request\":{\"inbps\":100,\"outbps\":100},\"limit\":{},\"rawRequest\":{},\"rawLimit\":{}}},\"2\":{\"dev1\":{\"qos\":0,\"request\":{\"inbps\":100,\"outbps\":100},\"limit\":{},\"rawRequest\":{},\"rawLimit\":{}}}}",
			wantErr: "",
		},
		{
			name: "network and disk: success with multiple request",
			args: &pb.PodRequest{
				Request: []*pb.IOResourceRequest{
					{
						IoType:       pb.IOType_NetworkIO,
						WorkloadType: utils.GaIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev1",
							Request: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
						},
					},
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
					{
						IoType:       pb.IOType_DiskIO,
						WorkloadType: utils.GaIndex,
						DevRequest: &pb.DeviceRequirement{
							DevName: "dev2",
							Request: &pb.Quantity{
								Inbps:  100,
								Outbps: 100,
							},
						},
					},
				},
			},
			want:    "{\"1\":{\"dev1\":{\"qos\":0,\"request\":{\"inbps\":100,\"outbps\":100},\"limit\":{},\"rawRequest\":{},\"rawLimit\":{}},\"dev2\":{\"qos\":0,\"request\":{\"inbps\":100,\"outbps\":100},\"limit\":{},\"rawRequest\":{},\"rawLimit\":{}}},\"2\":{\"dev1\":{\"qos\":0,\"request\":{\"inbps\":100,\"outbps\":100},\"limit\":{},\"rawRequest\":{},\"rawLimit\":{}}}}",
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PodRequest2String(tt.args)
			if err != nil {
				tu.ExpectError(t, err, tt.wantErr)
			} else {
				if got != tt.want {
					t.Errorf("PodRequest2String got %v, want %v", got, tt.want)
				}
			}
		})
	}

}
