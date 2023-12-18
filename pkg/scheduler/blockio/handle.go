/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package blockio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/component-helpers/storage/volume"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	common "sigs.k8s.io/IOIsolation/pkg/scheduler/util"
)

type Handle struct {
	ioiresource.HandleBase
	client kubernetes.Interface
	packer BinPacker
	sync.RWMutex
}

// ioiresource.Handle interface
func (h *Handle) Name() string {
	return "BlockIOHandle"
}

func (h *Handle) Run(c ioiresource.ExtendedCache, f informers.SharedInformerFactory, cli kubernetes.Interface) error {
	h.EC = c
	h.client = cli
	return nil
}

func addPodRequest(m, reqs map[string]*pb.IOResourceRequest) {
	for dev, req := range reqs {
		if _, ok := m[dev]; !ok {
			m[dev] = req
		} else {
			if req.WorkloadType == utils.GaIndex {
				// Once a device is assigned to GA workload, it will not be assigned to BE workload
				m[dev].WorkloadType = utils.GaIndex
				if m[dev].DevRequest.Request == nil {
					m[dev].DevRequest.Request = &pb.Quantity{}
				}
				if m[dev].DevRequest.Limit == nil {
					m[dev].DevRequest.Limit = &pb.Quantity{}
				}
				if m[dev].DevRequest.RawRequest == nil {
					m[dev].DevRequest.RawRequest = &pb.Quantity{}
				}
				if m[dev].DevRequest.RawLimit == nil {
					m[dev].DevRequest.RawLimit = &pb.Quantity{}
				}
				m[dev].DevRequest.Request.Inbps += req.DevRequest.Request.Inbps
				m[dev].DevRequest.Request.Outbps += req.DevRequest.Request.Outbps
				m[dev].DevRequest.Limit.Inbps += req.DevRequest.Limit.Inbps
				m[dev].DevRequest.Limit.Outbps += req.DevRequest.Limit.Outbps

				m[dev].DevRequest.RawRequest.Inbps += req.DevRequest.RawRequest.Inbps
				m[dev].DevRequest.RawRequest.Outbps += req.DevRequest.RawRequest.Outbps
				m[dev].DevRequest.RawLimit.Inbps += req.DevRequest.RawLimit.Inbps
				m[dev].DevRequest.RawLimit.Outbps += req.DevRequest.RawLimit.Outbps

				m[dev].DevRequest.RawRequest.Iniops += req.DevRequest.RawRequest.Iniops
				m[dev].DevRequest.RawRequest.Outiops += req.DevRequest.RawRequest.Outiops
				m[dev].DevRequest.RawLimit.Iniops += req.DevRequest.RawLimit.Iniops
				m[dev].DevRequest.RawLimit.Outiops += req.DevRequest.RawLimit.Outiops
			}
			if req.WorkloadType == utils.BeIndex && m[dev].WorkloadType != utils.GaIndex {
				// If the device is not assigned to GA workload, it can be assigned to BE workload currently.
				m[dev].WorkloadType = utils.BeIndex
			}
		}
	}
}

