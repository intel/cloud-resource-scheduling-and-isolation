/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"reflect"
	"testing"

	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/IOIsolation/pkg/agent/common"
)

func TestFilterDisk(t *testing.T) {
	diskInfo := agent.AllDiskInfo{
		"disk1": utils.BlockDevice{Name: "disk1"},
		"disk2": utils.BlockDevice{Name: "disk2"},
		"disk3": utils.BlockDevice{Name: "disk3"},
	}

	disks := []common.DiskInput{
		{DiskName: "disk1"},
		{DiskName: "disk3"},
	}

	expectedDiskCurSelect := map[string]utils.BlockDevice{
		"disk1": {Name: "disk1"},
		"disk3": {Name: "disk3"},
	}

	expectedDiskState := map[string]string{
		"disk1": utils.Profiling,
		"disk2": utils.NotEnabled,
		"disk3": utils.Profiling,
	}

	pDiskState = &map[string]string{}

	diskCurSelect := FilterDisk(diskInfo, disks)

	// Check if diskCurSelect is equal to expectedDiskCurSelect
	if !reflect.DeepEqual(diskCurSelect, expectedDiskCurSelect) {
		t.Errorf("FilterDisk() returned unexpected diskCurSelect.\nExpected: %v\nGot: %v", expectedDiskCurSelect, diskCurSelect)
	}

	// Check if pDiskState is equal to expectedDiskState
	if !reflect.DeepEqual(*pDiskState, expectedDiskState) {
		t.Errorf("FilterDisk() did not update pDiskState correctly.\nExpected: %v\nGot: %v", expectedDiskState, *pDiskState)
	}
}

func TestGetDeviceChange(t *testing.T) {
	oldNodeInfo := &v1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]v1.DeviceState{
			"disk1": {
				State: utils.Ready,
				Name:  "disk1",
			},
			"disk2": {
				State: utils.Ready,
				Name:  "disk2",
			},
		},
	}

	newNodeInfo := &v1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]v1.DeviceState{
			"disk1": {
				State: utils.Ready,
				Name:  "disk1",
			},
			"disk2": {
				State: utils.Profiling,
				Name:  "disk2",
			},
			"disk3": {
				State: utils.NotEnabled,
				Name:  "disk3",
			},
		},
	}

	expectedDevices := map[string][]string{
		"disk3": {"disk3"},
	}

	devices := GetDeviceChange(oldNodeInfo, newNodeInfo)

	// Check if devices is equal to expectedDevices
	if !reflect.DeepEqual(devices, expectedDevices) {
		t.Errorf("GetDeviceChange() returned unexpected devices.\nExpected: %v\nGot: %v", expectedDevices, devices)
	}

	GetDeviceChange(nil, nil)
}

func TestProcessDeviceStatusChange(t *testing.T) {
	oldNodeInfo := &v1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]v1.DeviceState{
			"disk1": {
				State: utils.NotEnabled,
				Name:  "disk1",
			},
			"disk2": {
				State: utils.Ready,
				Name:  "disk2",
			},
		},
	}

	newNodeInfo := &v1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]v1.DeviceState{
			"disk1": {
				State: utils.Profiling,
				Name:  "disk1",
			},
			"disk2": {
				State: utils.NotEnabled,
				Name:  "disk2",
			},
		},
	}

	ProcessDeviceStatusChange(oldNodeInfo, newNodeInfo)
	ProcessDeviceStatusChange(nil, nil)

	oldNodeInfoSame := &v1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]v1.DeviceState{
			"disk1": {
				State: utils.Profiling,
				Name:  "disk1",
			},
		},
	}

	newNodeInfoSame := &v1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]v1.DeviceState{
			"disk1": {
				State: utils.Profiling,
				Name:  "disk1",
			},
		},
	}

	ProcessDeviceStatusChange(oldNodeInfoSame, newNodeInfoSame)

	oldNodeInfoDev := &v1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]v1.DeviceState{
			"disk1": {
				State: utils.Profiling,
				Name:  "disk1",
			},
		},
	}

	newNodeInfoDev := &v1.NodeStaticIOInfoStatus{
		DeviceStates: map[string]v1.DeviceState{
			"disk12": {
				State: utils.Profiling,
				Name:  "disk12",
			},
		},
	}

	ProcessDeviceStatusChange(oldNodeInfoDev, newNodeInfoDev)
}

