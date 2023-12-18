/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package blockio

import (
	"context"
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	common "sigs.k8s.io/IOIsolation/pkg/scheduler/util"
)

type IOResourceRequest struct {
	IORequest      *pb.IOResourceRequest
	StorageRequest int64
}

type RequestInfo struct {
	ResourceRequest map[string]*utils.IOResourceRequest
	PvcToDevice     map[string]*utils.PVCInfo
}

type NodeInfo struct {
	DisksStatus    map[string]*DiskInfo // disk device uuid : diskStatus
	EmptyDirDevice string               // disk device uuid
}

type DiskInfo struct {
	DiskName           string
	ReadRatio          []utils.MappingRatio //sorted array according to block size
	WriteRatio         []utils.MappingRatio
	BEDefaultRead      float64
	BEDefaultWrite     float64
	GA                 *utils.IOPoolStatus
	BE                 *utils.IOPoolStatus
	Capacity           int64
	RequestedDiskSpace int64
	MountPath          string
}

func (d *DiskInfo) DeepCopy() *DiskInfo {
	newDiskInfo := &DiskInfo{
		DiskName:           d.DiskName,
		ReadRatio:          make([]utils.MappingRatio, len(d.ReadRatio)),
		WriteRatio:         make([]utils.MappingRatio, len(d.WriteRatio)),
		BEDefaultRead:      d.BEDefaultRead,
		BEDefaultWrite:     d.BEDefaultWrite,
		GA:                 &utils.IOPoolStatus{},
		BE:                 &utils.IOPoolStatus{},
		Capacity:           d.Capacity,
		RequestedDiskSpace: d.RequestedDiskSpace,
	}
	copy(newDiskInfo.ReadRatio, d.ReadRatio)
	copy(newDiskInfo.WriteRatio, d.WriteRatio)
	newDiskInfo.GA = d.GA.DeepCopy()
	newDiskInfo.BE = d.BE.DeepCopy()

	return newDiskInfo
}

func NewDiskInfo() *DiskInfo {
	diskInfo := &DiskInfo{
		ReadRatio:  []utils.MappingRatio{},
		WriteRatio: []utils.MappingRatio{},
		GA:         &utils.IOPoolStatus{},
		BE:         &utils.IOPoolStatus{},
	}
	return diskInfo
}

type Resource struct {
	nodeName string
	info     *NodeInfo
	ch       resource.CacheHandle
	sync.RWMutex
}

func (ps *Resource) Name() string {
	return "BlockIO"
}

func (ps *Resource) AddPod(pod *v1.Pod, rawRequest interface{}, client kubernetes.Interface) ([]*pb.IOResourceRequest, bool, error) {
	request, ok := rawRequest.(*RequestInfo)
	if !ok {
		return nil, false, fmt.Errorf("unable to convert state into %v's pod request format", ps.Name())
	}
	requestsArray := []*pb.IOResourceRequest{}
	for dev, ioreq := range request.ResourceRequest {
		diskStatus, exist := ps.info.DisksStatus[dev]
		if !exist {
			return nil, true, fmt.Errorf("node %v doesn't contain device %v info", ps.nodeName, dev)
		}
		if ioreq.IORequest.WorkloadType == utils.GaIndex {
			diskStatus.GA.In -= ioreq.IORequest.DevRequest.Request.Inbps
			diskStatus.GA.Out -= ioreq.IORequest.DevRequest.Request.Outbps
			diskStatus.GA.Total -= (ioreq.IORequest.DevRequest.Request.Inbps + ioreq.IORequest.DevRequest.Request.Outbps)
		} else if ioreq.IORequest.WorkloadType == utils.BeIndex {
			diskStatus.BE.Num += 1
		}
		requestsArray = append(requestsArray, ioreq.IORequest)
	}
	if request.PvcToDevice != nil && len(request.PvcToDevice) > 0 {
		// Update configmap to set volume device mapping
		err := ps.volumeDeviceMapping(request.PvcToDevice, pod, ps.nodeName, client)
		if err != nil {
			return nil, true, err
		}
	}
	ps.ch.PrintCacheInfo()
	return requestsArray, true, nil
}

