/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"fmt"
	"os"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	externalInformer "sigs.k8s.io/IOIsolation/generated/ioi/informers/externalversions"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/IOIsolation/pkg/agent/common"
)

var Clientset *versioned.Clientset
var pDiskState *map[string]string
var pDiskValid *map[string]utils.BlockDevice
var pDiskName *map[string]string

func GetValidDiskStatus(diskStatus *map[string]int) {
	for did := range *diskStatus {
		if _, ok := (*pDiskValid)[did]; !ok {
			delete(*diskStatus, did)
		}
	}
}

func SetProfilingDisk(DeviceStatus map[string]int) {
	for did, status := range DeviceStatus {
		if status == 3 {
			(*pDiskState)[did] = utils.Fail
		}
		if status == 1 {
			(*pDiskState)[did] = utils.Profiling
		}
		if status == 2 {
			(*pDiskState)[did] = utils.Ready
		}
	}
}

// Update disk state after profiled
func UpdateProfileDiskState(c *v1.NodeStaticIOInfo) {
	klog.V(utils.DBG).Infof("UpdateProfileDiskState: pDiskState %v", *pDiskState)
	for did, state := range *pDiskState {
		diskName := (*pDiskName)[did]
		deviceStatus := c.Status.DeviceStates[did]
		// not in success map and in profiling map--> so the disk is profiling, no change the status.
		if state == utils.Ready {
			deviceStatus.Name = diskName
			deviceStatus.State = utils.Ready
		} else if state == utils.Fail {
			deviceStatus.Name = diskName
			deviceStatus.State = utils.NotEnabled
			deviceStatus.Reason = "Profile failed"

			delete(*pDiskValid, did)
		} else {
			deviceStatus.Name = diskName
			deviceStatus.State = state
		}
		c.Status.DeviceStates[did] = deviceStatus
	}
	klog.V(utils.DBG).Infof("UpdateProfileDiskState: %v", c.Status.DeviceStates)
}

// Start to profiler: filter the current disks, update the disk state
func FilterDisk(diskInfo agent.AllDiskInfo, disks []common.DiskInput) map[string]utils.BlockDevice {
	diskCurSelect := make(map[string]utils.BlockDevice)

	for did, disk := range diskInfo {
		for _, diskInput := range disks {
			if diskInput.DiskName == disk.Name {
				diskCurSelect[did] = disk
				(*pDiskValid)[did] = disk
				(*pDiskState)[did] = utils.Profiling
			} else {
				if _, ok := (*pDiskState)[did]; !ok {
					(*pDiskState)[did] = utils.NotEnabled
				}
			}
			(*pDiskName)[did] = disk.Name
		}
		if disk.Type == utils.EmptyDir {
			if _, ok := (*pDiskState)[did]; !ok { // not repeat select empty disk
				diskCurSelect[did] = disk
				(*pDiskValid)[did] = disk
				(*pDiskState)[did] = utils.Profiling
			}
			(*pDiskName)[did] = disk.Name
		} else {
			if _, ok := (*pDiskState)[did]; !ok {
				(*pDiskState)[did] = utils.NotEnabled
			}
			(*pDiskName)[did] = disk.Name
		}
	}

	klog.V(utils.DBG).Infof("FilterDisk:pDiskState %+v", *pDiskState)
	return diskCurSelect
}

func GetDeviceChange(oldNodeInfo *v1.NodeStaticIOInfoStatus, newNodeInfo *v1.NodeStaticIOInfoStatus) map[string][]string {
	devices := make(map[string][]string)
	if oldNodeInfo == nil || newNodeInfo == nil {
		return devices
	}
	oldDevices := oldNodeInfo.DeviceStates
	newDevices := newNodeInfo.DeviceStates
	for key, value := range newDevices {
		if _, ok := oldDevices[key]; !ok {
			devices[key] = []string{value.Name}
		}
	}
	return devices
}