func (h *Handle) CanAdmitPod(nodeName string, rawRequest interface{}) (bool, interface{}, error) {
	requests, ok := rawRequest.(*RequestInfo)
	if !ok {
		return false, nil, fmt.Errorf("unable to convert state into %v's pod request format", h.Name())
	}
	// prepare for node allocatable resource
	pools := make(map[string]map[string]*utils.IOPoolStatus)
	rs := h.EC.GetExtendedResource(nodeName)
	if rs == nil {
		klog.Errorf("node %v not registered in cache", nodeName)
		return false, nil, fmt.Errorf("node %v not registered in cache", nodeName)
	}
	r, ok := rs.(*Resource)
	if !ok {
		klog.Error("incorrect resource cached")
		return false, nil, fmt.Errorf("incorrect resource cached")
	}
	for dev, diskStatus := range r.info.DisksStatus {
		poolStatus := make(map[string]*utils.IOPoolStatus)
		poolStatus[utils.GA_Type] = diskStatus.GA
		poolStatus[utils.BE_Type] = diskStatus.BE
		poolStatus[utils.BE_Min_BW] = &utils.IOPoolStatus{
			In:    diskStatus.BEDefaultRead,
			Out:   diskStatus.BEDefaultWrite,
			Total: diskStatus.BEDefaultRead + diskStatus.BEDefaultWrite,
		}
		poolStatus[utils.AVAIL_STORAGE] = &utils.IOPoolStatus{
			Total: float64(diskStatus.Capacity - diskStatus.RequestedDiskSpace),
		}
		pools[dev] = poolStatus
	}
	for dev, req := range requests.ResourceRequest {
		request := req.IORequest
		poolStatus, ok := pools[dev]
		if !ok {
			return false, nil, fmt.Errorf("device %v on Node %v is not registed: %v", dev, nodeName, h.Name())
		}
		if poolStatus == nil || len(poolStatus) < 4 {
			return false, nil, fmt.Errorf("internal %v error on Node %v", h.Name(), nodeName)
		}
		ga := poolStatus[utils.GA_Type]
		be := poolStatus[utils.BE_Type]
		bem := poolStatus[utils.BE_Min_BW]
		st := poolStatus[utils.AVAIL_STORAGE]
		if req.StorageRequest > int64(st.Total) {
			return false, nil, fmt.Errorf("insufficient %v storage space on node %v", h.Name(), nodeName)
		}
		if request.WorkloadType == utils.GaIndex {
			if ga.UnderPressure {
				return false, nil, fmt.Errorf("device on Node %v is under %v pressure", nodeName, h.Name())
			}
			if request.DevRequest.Request.Inbps+request.DevRequest.Request.Outbps > ga.Total {
				return false, nil, fmt.Errorf("insufficient %v total IO resource on node %v", h.Name(), nodeName)
			}
			if request.DevRequest.Request.Inbps > ga.In {
				return false, nil, fmt.Errorf("insufficient %v in IO resource on node %v", h.Name(), nodeName)
			}
			if request.DevRequest.Request.Outbps > ga.Out {
				return false, nil, fmt.Errorf("insufficient %v out IO resource on node %v", h.Name(), nodeName)
			}
		} else if request.WorkloadType == utils.BeIndex {
			if be.UnderPressure {
				return false, nil, fmt.Errorf("device on Node %v is under %v pressure", nodeName, h.Name())
			}
			if be.Total/float64(be.Num+1) <= bem.Total {
				return false, nil, fmt.Errorf("insufficient %v total BE IO resource on node %v", h.Name(), nodeName)
			}
			if be.In/float64(be.Num+1) <= bem.In {
				return false, nil, fmt.Errorf("insufficient %v in BE IO resource on node %v", h.Name(), nodeName)
			}
			if be.Out/float64(be.Num+1) <= bem.Out {
				return false, nil, fmt.Errorf("insufficient %v out BE IO resource on node %v", h.Name(), nodeName)
			}
		}
	}
	return true, pools, nil
}
func (h *Handle) NodePressureRatio(request interface{}, pool interface{}) (float64, error) {
	var ratio float64 = 0.0
	r, ok := request.(*RequestInfo)
	if !ok {
		return 0, fmt.Errorf("unable to convert state into %v's pod request format", h.Name())
	}
	p, ok := pool.(map[string]map[string]*utils.IOPoolStatus)
	if !ok {
		return 0, fmt.Errorf("unable to convert state into %v's node resource pool format", h.Name())
	}
	nDev := len(r.ResourceRequest)
	for dev, req := range r.ResourceRequest {
		if req.IORequest.WorkloadType == utils.BeIndex {
			ratio += 0
			continue
		}
		devicePool, ok := p[dev]
		if !ok {
			return 0, fmt.Errorf("cannot find device %v in pool", dev)
		}
		gapool, ok := devicePool[utils.GA_Type]
		if !ok {
			return 0, fmt.Errorf("cannot find gapool in pool")
		}
		var devRatio float64 = 0.0
		if gapool.In != 0 {
			devRatio += (req.IORequest.DevRequest.Request.Inbps) / gapool.In
		} else {
			devRatio += 1
		}
		if gapool.Out != 0 {
			devRatio += (req.IORequest.DevRequest.Request.Outbps) / gapool.Out
		} else {
			devRatio += 1
		}
		if gapool.Total != 0 {
			devRatio += (req.IORequest.DevRequest.Request.Inbps + req.IORequest.DevRequest.Request.Outbps) / gapool.Total
		} else {
			devRatio += 1
		}
		ratio += devRatio / 3.0
	}
	if nDev == 0 {
		return 0.5, nil
	}
	return ratio / float64(nDev), nil
}

// return a map volumeName:device of an existing pod
func (h *Handle) getDiskFromContext(pod *corev1.Pod, nodename string, defaultDev string) (map[string]string, error) {
	mapping := map[string]string{}
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			volumeName := vol.Name
			pvcName := vol.PersistentVolumeClaim.ClaimName
			ok, err := utils.IsIOIProvisioner(h.client, pvcName, pod.Namespace)
			if err != nil || !ok {
				continue
			}
			s, err := ioiresource.IoiContext.GetStorageInfo(fmt.Sprintf("%v.%v", pvcName, pod.Namespace))
			if err != nil {
				klog.Errorf("cannot get pvc name: %v", err)
				continue
			}
			mapping[volumeName] = s.DevID
		} else if vol.EmptyDir != nil {
			volumeName := vol.Name
			mapping[volumeName] = defaultDev
		}

	}
	return mapping, nil
}

func (h *Handle) getBoundedPVCInfo(pod *corev1.Pod, node string) (map[string]*utils.PVCInfo, error) {
	pvcToDevice := make(map[string]*utils.PVCInfo) // pvc.namespace -> pvc info
	for _, vol := range pod.Spec.Volumes {         // only check pvc type with the provisioner
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		pvc := vol.PersistentVolumeClaim.ClaimName
		ok, err := utils.IsIOIProvisioner(h.client, pvc, pod.Namespace)
		if err != nil || !ok {
			continue
		}
		claim, err := h.client.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(context.TODO(), pvc, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pvc %s when getting bound info, err: %s", pvc, err)
		}
		n, ok := claim.Annotations[volume.AnnSelectedNode]
		if !ok {
			continue
		}
		// if pvc bounded node is not the current, return error
		if n != node {
			return nil, fmt.Errorf("PVC %v is bounded to node %v, not %v", pvc, n, node)
		}
		reqStorage, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]
		if !ok {
			// No capacity to check for. Should not happen.
			klog.Errorf("the PVC storage request is not specified.")
		}
		if claim.Status.Phase == corev1.ClaimPending {
			// if pvc is not bound, skip it, will treat it as a new pvc
			continue
		}
		for k, v := range ioiresource.IoiContext.ClaimDevMap {
			if k == pvc+"."+pod.Namespace {
				pvcInfo := &utils.PVCInfo{
					DevID:      v.DevID,
					ReqStorage: reqStorage.Value(),
					Reuse:      true,
				}
				pvcToDevice[k] = pvcInfo
			}
		}
	}
	return pvcToDevice, nil
}