func (ps *Resource) RemovePod(pod *v1.Pod, client kubernetes.Interface) error {
	resource.IoiContext.Lock()
	defer resource.IoiContext.Unlock()
	podreq, err := resource.IoiContext.GetReservedPods(ps.nodeName)
	if err != nil {
		return err
	}
	pl, ok := podreq.PodList[string(pod.UID)]
	if !ok {
		return fmt.Errorf("pod doesn't exist in reserved pod cache: %v", pod.Name)
	}
	for _, v := range pl.Request {
		if v.IoType != pb.IOType_DiskIO {
			continue
		}
		dev := v.DevRequest.DevName
		diskStatus, exist := ps.info.DisksStatus[dev]
		if !exist {
			return fmt.Errorf("node %v doesn't contain device %v info", ps.nodeName, dev)
		}
		if v.WorkloadType == utils.GaIndex {
			diskStatus.GA.In += v.DevRequest.Request.Inbps
			diskStatus.GA.Out += v.DevRequest.Request.Outbps
			diskStatus.GA.Total += (v.DevRequest.Request.Inbps + v.DevRequest.Request.Outbps)
		}
		if v.WorkloadType == utils.BeIndex {
			diskStatus.BE.Num -= 1
			if diskStatus.BE.Num < 0 {
				diskStatus.BE.Num = 0
			}
		}
	}
	if client != nil && ps.podHasPVCs(pod, client) {
		err = ps.undoVolumeDeviceMapping(pod, ps.nodeName, client)
		if err != nil {
			klog.Error("undo VolumeDeviceMapping error: ", err.Error())
		}
	}
	return nil
}
func (ps *Resource) podHasPVCs(pod *v1.Pod, client kubernetes.Interface) bool {
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			pvc := vol.PersistentVolumeClaim.ClaimName
			ok, err := utils.IsIOIProvisioner(client, pvc, pod.Namespace)
			if err != nil || !ok {
				continue
			}
			return true
		}
	}
	return false
}

func (ps *Resource) volumeDeviceMapping(pvcMap map[string]*utils.PVCInfo, pod *v1.Pod, nodename string, client kubernetes.Interface) error {
	klog.V(utils.DBG).InfoS("Enter VolumeDeviceMapping for pod", "pod", klog.KObj(pod))
	mapping := map[string]string{}
	resource.IoiContext.Lock()
	for _, vol := range pod.Spec.Volumes {
		// volume uses PVC
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		pvcName := vol.PersistentVolumeClaim.ClaimName
		ok, err := utils.IsIOIProvisioner(client, pvcName, pod.Namespace)
		if err != nil || !ok {
			continue
		}
		// format current pvc key-value
		key := fmt.Sprintf("%s.%s", pvcName, pod.Namespace)
		dev, ok := pvcMap[key]
		if !ok {
			return fmt.Errorf("unable to get the reserved disk id for pvc %s", pvcName)
		}
		if !dev.Reuse {
			diskStatus, exist := ps.info.DisksStatus[dev.DevID]
			if !exist {
				return fmt.Errorf("node %v doesn't contain device %v info", ps.nodeName, dev.DevID)
			}
			mountPath := diskStatus.MountPath
			mapping[key] = fmt.Sprintf("%s:%s", dev.DevID, mountPath)
			// add device binding info in cache
			resource.IoiContext.ClaimDevMap[key] = &resource.StorageInfo{
				DevID:            dev.DevID,
				RequestedStorage: dev.ReqStorage,
				NodeName:         nodename,
			}
		}
	}
	resource.IoiContext.Unlock()
	// client-go is thread-safe
	// Update the bound deviceID to ConfigMap
	if len(mapping) > 0 {
		cmName := fmt.Sprintf("%s-%s", utils.DeviceIDConfigMap, nodename)
		cm, err := common.GetVolumeDeviceMap(client, nodename)
		if kubeerr.IsNotFound(err) {
			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: utils.IoiNamespace,
					Name:      cmName,
					Labels:    map[string]string{"app": utils.DeviceIDConfigMap},
				},
				Data: mapping,
			}
			cm, err := client.CoreV1().ConfigMaps(utils.IoiNamespace).Create(context.Background(), cm, metav1.CreateOptions{})
			if err != nil {
				klog.ErrorS(err, "Failed to create ConfigMap volume-device-map", "node", nodename)
				return err
			}
			klog.V(utils.DBG).InfoS("ConfigMap volume-device-map has been created", "node", nodename, "data", cm.Data)
		} else if err != nil {
			return fmt.Errorf("failed to get ConfigMap volume-device-map: nodename %s, err: %v ", nodename, err)
		} else {
			cm = cm.DeepCopy()
			if cm.Data == nil {
				cm.Data = make(map[string]string)
			}
			for k, v := range mapping {
				cm.Data[k] = v
			}
			configmap, err := client.CoreV1().ConfigMaps(utils.IoiNamespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
			if err != nil {
				klog.ErrorS(err, "failed to update volume/disk mapping info for pod", "pod", pod.Name)
				return err
			}
			klog.V(utils.DBG).InfoS("ConfigMap volume-device-map has been updated", "node", nodename, "data", configmap.Data)
		}
	}
	return nil
}

