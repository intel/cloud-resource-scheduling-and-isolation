/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package tool

import (
	"testing"

	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	crv1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned/fake"
	utils "sigs.k8s.io/IOIsolation/pkg"
	tu "sigs.k8s.io/IOIsolation/test/utils"
)

func Test_listDisk(t *testing.T) {
	client := fake.NewSimpleClientset()
	nodeName := "testNode"
	spec := &crv1.NodeStaticIOInfoSpec{
		NodeName: nodeName,
	}
	status := &crv1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]crv1.DeviceState{
			"diskUID1": {
				Name:  "sda",
				State: utils.Ready,
			},
			"diskUID2": {
				Name:  "sdb",
				State: utils.Ready,
			},
			"diskUID3": {
				Name:   "sdc",
				State:  utils.NotEnabled,
				Reason: "Profile fails",
			},
		},
	}
	type args struct {
		kubeClient versioned.Interface
		node       string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "list disks",
			args: args{
				kubeClient: client,
				node:       nodeName,
			},
		},
	}
	if err := tu.MakeNodeIOStaticInfoWithStatus(client, spec, status); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ListDisk(tt.args.kubeClient, tt.args.node)
		})
	}
}

func Test_addDisk(t *testing.T) {
	client := fake.NewSimpleClientset()
	nodeName := "testNode"
	spec := &crv1.NodeStaticIOInfoSpec{
		NodeName: nodeName,
	}
	status := &crv1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]crv1.DeviceState{
			"diskUID1": {
				Name:  "sda",
				State: utils.Ready,
			},
			"diskUID2": {
				Name:  "sdb",
				State: utils.NotEnabled,
			},
		},
	}
	type args struct {
		kubeClient versioned.Interface
		node       string
		devName    string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "add disk",
			args: args{
				kubeClient: client,
				node:       nodeName,
				devName:    "sdb",
			},
		},
	}
	for _, tt := range tests {
		if err := tu.MakeNodeIOStaticInfoWithStatus(client, spec, status); err != nil {
			t.Fatal(err)
		}
		t.Run(tt.name, func(t *testing.T) {
			AddDisk(tt.args.kubeClient, tt.args.node, tt.args.devName)
		})
		ListDisk(client, nodeName)
	}
}

func Test_deleteDisk(t *testing.T) {
	client := fake.NewSimpleClientset()
	cClient := fakekubeclientset.NewSimpleClientset()
	nodeName := "testNode"
	spec := &crv1.NodeStaticIOInfoSpec{
		NodeName: nodeName,
		ResourceConfig: map[string]crv1.ResourceConfigSpec{
			string(crv1.BlockIO): {
				Devices: map[string]crv1.Device{
					"diskUID1": {
						Name: "sda",
						Type: crv1.DefaultDevice,
					},
					"diskUID2": {
						Name: "sdb",
						Type: crv1.OtherDevice,
					},
				},
			},
		},
	}
	status := &crv1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]crv1.DeviceState{
			"diskUID1": {
				Name:  "sda",
				State: utils.Ready,
			},
			"diskUID2": {
				Name:  "sdb",
				State: utils.Ready,
			},
		},
	}
	type args struct {
		kubeClient versioned.Interface
		cClient    kubernetes.Interface
		node       string
		devName    string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "delete disk",
			args: args{
				kubeClient: client,
				cClient:    cClient,
				node:       nodeName,
				devName:    "sdb",
			},
		},
	}
	for _, tt := range tests {
		if err := tu.MakeNodeIOStaticInfoWithStatus(client, spec, status); err != nil {
			t.Fatal(err)
		}
		t.Run(tt.name, func(t *testing.T) {
			DeleteDisk(tt.args.kubeClient, tt.args.cClient, tt.args.node, tt.args.devName)
		})
		ListDisk(client, nodeName)
	}
}