func (h *Handle) GetRequestByAnnotation(pod *corev1.Pod, node string, init bool) (map[string]*pb.IOResourceRequest, interface{}, error) {
	requests := map[string]*pb.IOResourceRequest{}
	anno := utils.PodAnnoThroughput{}
	rs := h.EC.GetExtendedResource(node)
	if rs == nil {
		return nil, nil, fmt.Errorf("node not registered in cache")
	}
	r, ok := rs.(*Resource)
	if !ok {
		return nil, nil, fmt.Errorf("incorrect resource cached")
	}
	r.Lock()
	defer r.Unlock()
	if r.info == nil {
		return nil, nil, fmt.Errorf("blockio: node %v cache info is not registed", node)
	}
	if v, ok := pod.Annotations[utils.BlockIOConfigAnno]; ok {
		err := json.Unmarshal([]byte(v), &anno)
		if err != nil {
			klog.Errorf("cannot parse %v to block io request/limit: %v", v, err)
		}
	}
	// If it is for scheduler cache initialization, get existing binding info from configmap
	if init {
		// returns all volumes mapped devices in this pod, including pvc and emptydir type
		volumeMapping, err := h.getDiskFromContext(pod, node, r.info.EmptyDirDevice)
		if err != nil {
			klog.Errorf("pod %v in node %v get pvc mapping error:", pod.Name, node, err)
			return nil, nil, fmt.Errorf("pod %v in node %v get pvc mapping error: %v", pod.Name, node, err)
		}
		// volumeMapping includes all volumes in the pod, including pvc and emptydir type
		requests = h.getVolumeRequestFromAnno(volumeMapping, anno, r.info)
		return requests, nil, nil
	}
	var hasEmptDir, hasPVC bool
	// step 1: Check emptyDir type volumes first
	emptyDirInfo, ok := r.info.DisksStatus[r.info.EmptyDirDevice]
	if !ok {
		return nil, nil, fmt.Errorf("emptyDir's disk %v not registed in node %v", r.info.EmptyDirDevice, node)
	}
	for _, vol := range pod.Spec.Volumes {
		if vol.EmptyDir == nil {
			continue
		}
		hasEmptDir = true
		if v, ok := anno[vol.Name]; ok { // emptyDir typed volume io bandwidth is specified in pod annotation
			// get volume request and limit bandwidth, find its QoS and add it to devices' requests map
			addPodRequest(requests, h.parsePVIOAnnotations(v, r.info.EmptyDirDevice, emptyDirInfo))
		} else {
			addPodRequest(requests, h.parseBEIOAnnotations(r.info.EmptyDirDevice, emptyDirInfo))
		}
	}
	// step 2: Check whether pvc already bounded, then place volumes already bounded to device
	pvcToDevice, err := h.getBoundedPVCInfo(pod, node)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to schedule pod %v: %v", pod.Name, err)
	}
	if len(pvcToDevice) > 0 {
		klog.V(utils.DBG).Info("existing PVC to disk mapping:", pvcToDevice)
		existVolReq, err := h.getBoundedTotalRequests(pod, anno, pvcToDevice, r.info.DisksStatus)
		klog.V(utils.DBG).Infof("bounded device accumulated io request of pvc: %v", existVolReq)
		if err != nil {
			klog.Error(err)
			return nil, nil, err
		}
		addPodRequest(requests, existVolReq)
	}
	// step 3: Find a suitable device for pvc volume not bounded to device using bin pack algorithm.
	restDisksStatus := h.getRestDisksStatus(r.info.DisksStatus, requests, pvcToDevice)
	restVolumes, err := h.getRestVolumes(pod, anno, pvcToDevice)
	if err != nil {
		return nil, nil, err
	}
	if len(restVolumes) > 0 {
		restVolumeToDevice, restPvcToDevice, ok := h.packer.Pack(restDisksStatus, restVolumes)
		if !ok {
			return nil, nil, fmt.Errorf("cannot pack pvc volume to device")
		}
		if len(restPvcToDevice) > 0 {
			// get the request of the rest pvc volumes
			restRequests := h.getVolumeRequestFromAnno(restVolumeToDevice, anno, r.info)
			addPodRequest(requests, restRequests)
		}
		utils.MergePVCInfo(pvcToDevice, restPvcToDevice)
	}
	if len(pvcToDevice) > 0 {
		hasPVC = true
	}
	// if the pod has neither pvc nor emptyDir, add the request to the default device in BE type
	if !hasPVC && !hasEmptDir {
		addPodRequest(requests, h.parseBEIOAnnotations(r.info.EmptyDirDevice, emptyDirInfo))
	}
	return requests, pvcToDevice, nil
}