func ProcessDeviceStatusChange(oldNodeInfo *v1.NodeStaticIOInfoStatus, newNodeInfo *v1.NodeStaticIOInfoStatus) {
	devices := make(map[string][]string)
	if oldNodeInfo == nil || newNodeInfo == nil {
		return
	}
	oldDevices := oldNodeInfo.DeviceStates
	newDevices := newNodeInfo.DeviceStates
	for did, newState := range newDevices {
		if oldState, ok1 := oldDevices[did]; !ok1 {
			klog.Warning("the newDevices is not same with oldDevices")
		} else {
			if newState.State == oldState.State {
				continue
			}
			// NotEnabled --> Profiling
			klog.V(utils.DBG).Infof("ProcessDeviceStatusChange: %s, %s", oldState.State, newState.State)
			if oldState.State == utils.NotEnabled && newState.State == utils.Profiling {
				NodeName := os.Getenv("Node_Name")
				devices[NodeName] = []string{newState.Name}
				go StartDiskProfiler(devices)
			}
			// Ready --> NotEnabled: delete spec info from cr
			if oldState.State == utils.Ready && newState.State == utils.NotEnabled {
				(*pDiskState)[did] = utils.NotEnabled
				delete((*pDiskValid), did)
				delete(DiskInfos, did)
				c := &agent.GetAgent().IOInfo
				delete(c.Spec.ResourceConfig[string(v1.BlockIO)].Devices, did)
				c.Status.DeviceStates[did] = newState
				if Clientset != nil {
					common.UpdateNodeStaticIOInfoSpec(c, Clientset)
				}
			}
		}
	}
}

func NodeDeviceStateUpdate(oldObj, newObj interface{}) {
	oldStaticInfo, ok := oldObj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[NodeDeviceStateUpdate]cannot convert oldObj to *v1.NodeStaticIOInfo: %v", oldObj)
		return
	}
	newStaticInfo, ok := newObj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[NodeDeviceStateUpdate]cannot convert newObj to *v1.NodeStaticIOInfo: %v", newObj)
		return
	}

	// Static cr status is not changed
	if utils.HashObject(oldStaticInfo.Status) == utils.HashObject(newStaticInfo.Status) {
		klog.V(utils.INF).Infof("[NodeDeviceStateUpdate] the status is not change: %v", newObj)
		return
	}

	klog.V(utils.DBG).Infof("NodeDeviceStateUpdate: \noldStaticInfo: %v \nnewStaticInfo: %v", oldStaticInfo, newStaticInfo)
	// process the disk change，profile it again
	disks := GetDeviceChange(&(oldStaticInfo.Status), &(newStaticInfo.Status))
	if len(disks) > 0 {
		klog.V(utils.INF).Infof("NodeDeviceStateUpdate scan new disks: %v", disks)
	} else {
		ProcessDeviceStatusChange(&(oldStaticInfo.Status), &(newStaticInfo.Status))
	}
}

func WatchDeviceStatus(client *versioned.Clientset) error {
	Clientset = client
	diskInformerFactory := externalInformer.NewSharedInformerFactory(client, 0)
	diskStatusInformer := diskInformerFactory.Ioi().V1().NodeStaticIOInfos().Informer()
	diskStatusInformer.AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				crStatus, ok := obj.(*v1.NodeStaticIOInfo)
				if !ok {
					klog.Errorf("cannot convert to *v1.NodeStaticIOInfo: %v", obj)
					return false
				}
				NodeName := os.Getenv("Node_Name")
				if crStatus.Name != NodeName+"-nodestaticioinfo" {
					return false
				} else {
					return true
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				UpdateFunc: NodeDeviceStateUpdate,
			},
		})

	diskInformerFactory.Start(nil)

	if !cache.WaitForCacheSync(nil, diskStatusInformer.HasSynced) {
		return fmt.Errorf("timed out waiting for caches to sync resource NodeDiskIOStatusInfo and NodeNetworkIOStatusInfo")
	}
	return nil
}

func LoadDiskInfo(cr *v1.NodeStaticIOInfo) {
	for did, disk := range cr.Status.DeviceStates {
		if disk.State == utils.Ready {
			(*pDiskState)[did] = utils.Ready
			(*pDiskName)[did] = disk.Name
			(*pDiskValid)[did] = utils.BlockDevice{Name: disk.Name}
		} else if disk.State == utils.Profiling {
			(*pDiskState)[did] = utils.Profiling
			(*pDiskName)[did] = disk.Name
			(*pDiskValid)[did] = utils.BlockDevice{Name: disk.Name}
		} else {
			(*pDiskState)[did] = utils.NotEnabled
			(*pDiskName)[did] = disk.Name
		}
	}
}

func init() {
	diskState := make(map[string]string)
	diskName := make(map[string]string)
	diskValid := make(map[string]utils.BlockDevice)

	pDiskState = &diskState
	pDiskName = &diskName
	pDiskValid = &diskValid
}
