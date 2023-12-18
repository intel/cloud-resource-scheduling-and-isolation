/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdtquantity

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

type Handle struct {
	ioiresource.HandleBase
	client kubernetes.Interface
	sync.RWMutex
}

// ioiresource.Handle interface
func (h *Handle) Name() string {
	return "RDTQuantityHandle"
}

func (h *Handle) Run(c ioiresource.ExtendedCache, f informers.SharedInformerFactory, cli kubernetes.Interface) error {
	h.EC = c
	h.client = cli
	return nil
}

func (h *Handle) CanAdmitPod(nodeName string, rawRequest interface{}) (bool, interface{}, error) {
	klog.V(utils.DBG).Infof("CanAdmitPod rawRequest: %+v", rawRequest)
	requestInfo, ok := rawRequest.(*pb.IOResourceRequest)
	if !ok {
		return false, nil, fmt.Errorf("unable to convert state into %v's pod request format", h.Name())
	}
	rs := h.EC.GetExtendedResource(nodeName)
	if rs == nil {
		klog.Errorf("node %v not registered in cache", nodeName)
		return false, nil, fmt.Errorf("node %v not registered in cache", nodeName)
	}
	ps, ok := rs.(*Resource)
	if !ok || ps.info == nil {
		klog.Error("incorrect resource cached")
		return false, nil, fmt.Errorf("incorrect resource cached")
	}
	classResurce, exist := ps.info.classes[requestInfo.DevRequest.DevName]
	if exist && requestInfo.DevRequest.DevName == BE {
		return true, ps.info, nil
	} else if exist && requestInfo.DevRequest.Limit.Inbps == classResurce.Limits.l3 && requestInfo.DevRequest.Request.Inbps == classResurce.Requests.l3 &&
		requestInfo.DevRequest.RawLimit.Inbps == classResurce.Limits.mb && requestInfo.DevRequest.RawRequest.Inbps == classResurce.Requests.mb {
		return true, ps.info, nil
	} else if !exist && ps.info.classes[BE].Requests.l3 >= requestInfo.DevRequest.Request.Inbps+ps.info.BEL3CapacityBaseline &&
		ps.info.classes[BE].Requests.mb >= requestInfo.DevRequest.RawRequest.Inbps+ps.info.BEMBCapacityBaseline {
		return true, ps.info, nil
	} else {
		return false, nil, fmt.Errorf("pod request does not match the node's resource")
	}
}

func (h *Handle) NodePressureRatio(request interface{}, nodeInfo interface{}) (float64, error) {
	requestInfo, ok := request.(*pb.IOResourceRequest)
	if !ok {
		return 0, fmt.Errorf("unable to convert state into %v's pod request format", h.Name())
	}
	n, ok := nodeInfo.(*NodeInfo)
	if !ok {
		return 0, fmt.Errorf("unable to convert state into %v's node resource info format", h.Name())
	}
	if requestInfo.DevRequest.DevName == BE {
		return n.BEScore, nil
	}
	score := 0.0
	score += ((n.classes[BE].Requests.l3-n.BEL3CapacityBaseline)/(n.L3CacheCapacity-n.BEL3CapacityBaseline) + (n.classes[BE].Requests.mb-n.BEMBCapacityBaseline)/(n.MBCapacity-n.BEMBCapacityBaseline)) / 2 * 100
	return score, nil
}