func (h *Handle) getRestVolumes(pod *corev1.Pod, anno utils.PodAnnoThroughput, pvcToDevice map[string]*utils.PVCInfo) ([]*Volume, error) {
	volumes := []*Volume{}
	for _, vol := range pod.Spec.Volumes {
		// only check pvc type with the provisioner
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		pvc := vol.PersistentVolumeClaim.ClaimName
		ok, err := utils.IsIOIProvisioner(h.client, pvc, pod.Namespace)
		if err != nil || !ok {
			continue
		}
		// skip pvc already bounded to device
		k := pvc + "." + pod.Namespace
		if _, ok := pvcToDevice[k]; ok {
			continue
		}
		claim, err := h.client.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(context.TODO(), pvc, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pvc %s when getting bound info, err: %s", pvc, err)
		}
		reqStorage := claim.Spec.Resources.Requests[corev1.ResourceStorage]
		var qos string
		var rawRead, rawWrite float64
		var bsRead, bsWrite resource.Quantity
		qos = NotBE
		if v, ok := anno[vol.Name]; ok {
			rbpsbw, rbpsbs, rrerr := utils.ParseDiskRequestBandwidth(v.Requests.Rbps)
			if rrerr == nil {
				bwValue, _ := rbpsbw.AsInt64()
				rawRead = float64(bwValue) / utils.MiB
				bsRead = rbpsbs
			} else {
				qos = BE
			}

			wbpsbw, wbpsbs, wrerr := utils.ParseDiskRequestBandwidth(v.Requests.Wbps)
			if wrerr == nil {
				bwValue, _ := wbpsbw.AsInt64()
				rawWrite = float64(bwValue) / utils.MiB
				bsWrite = wbpsbs
			} else {
				qos = BE
			}
		} else {
			qos = BE
		}
		vol := &Volume{
			Name:           vol.Name,
			PvcName:        k,
			QoS:            qos,
			RawRead:        rawRead,
			RawWrite:       rawWrite,
			BlockSizeRead:  bsRead,
			BlockSizeWrite: bsWrite,
			Size:           reqStorage.Value(),
		}
		volumes = append(volumes, vol)
	}
	return volumes, nil
}
func (h *Handle) getRestDisksStatus(disksStatus map[string]*DiskInfo, requests map[string]*pb.IOResourceRequest, pvcToDevice map[string]*utils.PVCInfo) map[string]*DiskInfo {
	rest := make(map[string]*DiskInfo)
	stMap := make(map[string]int64)
	for _, info := range pvcToDevice {
		if !info.Reuse {
			stMap[info.DevID] += info.ReqStorage
		}
	}
	for k, v := range disksStatus {
		r := v.DeepCopy()
		request, ok := requests[k]
		if ok {
			if requests[k].WorkloadType == utils.GaIndex {
				r.GA.In -= request.DevRequest.Request.Inbps
				r.GA.Out -= request.DevRequest.Request.Outbps
				r.GA.Total -= request.DevRequest.Request.Inbps + request.DevRequest.Request.Outbps
			}
		}
		st, ok := stMap[k]
		if ok {
			r.RequestedDiskSpace += st
		}
		rest[k] = r
	}
	return rest
}

// total io request in a certain device if run all PVC's volume in this device
func (h *Handle) getBoundedTotalRequests(pod *corev1.Pod, anno utils.PodAnnoThroughput, pvcToDevice map[string]*utils.PVCInfo, disksStatus map[string]*DiskInfo) (map[string]*pb.IOResourceRequest, error) {
	requests := map[string]*pb.IOResourceRequest{}
	for _, vol := range pod.Spec.Volumes {
		// only check pvc type with the provisioner
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		pvc := vol.PersistentVolumeClaim.ClaimName
		k := pvc + "." + pod.Namespace
		info, ok := pvcToDevice[k]
		if !ok {
			continue
		}
		ok, err := utils.IsIOIProvisioner(h.client, pvc, pod.Namespace)
		if err != nil || !ok {
			continue
		}
		devInfo, ok := disksStatus[info.DevID]
		if !ok {
			klog.Errorf("device %v not registed in node", info.DevID)
			continue
		}
		// pvc has been assigned in the device
		if v, ok := anno[vol.Name]; ok { // pvc typed volume io bandwidth is specified in pod annotation
			// get volume request and limit bandwidth, find its QoS and find a suitable device for the volume, and add it to devices' requests map
			addPodRequest(requests, h.parsePVIOAnnotations(v, info.DevID, devInfo))
		} else { // pvc typed volume io bandwidth is BE
			// get volume request and limit bandwidth, and find a suitable device for the volume, add it to devices' requests map
			addPodRequest(requests, h.parseBEIOAnnotations(info.DevID, devInfo))
		}
	}
	return requests, nil
}

func (h *Handle) CriticalPod(annotations map[string]string) bool {
	anno := utils.PodAnnoThroughput{}
	if v, ok := annotations[utils.BlockIOConfigAnno]; ok {
		if err := json.Unmarshal([]byte(v), &anno); err != nil {
			return false
		}
		return true
	}
	return false
}

// calculate the reserved BW for GA and BT workloads on one device
// return the reserved rbps/wbps and if it is BE workload
func (h *Handle) calculateReservedBw(dev string, l *pb.PodRequest) (float64, float64, bool) {
	var in, out float64
	for _, r := range l.Request {
		if r.IoType != pb.IOType_DiskIO || r.DevRequest == nil || r.DevRequest.DevName != dev {
			continue
		}
		if r.WorkloadType == utils.BeIndex {
			return in, out, true
		}
		if r.DevRequest.Request != nil {
			in += r.DevRequest.Request.Inbps
			out += r.DevRequest.Request.Outbps
		}
	}
	return in, out, false
}

func (h *Handle) InitBENum(nodeName string, rc v1.ResourceConfigSpec) {
	klog.V(utils.DBG).Info("Enter init BE num")
	rs := h.EC.GetExtendedResource(nodeName)
	if rs == nil {
		klog.Error("node not registered in cache")
		return
	}
	r, ok := rs.(*Resource)
	if !ok {
		klog.Error("incorrect resource cached")
		return
	}

	pl, err := ioiresource.IoiContext.GetReservedPods(nodeName)
	if err != nil {
		klog.Errorf("node %v does not registed in reserved pod cache")
	}
	h.Lock()
	defer h.Unlock()
	for dev, info := range rc.Devices {
		if _, ok := r.info.DisksStatus[dev]; !ok {
			klog.Errorf("device %v not registed in status cache", dev)
			continue
		}
		klog.V(utils.DBG).Infof("device %v info from cr: %v", dev, info)
		var beNum int
		for _, l := range pl.PodList {
			_, _, isbe := h.calculateReservedBw(dev, l)
			if isbe {
				beNum += 1
			}
		}
		if r.info.DisksStatus[dev].BE != nil {
			r.info.DisksStatus[dev].BE.Num = beNum
		} else {
			r.info.DisksStatus[dev].BE = &utils.IOPoolStatus{
				Num: beNum,
			}
		}
	}
	h.EC.PrintCacheInfo()
}

