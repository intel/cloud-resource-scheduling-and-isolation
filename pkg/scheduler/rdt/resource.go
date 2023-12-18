/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

type RequestInfo struct {
	RDTClass string
}

type NodeInfo struct {
	RDTClassStatus map[string]float64 // class name: status
}

type Resource struct {
	nodeName string
	info     *NodeInfo
	ch       resource.CacheHandle
	sync.RWMutex
}

func (ps *Resource) Name() string {
	return "RDT"
}

func (ps *Resource) AddPod(pod *v1.Pod, rawRequest interface{}, client kubernetes.Interface) ([]*pb.IOResourceRequest, bool, error) {
	ps.ch.PrintCacheInfo()
	ioreq := &pb.IOResourceRequest{
		IoType:       pb.IOType_RDT,
		DevRequest:   &pb.DeviceRequirement{},
	}
	requestsArray := []*pb.IOResourceRequest{ioreq}
	return requestsArray, true, nil
}

func (ps *Resource) RemovePod(pod *v1.Pod, client kubernetes.Interface) error {
	return nil
}

func (ps *Resource) AdmitPod(pod *v1.Pod) (interface{}, error) {
	_, rdtClass, err := ps.ch.GetRequestByAnnotation(pod, ps.nodeName, false)
	if err != nil {
		return nil, err
	}
	value, ok := rdtClass.(string)
	if !ok {
		return nil, fmt.Errorf("cannot get pod %v rdt class", pod.Name)
	}
	ret := &RequestInfo{
		RDTClass: value,
	}
	return ret, nil
}

func (ps *Resource) Clone() resource.ExtendedResource {
	// todo rdt deepcopy
	return ps
}

func (ps *Resource) PrintInfo() {
	for class, status := range ps.info.RDTClassStatus {
		klog.V(utils.DBG).Info("***class: ", class, " ***")
		klog.V(utils.DBG).Info("status: ", status)
	}
}
