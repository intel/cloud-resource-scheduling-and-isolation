/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package networkio

import (
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

type NodeInfo struct {
	NICsStatus map[string]*NICInfo // mac addr : NIC info
	MasterNic  string
}

type NICInfo struct {
	Name         string
	DefaultSpeed float64
	GA           *utils.IOPoolStatus
	BE           *utils.IOPoolStatus
}

func NewNICInfo() *NICInfo {
	nicInfo := &NICInfo{
		GA: &utils.IOPoolStatus{},
		BE: &utils.IOPoolStatus{},
	}
	return nicInfo
}

type Resource struct {
	nodeName string
	info     *NodeInfo
	ch       resource.CacheHandle
	sync.RWMutex
}

func (ps *Resource) Name() string {
	return "NetworkIO"
}

func (ps *Resource) AddPod(pod *v1.Pod, rawRequest interface{}, client kubernetes.Interface) ([]*aggregator.IOResourceRequest, bool, error) {
	request, ok := rawRequest.(map[string]*aggregator.IOResourceRequest)
	if !ok {
		return nil, false, fmt.Errorf("unable to convert state into %v's pod request format", ps.Name())
	}
	masterNic := ps.info.MasterNic
	nicStatus, exist := ps.info.NICsStatus[masterNic]
	if !exist {
		return nil, true, fmt.Errorf("node %v doesn't contain master nic info: %v", ps.nodeName, masterNic)
	}
	ioreq, ok := request[masterNic]
	if !ok {
		return nil, true, fmt.Errorf("networkIO: device %v's request not found in pod network io request", masterNic)
	}
	klog.V(utils.DBG).Info("ioreq minus in cache:", ioreq.String())
	if ioreq.WorkloadType == utils.GaIndex {
		nicStatus.GA.In -= ioreq.DevRequest.Request.Inbps
		nicStatus.GA.Out -= ioreq.DevRequest.Request.Outbps
		nicStatus.GA.Total -= (ioreq.DevRequest.Request.Inbps + ioreq.DevRequest.Request.Outbps)
	} else if ioreq.WorkloadType == utils.BeIndex {
		nicStatus.BE.Num += 1
	}
	ps.ch.PrintCacheInfo()
	requestsArray := []*aggregator.IOResourceRequest{ioreq}
	return requestsArray, true, nil
}

func (ps *Resource) RemovePod(pod *v1.Pod, client kubernetes.Interface) error {
	masterNic := ps.info.MasterNic
	nicStatus, exist := ps.info.NICsStatus[masterNic]
	if !exist {
		klog.Errorf("node %v doesn't contain master nic info: %v", ps.nodeName, masterNic)
		return fmt.Errorf("node %v doesn't contain master nic info: %v", ps.nodeName, masterNic)
	}
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
		if v.IoType != aggregator.IOType_NetworkIO {
			continue
		}
		if (v.WorkloadType == utils.GaIndex) && masterNic == v.DevRequest.DevName {
			nicStatus.GA.In += v.DevRequest.Request.Inbps
			nicStatus.GA.Out += v.DevRequest.Request.Outbps
			nicStatus.GA.Total += (v.DevRequest.Request.Inbps + v.DevRequest.Request.Outbps)
		}
		if v.WorkloadType == utils.BeIndex && masterNic == v.DevRequest.DevName {
			nicStatus.BE.Num -= 1
			if nicStatus.BE.Num < 0 {
				nicStatus.BE.Num = 0
			}
		}
	}
	ps.ch.PrintCacheInfo()
	return nil
}

func (ps *Resource) AdmitPod(pod *v1.Pod) (interface{}, error) {
	requests, _, err := ps.ch.GetRequestByAnnotation(pod, ps.nodeName, false)
	if err != nil {
		return nil, err
	}
	ret := make(map[string]*aggregator.IOResourceRequest)
	for dev, req := range requests {
		ret[dev] = req
		klog.V(utils.DBG).Infof("NetworkIO: Device %v request: %v", dev, req.String())
	}

	return ret, nil
}

func (ps *Resource) Clone() resource.ExtendedResource {
	// todo NetworkIO deepcopy
	return ps
}

func (ps *Resource) PrintInfo() {
	for nic, nicInfo := range ps.info.NICsStatus {
		klog.V(utils.DBG).Info("NIC: ", nic)
		klog.V(utils.DBG).Info("device name: ", nicInfo.Name)
		klog.V(utils.DBG).Info("DefaultSpeed: ", nicInfo.DefaultSpeed)
		klog.V(utils.DBG).Info("GA: ", *nicInfo.GA)
		klog.V(utils.DBG).Info("BE: ", *nicInfo.BE)
	}
}
