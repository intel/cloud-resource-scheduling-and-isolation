/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
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

func (h *Handle) Name() string {
	return "RDTHandle"
}

func (h *Handle) Run(c ioiresource.ExtendedCache, f informers.SharedInformerFactory, cli kubernetes.Interface) error {
	h.EC = c
	h.client = cli
	return nil
}

func (h *Handle) CanAdmitPod(nodeName string, rawRequest interface{}) (bool, interface{}, error) {
	requests, ok := rawRequest.(*RequestInfo)
	if !ok {
		return false, nil, fmt.Errorf("unable to convert state into %v's pod request format", h.Name())
	}
	rs := h.EC.GetExtendedResource(nodeName)
	if rs == nil {
		klog.Errorf("node %v not registered in cache", nodeName)
		return false, nil, fmt.Errorf("node %v not registered in cache", nodeName)
	}
	r, ok := rs.(*Resource)
	if !ok || r.info == nil {
		klog.Error("incorrect resource cached")
		return false, nil, fmt.Errorf("incorrect resource cached")
	}
	if requests.RDTClass == "" {
		return true, r.info.RDTClassStatus, nil
	}
	if _, ok := r.info.RDTClassStatus[requests.RDTClass]; ok {
		return true, r.info.RDTClassStatus, nil
	}
	return false, nil, fmt.Errorf("there is no %v rdt class on %v node", requests.RDTClass, nodeName)
}

func (h *Handle) NodePressureRatio(request interface{}, classStatus interface{}) (float64, error) {
	r, ok := request.(*RequestInfo)
	if !ok {
		return 0, fmt.Errorf("unable to convert state into %v's pod request format", h.Name())
	}
	if r.RDTClass == "" {
		return 0, nil
	}
	s, ok := classStatus.(map[string]float64)
	if !ok {
		return 0, fmt.Errorf("unable to convert state into %v's node resource class format", h.Name())
	}
	_, ok = s[r.RDTClass]
	if !ok {
		return 0, fmt.Errorf("unable to get class %v's status", r.RDTClass)
	}
	return s[r.RDTClass] / float64(100), nil
}

func (h *Handle) GetRequestByAnnotation(pod *corev1.Pod, node string, init bool) (map[string]*pb.IOResourceRequest, interface{}, error) {
	v, ok := pod.Annotations[utils.RDTConfigAnno]
	if ok {
		return nil, v, nil
	} else {
		return nil, "", nil
	}
}

func (h *Handle) CriticalPod(annotations map[string]string) bool {
	if _, ok := annotations[utils.RDTConfigAnno]; ok {
		return true
	}
	return false
}

func (h *Handle) InitBENum(nodeName string, rc v1.ResourceConfigSpec) {
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
	for class := range rc.Devices {
		r.info.RDTClassStatus[class] = 0
	}
	h.EC.PrintCacheInfo()
}

func (h *Handle) AddCacheNodeInfo(nodeName string, rc v1.ResourceConfigSpec) error {
	nodeInfo := &NodeInfo{
		RDTClassStatus: make(map[string]float64),
	}
	for class := range rc.Devices {
		nodeInfo.RDTClassStatus[class] = 0
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
	klog.V(utils.DBG).Info("RDT UpdateCacheNodeStatus from CR")

	for typ, bw := range nodeIoBw.AllocatableBandwidth {
		if len(bw.DeviceIOStatus) == 0 || typ != v1.RDT {
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

		for class := range r.info.RDTClassStatus {
			_, ok := bw.DeviceIOStatus[class]
			if !ok {
				return fmt.Errorf("node %v DeviceBwInfo do not exist", n)
			}
			_, ok = bw.DeviceIOStatus[class].IOPoolStatus[v1.WorkloadGA]
			if !ok {
				return fmt.Errorf("node %v rdt class %v do not exist", n, class)
			}
			if bw.DeviceIOStatus[class].IOPoolStatus[v1.WorkloadGA].Pressure >= 0 {
				r.info.RDTClassStatus[class] = bw.DeviceIOStatus[class].IOPoolStatus[v1.WorkloadGA].Pressure
			}
		}
	}
	h.EC.PrintCacheInfo()
	return nil
}

func (h *Handle) UpdatePVC(pvc *corev1.PersistentVolumeClaim, nodename string, devId string, storage int64, add bool) error {
	return nil
}

func (h *Handle) NodeHasIoiLabel(node *corev1.Node) bool {
	v, ok := node.Labels["ioisolation"]
	if ok && (v == "all" || v == "rdt") {
		return true
	} else if ok {
		resources := strings.Split(v, "-")
		for _, r := range resources {
			if r == "rdt" {
				return true
			}
		}
	}
	return false
}

func New() ioiresource.Handle {
	return &Handle{}
}