func (ps *Resource) undoVolumeDeviceMapping(pod *v1.Pod, nodename string, client kubernetes.Interface) error {
	klog.V(utils.DBG).InfoS("Enter UndoVolumeDeviceMapping for pod", "pod", klog.KObj(pod))
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			pvc := vol.PersistentVolumeClaim.ClaimName
			ok, err := utils.IsIOIProvisioner(client, pvc, pod.Namespace)
			if err != nil || !ok {
				continue
			}
			obj, err := client.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(context.TODO(), pvc, metav1.GetOptions{})
			if err != nil {
				klog.V(utils.INF).Infof("Get PVC error: %v", err)
				continue
			}
			if obj.Status.Phase != v1.ClaimPending {
				continue
			}
			err = common.RemovePVCsFromVolDiskCm(client, obj, nodename)
			if err != nil {
				klog.Error("Remove all pods from volume-device-map err: ", err)
				return err
			}
			// remove device binding in cache
			resource.IoiContext.Lock()
			defer resource.IoiContext.Unlock()
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil {
					pvc := vol.PersistentVolumeClaim.ClaimName
					ok, err := utils.IsIOIProvisioner(client, pvc, pod.Namespace)
					if err != nil || !ok {
						continue
					}
					delete(resource.IoiContext.ClaimDevMap, fmt.Sprintf("%s.%s", vol.PersistentVolumeClaim.ClaimName, pod.Namespace))
				}
			}
		}
	}
	return nil
}

func (ps *Resource) AdmitPod(pod *v1.Pod) (interface{}, error) {
	request, pvcMapping, err := ps.ch.GetRequestByAnnotation(pod, ps.nodeName, false) // we only consider WaitForFirstConsumer
	if err != nil {
		return nil, err
	}
	pvcToDevice, ok := pvcMapping.(map[string]*utils.PVCInfo)
	if !ok {
		return nil, fmt.Errorf("pvcMapping cannot convert to pvcToDevice type")
	}
	sReq := make(map[string]int64) // key -> devid, val -> requested storage
	for _, info := range pvcToDevice {
		if info != nil && !info.Reuse {
			_, ok := sReq[info.DevID]
			if ok {
				sReq[info.DevID] += info.ReqStorage
			} else {
				sReq[info.DevID] = info.ReqStorage
			}
		}
	}
	ret := &RequestInfo{
		ResourceRequest: make(map[string]*utils.IOResourceRequest),
		PvcToDevice:     pvcToDevice,
	}
	for dev, req := range request {
		v, ok := sReq[dev]
		if !ok { // emptyDir case
			v = 0
		}
		ret.ResourceRequest[dev] = &utils.IOResourceRequest{
			IORequest:      req,
			StorageRequest: v,
		}
		klog.V(utils.DBG).Infof("blockio: device %v io request: %v, disk request: %d", dev, req.String(), v)
	}
	return ret, nil
}

func (ps *Resource) Clone() resource.ExtendedResource {
	// todo BlockIO deepcopy
	return ps
}

func (ps *Resource) PrintInfo() {

	for disk, diskInfo := range ps.info.DisksStatus {
		klog.V(utils.DBG).Info("***device: ", disk, " ***")
		klog.V(utils.DBG).Info("device name: ", diskInfo.DiskName)
		klog.V(utils.DBG).Info("ReadRatio: ", diskInfo.ReadRatio)
		klog.V(utils.DBG).Info("WriteRatio: ", diskInfo.WriteRatio)
		klog.V(utils.DBG).Info("BEDefaultRead: ", diskInfo.BEDefaultRead)
		klog.V(utils.DBG).Info("BEDefaultWrite: ", diskInfo.BEDefaultWrite)
		klog.V(utils.DBG).Info("GA: ", *diskInfo.GA)
		klog.V(utils.DBG).Info("BE: ", *diskInfo.BE)
		klog.V(utils.DBG).Info("Storage capacity: ", diskInfo.Capacity)
		klog.V(utils.DBG).Info("Storage used: ", diskInfo.RequestedDiskSpace)
	}
}