func (h *Handle) InitNodeStatus(nodeName string, rc v1.ResourceConfigSpec) {
	rs := h.EC.GetExtendedResource(nodeName)
	if rs == nil {
		klog.Error("node not registered in cache")
		return
	}
	r, ok := rs.(*Resource)
	if !ok {
		klog.Error("incorrect resource cached")
		return
	}
	h.Lock()
	defer h.Unlock()
	pl, err := ioiresource.IoiContext.GetReservedPods(nodeName)
	if err != nil {
		klog.Errorf("node %v does not registed in reserved pod cache")
	}
	for dev, info := range rc.Devices {
		if _, ok := r.info.DisksStatus[dev]; !ok {
			klog.Errorf("device %v not registed in status cache", dev)
			continue
		}
		var gaIn, gaOut float64
		var beNum int
		for _, l := range pl.PodList {
			gain, gaout, isbe := h.calculateReservedBw(dev, l)
			gaIn += gain
			gaOut += gaout
			if isbe {
				beNum += 1
			}
		}
		r.info.DisksStatus[dev].GA = &utils.IOPoolStatus{
			In:            float64((100-rc.BEPool)*int(info.CapacityIn)/100) - gaIn,
			Out:           float64((100-rc.BEPool)*int(info.CapacityOut)/100) - gaOut,
			Total:         float64((100-rc.BEPool)*int(info.CapacityTotal)/100) - gaIn - gaOut,
			UnderPressure: false,
		}
		if r.info.DisksStatus[dev].BE != nil {
			r.info.DisksStatus[dev].BE.Num = beNum
		} else {
			r.info.DisksStatus[dev].BE = &utils.IOPoolStatus{
				Num: beNum,
			}
		}
	}
	h.EC.PrintCacheInfo()
}

// parseQuantity parses the specified value (if provided) otherwise returns 0 value
func (h *Handle) parseQuantity(value string) resource.Quantity {
	if len(value) == 0 {
		return resource.MustParse("0")
	}
	return resource.MustParse(value)
}

// assume the IoIContext lock is acquired
func (h *Handle) AddCacheNodeInfo(nodeName string, rc v1.ResourceConfigSpec) error {
	nodeInfo := &NodeInfo{
		DisksStatus: make(map[string]*DiskInfo),
	}
	// get device usage
	usage, err := h.getDeviceUsage(nodeName)
	if err != nil {
		return err
	}
	for dev, info := range rc.Devices {
		diskInfo := &DiskInfo{
			GA: &utils.IOPoolStatus{},
			BE: &utils.IOPoolStatus{},
		}
		if info.Type == v1.DefaultDevice {
			nodeInfo.EmptyDirDevice = dev
		}
		diskInfo.DiskName = info.Name
		diskInfo.MountPath = info.MountPath
		diskInfo.BEDefaultRead = info.DefaultIn
		diskInfo.BEDefaultWrite = info.DefaultOut
		diskInfo.ReadRatio = utils.ParseMappingRatio(info.ReadRatio)
		diskInfo.WriteRatio = utils.ParseMappingRatio(info.WriteRatio)
		diskInfo.BE.Total = float64(rc.BEPool * int(info.CapacityTotal) / 100)
		diskInfo.BE.In = float64(rc.BEPool * int(info.CapacityIn) / 100)
		diskInfo.BE.Out = float64(rc.BEPool * int(info.CapacityOut) / 100)
		qty := h.parseQuantity(info.DiskSize)
		diskInfo.Capacity = qty.Value()
		if u, ok := usage[dev]; ok {
			diskInfo.RequestedDiskSpace = u
		} else {
			diskInfo.RequestedDiskSpace = 0
		}
		nodeInfo.DisksStatus[dev] = diskInfo
	}
	h.Lock()
	defer h.Unlock()
	h.EC.SetExtendedResource(nodeName, &Resource{
		nodeName: nodeName,
		info:     nodeInfo,
		ch:       h})
	h.EC.PrintCacheInfo()
	return nil
}