func (h *Handle) GetRequestByAnnotation(pod *corev1.Pod, node string, init bool) (map[string]*pb.IOResourceRequest, interface{}, error) {
	requests := map[string]*pb.IOResourceRequest{}
	anno := RDTAnnotationResource{}
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
		return nil, nil, fmt.Errorf("rdt quantity: node %v cache info is not registered", node)
	}

	v, ok := pod.Annotations[utils.RDTQuantityConfigAnno]
	if ok {
		if err := json.Unmarshal([]byte(v), &anno); err != nil {
			return nil, nil, fmt.Errorf("cannot parse %v to rdt quantity requests/limits: %v", v, err)
		}
	} else {
		return nil, nil, fmt.Errorf("pod %v does not have rdt quantity annotation", pod.Name)
	}
	if anno.Class == "" {
		anno.Class = pod.Name
	}
	if anno.Requests.L3 == "" && anno.Requests.Mb == "" && anno.Limits.L3 == "" && anno.Limits.Mb == "" {
		anno.Class = "BE"
	} else if anno.Requests.L3 == "" || anno.Requests.Mb == "" {
		return nil, nil, fmt.Errorf("pod %v does not have complete rdt quantity annotation", pod.Name)
	} else {
		if anno.Limits.L3 == "" {
			anno.Limits.L3 = anno.Requests.L3
		}
		if anno.Limits.Mb == "" {
			anno.Limits.Mb = anno.Requests.Mb
		}
	}

	var err error
	requests[anno.Class] = &pb.IOResourceRequest{
		IoType: pb.IOType_RDTQuantity,
		WorkloadType: utils.GaIndex,
		DevRequest: &pb.DeviceRequirement{
			DevName:    anno.Class,
			Request:    &pb.Quantity{},
			Limit:      &pb.Quantity{},
			RawRequest: &pb.Quantity{},
			RawLimit:   &pb.Quantity{},
		},
	}
	requests[anno.Class].DevRequest.Request.Inbps, err = anno.Requests.L3.Parse()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse %v to rdt quantity request/limit: %v", anno.Requests.L3, err)
	}
	requests[anno.Class].DevRequest.Limit.Inbps, err = anno.Limits.L3.Parse()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse %v to rdt quantity request/limit: %v", anno.Limits.L3, err)
	}
	requests[anno.Class].DevRequest.RawRequest.Inbps, err = anno.Requests.Mb.Parse()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse %v to rdt quantity request/limit: %v", anno.Requests.Mb, err)
	}
	requests[anno.Class].DevRequest.RawLimit.Inbps, err = anno.Limits.Mb.Parse()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse %v to rdt quantity request/limit: %v", anno.Limits.Mb, err)
	}
	klog.V(utils.DBG).Infof("GetRequestByAnnotation requests: %+v", requests)
	return requests, nil, nil

}

func (h *Handle) CriticalPod(annotations map[string]string) bool {
	anno := RDTAnnotationResource{}
	if v, ok := annotations[utils.RDTQuantityConfigAnno]; ok {
		if err := json.Unmarshal([]byte(v), &anno); err != nil {
			return false
		}
		return true
	}
	return false
}

func (h *Handle) InitBENum(nodeName string, rc v1.ResourceConfigSpec) {
}

func (h *Handle) GetBENum(nodeName string) int {
	return 0
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

	podRDTQuantityRequests := make(map[string]map[string]*pb.IOResourceRequest)
	for puid, l := range pl.PodList {
		for _, rq := range l.Request {
			if rq.IoType == pb.IOType_RDTQuantity {
				podRDTQuantityRequests[puid] = make(map[string]*pb.IOResourceRequest)
				podRDTQuantityRequests[puid][rq.DevRequest.DevName] = rq
			}
		}
	}
	for puid, rqs := range podRDTQuantityRequests {
		for class, rq := range rqs {
			classResource, exist := r.info.classes[class]
			if exist && class == BE {
				r.info.classes[class].PodList = append(classResource.PodList, puid)
			} else if exist && rq.DevRequest.Limit.Inbps == classResource.Limits.l3 && rq.DevRequest.Request.Inbps == classResource.Requests.l3 &&
				rq.DevRequest.RawLimit.Inbps == classResource.Limits.mb && rq.DevRequest.RawRequest.Inbps == classResource.Requests.mb {
				r.info.classes[class].PodList = append(classResource.PodList, puid)
			} else {
				r.info.classes[class] = &classInfo{
					Requests: rdtResource{
						l3: rq.DevRequest.Request.Inbps,
						mb: rq.DevRequest.RawRequest.Inbps,
					},
					Limits: rdtResource{
						l3: rq.DevRequest.Limit.Inbps,
						mb: rq.DevRequest.RawLimit.Inbps,
					},
					PodList: []string{puid},
				}
				r.info.classes[BE].Limits.l3 -= rq.DevRequest.Request.Inbps
				r.info.classes[BE].Requests.l3 -= rq.DevRequest.Request.Inbps
				r.info.classes[BE].Limits.mb -= rq.DevRequest.RawRequest.Inbps
				r.info.classes[BE].Requests.mb -= rq.DevRequest.RawRequest.Inbps
			}
		}

	}

	h.EC.PrintCacheInfo()
}