func TestUpdateNodeDeviceState(t *testing.T) {
	oldStaticInfo := &v1.NodeStaticIOInfo{
		Status: v1.NodeStaticIOInfoStatus{
			DeviceStates: map[string]v1.DeviceState{
				"disk1": {
					State: utils.Ready,
					Name:  "disk1",
				},
				"disk2": {
					State: utils.Ready,
					Name:  "disk2",
				},
			},
		},
	}

	newStaticInfo := &v1.NodeStaticIOInfo{
		Status: v1.NodeStaticIOInfoStatus{
			DeviceStates: map[string]v1.DeviceState{
				"disk1": {
					State: utils.Ready,
					Name:  "disk1",
				},
				"disk2": {
					State: utils.Profiling,
					Name:  "disk2",
				},
				"disk3": {
					State: utils.NotEnabled,
					Name:  "disk3",
				},
			},
		},
	}

	expectedDevices := map[string][]string{
		"disk3": {"disk3"},
	}

	devices := GetDeviceChange(&(oldStaticInfo.Status), &(newStaticInfo.Status))

	// Check if devices is equal to expectedDevices
	if !reflect.DeepEqual(devices, expectedDevices) {
		t.Errorf("GetDeviceChange() returned unexpected devices.\nExpected: %v\nGot: %v", expectedDevices, devices)
	}
}

func TestGetValidDiskStatus(t *testing.T) {
	type args struct {
		diskStatus *map[string]int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test get valid disk status",
			args: args{
				diskStatus: &map[string]int{
					"disk1": 1,
					"disk2": 2,
					"disk3": 3,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GetValidDiskStatus(tt.args.diskStatus)
		})
	}
}

func TestSetProfilingDisk(t *testing.T) {
	type args struct {
		DeviceStatus map[string]int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test set profiling disk",
			args: args{
				DeviceStatus: map[string]int{
					"disk1": 1,
					"disk2": 2,
					"disk3": 3,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetProfilingDisk(tt.args.DeviceStatus)
		})
	}
}

func TestUpdateProfileDiskState(t *testing.T) {
	type args struct {
		c *v1.NodeStaticIOInfo
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test update profile disk state",
			args: args{
				c: &v1.NodeStaticIOInfo{
					Status: v1.NodeStaticIOInfoStatus{
						DeviceStates: map[string]v1.DeviceState{
							"disk1": {
								State: utils.Ready,
								Name:  "disk1",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateProfileDiskState(tt.args.c)
		})
	}
}

func TestNodeDeviceStateUpdate(t *testing.T) {
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test node device state update same object",
			args: args{
				oldObj: &v1.NodeStaticIOInfo{
					Status: v1.NodeStaticIOInfoStatus{
						DeviceStates: map[string]v1.DeviceState{
							"disk1": {
								State: utils.Ready,
								Name:  "disk1",
							},
						},
					},
				},
				newObj: &v1.NodeStaticIOInfo{
					Status: v1.NodeStaticIOInfoStatus{
						DeviceStates: map[string]v1.DeviceState{
							"disk1": {
								State: utils.Ready,
								Name:  "disk1",
							},
						},
					},
				},
			},
		},
		{
			name: "test node device state difference",
			args: args{
				oldObj: &v1.NodeStaticIOInfo{
					Status: v1.NodeStaticIOInfoStatus{
						DeviceStates: map[string]v1.DeviceState{
							"disk1": {
								State: utils.Ready,
								Name:  "disk1",
							},
						},
					},
				},
				newObj: &v1.NodeStaticIOInfo{
					Status: v1.NodeStaticIOInfoStatus{
						DeviceStates: map[string]v1.DeviceState{
							"disk1": {
								State: utils.Profiling,
								Name:  "disk1",
							},
						},
					},
				},
			},
		},
		{
			name: "test node device state update not same disk",
			args: args{
				oldObj: &v1.NodeStaticIOInfo{
					Status: v1.NodeStaticIOInfoStatus{
						DeviceStates: map[string]v1.DeviceState{
							"disk1": {
								State: utils.Ready,
								Name:  "disk1",
							},
						},
					},
				},
				newObj: &v1.NodeStaticIOInfo{
					Status: v1.NodeStaticIOInfoStatus{
						DeviceStates: map[string]v1.DeviceState{
							"disk1": {
								State: utils.Ready,
								Name:  "disk1",
							},
							"disk2": {
								State: utils.Ready,
								Name:  "disk2",
							},
						},
					},
				},
			},
		},
		{
			name: "test node device state update error new obj",
			args: args{
				oldObj: &v1.NodeStaticIOInfo{
					Status: v1.NodeStaticIOInfoStatus{
						DeviceStates: map[string]v1.DeviceState{
							"disk1": {
								State: utils.Ready,
								Name:  "disk1",
							},
						},
					},
				},
				newObj: &v1.Device{},
			},
		},
		{
			name: "test node device state update error old obj",
			args: args{
				oldObj: &v1.Device{},
				newObj: &v1.Device{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NodeDeviceStateUpdate(tt.args.oldObj, tt.args.newObj)
		})
	}
}