func (h *Handle) UpdateDeviceNodeInfo(node string, old *v1.ResourceConfigSpec, new *v1.ResourceConfigSpec) (map[string]v1.Device, map[string]v1.Device, error) {
	adds := make(map[string]v1.Device)
	dels := make(map[string]v1.Device)
	// compare changes
	for dev, info := range new.Devices {
		if _, ok := old.Devices[dev]; !ok {
			adds[dev] = info
		}
	}
	for dev, info := range old.Devices {
		if _, ok := new.Devices[dev]; !ok {
			dels[dev] = info
		}
	}
	// update local cache
	rs := h.EC.GetExtendedResource(node)
	if rs == nil {
		return nil, nil, fmt.Errorf("node %v not registered in cache", node)

	}
	r, ok := rs.(*Resource)
	if !ok {
		return nil, nil, fmt.Errorf("incorrect resource type cached")
	}
	h.Lock()
	defer h.Unlock()
	for dev, info := range adds {
		r.info.DisksStatus[dev] = &DiskInfo{
			DiskName:       info.Name,
			MountPath:      info.MountPath,
			BEDefaultRead:  info.DefaultIn,
			BEDefaultWrite: info.DefaultOut,
			ReadRatio:      utils.ParseMappingRatio(info.ReadRatio),
			WriteRatio:     utils.ParseMappingRatio(info.WriteRatio),
		}
		if info.Type == v1.DefaultDevice {
			r.info.EmptyDirDevice = dev
		}
		r.info.DisksStatus[dev].GA = &utils.IOPoolStatus{
			In:            float64((100 - new.BEPool) * int(info.CapacityIn) / 100),
			Out:           float64((100 - new.BEPool) * int(info.CapacityOut) / 100),
			Total:         float64((100 - new.BEPool) * int(info.CapacityTotal) / 100),
			UnderPressure: false,
		}
		r.info.DisksStatus[dev].BE = &utils.IOPoolStatus{
			In:            float64(new.BEPool * int(info.CapacityIn) / 100),
			Out:           float64(new.BEPool * int(info.CapacityOut) / 100),
			Total:         float64(new.BEPool * int(info.CapacityTotal) / 100),
			UnderPressure: false,
			Num:           0,
		}
		qty := h.parseQuantity(info.DiskSize)
		r.info.DisksStatus[dev].Capacity = qty.Value()
	}
	for dev := range dels {
		delete(r.info.DisksStatus, dev)
	}
	h.EC.PrintCacheInfo()

	return adds, dels, nil
}

func (h *Handle) UpdatePVC(pvc *corev1.PersistentVolumeClaim, nodename string, devId string, storage int64, add bool) error {
	rs := h.EC.GetExtendedResource(nodename)
	if rs == nil {
		klog.Errorf("node %v not registered in cache", nodename)
		return fmt.Errorf("node %v not registered in cache", nodename)
	}
	r, ok := rs.(*Resource)
	if !ok {
		klog.Error("incorrect resource cached")
		return fmt.Errorf("incorrect resource cached")
	}
	diskStatus, exist := r.info.DisksStatus[devId]
	if !exist {
		return fmt.Errorf("node %v doesn't contain device %v info", nodename, devId)
	}
	if !add {
		// delete cache and pvc in configmap
		node, ok := pvc.Annotations[volume.AnnSelectedNode]
		if ok {
			err := common.RemovePVCsFromVolDiskCm(h.client, pvc, node)
			if err != nil {
				klog.Errorf("fail to remove PVC %v from VolDisk configmap: %v", pvc.Name, err)
			}
			for k := range ioiresource.IoiContext.ClaimDevMap {
				if k == pvc.Name+"."+pvc.Namespace {
					delete(ioiresource.IoiContext.ClaimDevMap, k)
				}
			}
		}
		storage *= -1
	}
	diskStatus.RequestedDiskSpace += storage
	h.EC.PrintCacheInfo()
	return nil
}

// assume the IoIContext lock is acquired
func (h *Handle) getDeviceUsage(nodename string) (map[string]int64, error) {
	requests, err := h.getBoundPVCsStorageRequest(nodename)
	if err != nil {
		return nil, err
	}
	used := make(map[string]int64) // key -> deviceID, value: accumulated storage usage
	m, err := common.GetVolumeDeviceMap(h.client, nodename)
	if kubeerr.IsNotFound(err) {
		if len(requests) > 0 {
			return used, fmt.Errorf("internal error: volume-device-map is not found for nodename %s but pvcs %v exist", nodename, requests)
		} else {
			return used, nil
		}
	} else if err != nil {
		return used, fmt.Errorf("failed to get ConfigMap volume-device-map: nodename %s, err: %v ", nodename, err)
	}

	for k, raw := range m.Data {
		if k == utils.DeviceInUse {
			continue
		}
		v := raw
		strs := strings.Split(raw, ":")
		if len(strs) == 2 {
			v = strs[0]
		}
		req, ok := requests[k] // the specified request in pvc
		if !ok {
			req = 0
		}
		_, ok = used[v]
		if ok {
			used[v] += req
		} else {
			used[v] = req
		}
		ioiresource.IoiContext.ClaimDevMap[k] = &ioiresource.StorageInfo{
			DevID:            v,
			RequestedStorage: req,
			NodeName:         nodename,
		}
	}
	return used, nil
}

func (h *Handle) getBoundPVCsStorageRequest(nodename string) (map[string]int64, error) {
	if len(nodename) == 0 {
		return nil, fmt.Errorf("node name cannot be empty")
	}
	requests := make(map[string]int64) // key -> pvc_name.pvc_namespace, value -> storage request
	// pvcs, err := h.pvcLister.List(labels.Everything())
	pvcs, err := h.client.CoreV1().PersistentVolumeClaims("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return requests, fmt.Errorf("get PVC error: %s", err)
	}
	for _, pvc := range pvcs.Items {
		node, ok := pvc.Annotations[volume.AnnSelectedNode]
		phase := pvc.Status.Phase
		provisioner, ok1 := pvc.Annotations[volume.AnnStorageProvisioner]
		if ok && ok1 && node == nodename && phase == corev1.ClaimBound && provisioner == utils.StorageProvisioner {
			quantity, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			if !ok {
				// No capacity to check for. Should not happen.
				klog.Errorf("The PVC storage request is not specified.")
			} else {
				requests[fmt.Sprintf("%s.%s", pvc.Name, pvc.Namespace)] = quantity.Value()
			}
		}
	}
	return requests, nil
}

func (h *Handle) DeleteCacheNodeInfo(nodeName string) error {
	h.Lock()
	defer h.Unlock()
	h.EC.DeleteExtendedResource(nodeName)
	return nil
}