func (h *Handle) AddCacheNodeInfo(nodeName string, rc v1.ResourceConfigSpec) error {
	nodeInfo := &NodeInfo{
		BEScore:              100,
		L3CacheCapacity:      rc.Devices[L3].CapacityIn,
		MBCapacity:           rc.Devices[MB].CapacityIn,
		BEL3CapacityBaseline: rc.Devices[L3].DefaultIn,
		BEMBCapacityBaseline: rc.Devices[MB].DefaultIn,
		classes:              make(map[string]*classInfo),
	}
	nodeInfo.classes[BE] = &classInfo{
		Requests: rdtResource{
			l3: rc.Devices[L3].CapacityIn,
			mb: rc.Devices[MB].CapacityIn,
		},
		Limits: rdtResource{
			l3: rc.Devices[L3].CapacityIn,
			mb: rc.Devices[MB].CapacityIn,
		},
		PodList: []string{},
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

func (h *Handle) UpdatePVC(pvc *corev1.PersistentVolumeClaim, nodename string, devId string, storage int64, add bool) error {
	return nil
}

func (h *Handle) UpdateDeviceNodeInfo(string, *v1.ResourceConfigSpec, *v1.ResourceConfigSpec) (map[string]v1.Device, map[string]v1.Device, error) {
	return nil, nil, nil
}

func (h *Handle) DeleteCacheNodeInfo(nodeName string) error {
	h.Lock()
	defer h.Unlock()
	h.EC.DeleteExtendedResource(nodeName)
	return nil
}

func (h *Handle) UpdateCacheNodeStatus(n string, nodeIoBw *v1.NodeIOStatusStatus) error {
	klog.V(utils.DBG).Info("RDTQuantity UpdateCacheNodeStatus from CR")
	for typ, bw := range nodeIoBw.AllocatableBandwidth {
		if len(bw.DeviceIOStatus) == 0 || typ != v1.RDTQuantity {
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
		h.Lock()
		defer h.Unlock()
		klog.V(utils.INF).Info(bw)

		_, ok = bw.DeviceIOStatus[BE]
		if !ok {
			return fmt.Errorf("node %v DeviceBwInfo do not exist", n)
		}
		_, ok = bw.DeviceIOStatus[BE].IOPoolStatus[v1.WorkloadGA]
		if !ok {
			return fmt.Errorf("node %v rdt class %v do not exist", n, BE)
		}
		r.info.BEScore = bw.DeviceIOStatus[BE].IOPoolStatus[v1.WorkloadGA].Pressure
	}
	h.EC.PrintCacheInfo()
	return nil
}

func (h *Handle) NodeHasIoiLabel(node *corev1.Node) bool {
	v, ok := node.Labels["ioisolation"]
	if ok && (v == "all" || v == "rdtQuantity") {
		return true
	} else if ok {
		resources := strings.Split(v, "-")
		for _, r := range resources {
			if r == "rdtQuantity" {
				return true
			}
		}
	}
	return false
}

func New() ioiresource.Handle {
	return &Handle{}
}