func (h *Handle) UpdateCacheNodeStatus(n string, nodeIoBw *v1.NodeIOStatusStatus) error {
	klog.V(utils.DBG).Info("Disk UpdateCacheNodeStatus from CR")
	for typ, bw := range nodeIoBw.AllocatableBandwidth {
		if len(bw.DeviceIOStatus) == 0 || typ != v1.BlockIO {
			continue
		}
		rs := h.EC.GetExtendedResource(n)
		if rs == nil {
			return fmt.Errorf("node not registered in cache")
		}
		r, ok := rs.(*Resource)
		if !ok {
			return fmt.Errorf("incorrect resource cached")
		}
		v, err := ioiresource.IoiContext.GetReservedPods(n)
		h.Lock()
		defer h.Unlock()
		if err == nil && nodeIoBw.ObservedGeneration == v.Generation {
			for dev, info := range r.info.DisksStatus {
				ga, be, update := h.calculateDiskStatus(bw.DeviceIOStatus, dev)
				if update {
					info.GA = ga
					if be != nil {
						info.BE.UnderPressure = be.UnderPressure
					}
				}
			}
		}
	}
	h.EC.PrintCacheInfo()
	return nil
}

// return GA and BE pool status
func (h *Handle) calculateDiskStatus(in map[string]v1.IOAllocatableBandwidth, dev string) (*utils.IOPoolStatus, *utils.IOPoolStatus, bool) {
	ga := &utils.IOPoolStatus{}
	be := &utils.IOPoolStatus{}
	info, ok := in[dev]
	if ok {
		v, exist := info.IOPoolStatus[v1.WorkloadGA]
		if exist {
			if v.In < 0 {
				return nil, nil, false
			}
			ga.In = v.In
			ga.Out = v.Out
			ga.Total = v.Total
			ga.UnderPressure = common.IsUnderPressure(v.Pressure)
		}
		r, exist := info.IOPoolStatus[v1.WorkloadBE]
		if exist {
			be.UnderPressure = common.IsUnderPressure(r.Pressure)
		}
	}
	return ga, be, true
}

func (h *Handle) NodeHasIoiLabel(node *corev1.Node) bool {
	v, ok := node.Labels["ioisolation"]
	if ok && (v == "all" || v == "disk") {
		return true
	} else if ok {
		resources := strings.Split(v, "-")
		for _, r := range resources {
			if r == "disk" {
				return true
			}
		}
	}
	return false
}

// New Resource Handle
func New(bpStrategy string) ioiresource.Handle {
	// todo: to support more bin packers
	if len(bpStrategy) == 0 || bpStrategy != common.DefaultBinPack {
		klog.Warningf("unknown bin packing strategy %v, use default packer", bpStrategy)
	}
	return &Handle{
		packer: NewFirstFitPacker(),
	}
}

func (h *Handle) getVolumeRequestFromAnno(pvcMapping map[string]string, anno utils.PodAnnoThroughput, info *NodeInfo) map[string]*pb.IOResourceRequest {
	requests := map[string]*pb.IOResourceRequest{}
	if len(pvcMapping) == 0 && info.DisksStatus != nil { // no volume, reserve as BE in default disk
		emptyDirInfo, ok := info.DisksStatus[info.EmptyDirDevice]
		if ok {
			addPodRequest(requests, h.parseBEIOAnnotations(info.EmptyDirDevice, emptyDirInfo))
		}
		return requests
	}
	for vol, dev := range pvcMapping {
		if req, ok := anno[vol]; ok { // GA/BT
			request := &pb.IOResourceRequest{
				IoType: pb.IOType_DiskIO,
				DevRequest: &pb.DeviceRequirement{
					DevName:    dev,
					Request:    &pb.Quantity{},
					Limit:      &pb.Quantity{},
					RawRequest: &pb.Quantity{},
					RawLimit:   &pb.Quantity{},
				},
			}
			if _, ok := info.DisksStatus[dev]; !ok {
				klog.Errorf("device %v not registed", dev)
				continue
			}
			h.getDeviceRequest(request, info.DisksStatus[dev], &req)
			addPodRequest(requests, map[string]*pb.IOResourceRequest{dev: request})
		} else { // BE
			klog.Info("BE:", req)
			request := &pb.IOResourceRequest{
				IoType:       pb.IOType_DiskIO,
				WorkloadType: utils.BeIndex,
				DevRequest: &pb.DeviceRequirement{
					DevName: dev,
				},
			}
			addPodRequest(requests, map[string]*pb.IOResourceRequest{dev: request})
		}
	}
	return requests
}

func (h *Handle) getDeviceRequest(request *pb.IOResourceRequest, devInfo *DiskInfo, anno *utils.VolumeAnnoThroughput) {
	rbpsbw, rbpsbs, rrerr := utils.ParseDiskRequestBandwidth(anno.Requests.Rbps)
	if rrerr == nil {
		bwValue, _ := rbpsbw.AsInt64()
		bs, ratio := utils.FindMappingRatio(rbpsbs, devInfo.ReadRatio)
		bsValue, _ := bs.AsInt64()
		request.DevRequest.RawRequest.Inbps = float64(bwValue) / utils.MiB
		request.DevRequest.RawRequest.Iniops = float64(bwValue / bsValue)
		request.DevRequest.Request.Inbps = float64(bwValue) / utils.MiB * ratio
	}

	wbpsbw, wbpsbs, wrerr := utils.ParseDiskRequestBandwidth(anno.Requests.Wbps)
	if wrerr == nil {
		bwValue, _ := wbpsbw.AsInt64()
		bs, ratio := utils.FindMappingRatio(wbpsbs, devInfo.WriteRatio)
		bsValue, _ := bs.AsInt64()
		request.DevRequest.RawRequest.Outbps = float64(bwValue) / utils.MiB
		request.DevRequest.RawRequest.Outiops = float64(bwValue / bsValue)
		request.DevRequest.Request.Outbps = float64(bwValue) / utils.MiB * ratio
	}

	rbpslimbw, rbpslimbs, rlerr := utils.ParseDiskRequestBandwidth(anno.Limits.Rbps)
	if rlerr == nil {
		bwValue, _ := rbpslimbw.AsInt64()
		bs, ratio := utils.FindMappingRatio(rbpslimbs, devInfo.ReadRatio)
		bsValue, _ := bs.AsInt64()
		request.DevRequest.RawLimit.Inbps = float64(bwValue) / utils.MiB
		request.DevRequest.RawLimit.Iniops = float64(bwValue / bsValue)
		request.DevRequest.Limit.Inbps = float64(bwValue) / utils.MiB * ratio
	}
	wbpslimbw, wbpslimbs, wlerr := utils.ParseDiskRequestBandwidth(anno.Limits.Wbps)
	if wlerr == nil {
		bwValue, _ := wbpslimbw.AsInt64()
		bs, ratio := utils.FindMappingRatio(wbpslimbs, devInfo.WriteRatio)
		bsValue, _ := bs.AsInt64()
		request.DevRequest.RawLimit.Outbps = float64(bwValue) / utils.MiB
		request.DevRequest.RawLimit.Outiops = float64(bwValue / bsValue)
		request.DevRequest.Limit.Outbps = float64(bwValue) / utils.MiB * ratio
	}
	if rrerr != nil && rlerr != nil && wrerr != nil && wlerr != nil {
		request.WorkloadType = utils.BeIndex
	} else {
		request.WorkloadType = utils.GaIndex
		defaultBs := resource.MustParse("4k")
		defaultBsValue := defaultBs.Value()
		_, rratio := utils.FindMappingRatio(defaultBs, devInfo.ReadRatio)
		_, wratio := utils.FindMappingRatio(defaultBs, devInfo.WriteRatio)
		if rlerr != nil {
			request.DevRequest.Limit.Inbps = utils.MAX_BT_LIMIT
			request.DevRequest.RawLimit.Inbps = utils.MAX_BT_LIMIT / rratio
			request.DevRequest.RawLimit.Iniops = utils.MAX_BT_LIMIT * utils.MiB / rratio / float64(defaultBsValue)
		}
		if wlerr != nil {
			request.DevRequest.Limit.Outbps = utils.MAX_BT_LIMIT / wratio
			request.DevRequest.RawLimit.Outbps = utils.MAX_BT_LIMIT / wratio
			request.DevRequest.RawLimit.Outiops = utils.MAX_BT_LIMIT * utils.MiB / wratio / float64(defaultBsValue)
		}
		if request.DevRequest.Request.Inbps < devInfo.BEDefaultRead {
			request.DevRequest.Request.Inbps = devInfo.BEDefaultRead
			request.DevRequest.RawRequest.Inbps = devInfo.BEDefaultRead / rratio
			request.DevRequest.RawRequest.Iniops = devInfo.BEDefaultRead / rratio * utils.MiB / float64(defaultBsValue)
		}
		if request.DevRequest.Request.Outbps < devInfo.BEDefaultWrite {
			request.DevRequest.Request.Outbps = devInfo.BEDefaultWrite
			request.DevRequest.RawRequest.Outbps = devInfo.BEDefaultRead / wratio
			request.DevRequest.RawRequest.Outiops = devInfo.BEDefaultRead / wratio * utils.MiB / float64(defaultBsValue)
		}
		if request.DevRequest.Limit.Inbps < devInfo.BEDefaultRead {
			request.DevRequest.Limit.Inbps = devInfo.BEDefaultRead
			request.DevRequest.RawLimit.Inbps = devInfo.BEDefaultRead / rratio
			request.DevRequest.RawLimit.Iniops = devInfo.BEDefaultRead / rratio * utils.MiB / float64(defaultBsValue)
		}
		if request.DevRequest.Limit.Outbps < devInfo.BEDefaultWrite {
			request.DevRequest.Limit.Outbps = devInfo.BEDefaultWrite
			request.DevRequest.RawLimit.Outbps = devInfo.BEDefaultRead / wratio
			request.DevRequest.RawLimit.Outiops = devInfo.BEDefaultRead / wratio * utils.MiB / float64(defaultBsValue)
		}
	}
}

func (h *Handle) parseBEIOAnnotations(devId string, devInfo *DiskInfo) map[string]*pb.IOResourceRequest {
	request := &pb.IOResourceRequest{
		IoType:       pb.IOType_DiskIO,
		WorkloadType: utils.BeIndex,
		DevRequest: &pb.DeviceRequirement{
			DevName: devId,
		},
	}
	return map[string]*pb.IOResourceRequest{devId: request}
}

func (h *Handle) parsePVIOAnnotations(anno utils.VolumeAnnoThroughput, devId string, devInfo *DiskInfo) map[string]*pb.IOResourceRequest {
	request := &pb.IOResourceRequest{
		IoType: pb.IOType_DiskIO,
		DevRequest: &pb.DeviceRequirement{
			DevName:    devId,
			Request:    &pb.Quantity{},
			Limit:      &pb.Quantity{},
			RawRequest: &pb.Quantity{},
			RawLimit:   &pb.Quantity{},
		},
	}
	h.getDeviceRequest(request, devInfo, &anno)
	klog.Info(request.String())
	return map[string]*pb.IOResourceRequest{devId: request}
}
