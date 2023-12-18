/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdtquantity

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
)

const (
	L3 string = "l3"
	MB string = "mb"
	BE string = "BE"
)

type RDTAnnotationResource struct {
	Requests RDTResource `json:"requests,omitempty"`
	Limits   RDTResource `json:"limits,omitempty"`
	Class    string      `json:"class,omitempty"`
}

type RDTResource struct {
	L3 L3Resource `json:"l3cache,omitempty"`
	Mb MbResource `json:"mb,omitempty"`
}

type L3Resource string

// L3Resource --> xx MiB
func (l L3Resource) Parse() (float64, error) {
	str := string(l)
	if l == "" {
		return 0, nil
	}
	if strings.HasSuffix(str, "MiB") {
		value, err := strconv.ParseInt(strings.TrimSuffix(str, "MiB"), 10, 10)
		if err != nil {
			return 0, err
		}
		return float64(value), nil
	}
	return 0, fmt.Errorf("missing 'MiB' value from L3Resource")
}

type MbResource string

// MbResource --> xx MBps
func (m MbResource) Parse() (float64, error) {
	str := string(m)
	if m == "" {
		return 0, nil
	}
	if strings.HasSuffix(str, "GBps") {
		value, err := strconv.ParseInt(strings.TrimSuffix(str, "GBps"), 10, 10)
		if err != nil {
			return 0, err
		}
		return float64(value) * 1024, nil
	} else if strings.HasSuffix(str, "MBps") {
		value, err := strconv.ParseInt(strings.TrimSuffix(str, "MBps"), 10, 10)
		if err != nil {
			return 0, err
		}
		return float64(value), nil
	}
	return 0, fmt.Errorf("missing 'GBps' or 'MBps' value from MbResource")
}

type Resource struct {
	nodeName string
	info     *NodeInfo
	ch       resource.CacheHandle
	sync.RWMutex
}

type classInfo struct {
	Requests rdtResource
	Limits   rdtResource
	PodList  []string
}

type rdtResource struct {
	l3 float64
	mb float64
}

type NodeInfo struct {
	BEScore              float64
	L3CacheCapacity      float64
	MBCapacity           float64
	BEL3CapacityBaseline float64
	BEMBCapacityBaseline float64
	classes              map[string]*classInfo
}

func (ps *Resource) Name() string {
	return "RDTQuantity"
}

func (ps *Resource) AddPod(pod *v1.Pod, rawRequest interface{}, client kubernetes.Interface) ([]*pb.IOResourceRequest, bool, error) {
	requestInfo, ok := rawRequest.(*pb.IOResourceRequest)
	requestsArray := []*pb.IOResourceRequest{}
	if !ok {
		return nil, false, fmt.Errorf("unable to convert state into %v's pod request format", ps.Name())
	}
	classResurce, exist := ps.info.classes[requestInfo.DevRequest.DevName]
	if exist && requestInfo.DevRequest.DevName == BE {
		classResurce.PodList = append(classResurce.PodList, string(pod.UID))
	} else if exist && requestInfo.DevRequest.Limit.Inbps == classResurce.Limits.l3 && requestInfo.DevRequest.Request.Inbps == classResurce.Requests.l3 &&
		requestInfo.DevRequest.RawLimit.Inbps == classResurce.Limits.mb && requestInfo.DevRequest.RawRequest.Inbps == classResurce.Requests.mb {
		classResurce.PodList = append(classResurce.PodList, string(pod.UID))
	} else {
		ps.info.classes[requestInfo.DevRequest.DevName] = &classInfo{
			Requests: rdtResource{
				l3: requestInfo.DevRequest.Request.Inbps,
				mb: requestInfo.DevRequest.RawRequest.Inbps,
			},
			Limits: rdtResource{
				l3: requestInfo.DevRequest.Limit.Inbps,
				mb: requestInfo.DevRequest.RawLimit.Inbps,
			},
			PodList: []string{string(pod.UID)},
		}
		ps.info.classes[BE].Limits.l3 -= requestInfo.DevRequest.Request.Inbps
		ps.info.classes[BE].Requests.l3 -= requestInfo.DevRequest.Request.Inbps
		ps.info.classes[BE].Limits.mb -= requestInfo.DevRequest.RawRequest.Inbps
		ps.info.classes[BE].Requests.mb -= requestInfo.DevRequest.RawRequest.Inbps
	}
	requestsArray = append(requestsArray, requestInfo)
	return requestsArray, true, nil
}

func (ps *Resource) RemovePod(pod *v1.Pod, client kubernetes.Interface) error {
	for class, classResource := range ps.info.classes {
		for i, p := range classResource.PodList {
			if p == string(pod.UID) {
				classResource.PodList = append(classResource.PodList[:i], classResource.PodList[i+1:]...)
				if len(classResource.PodList) == 0 && class != BE {
					delete(ps.info.classes, class)
					ps.info.classes[BE].Limits.l3 += classResource.Requests.l3
					ps.info.classes[BE].Requests.l3 += classResource.Requests.l3
					ps.info.classes[BE].Limits.mb += classResource.Requests.mb
					ps.info.classes[BE].Requests.mb += classResource.Requests.mb
				}
				return nil
			}
		}
	}
	return fmt.Errorf("pod %v not found in cache", pod.Name)
}

func (ps *Resource) AdmitPod(pod *v1.Pod) (interface{}, error) {
	requests, _, err := ps.ch.GetRequestByAnnotation(pod, ps.nodeName, false)
	if err != nil {
		return nil, err
	}
	var ret *pb.IOResourceRequest
	for _, request := range requests {
		ret = request
	}
	return ret, nil
}

func (ps *Resource) Clone() resource.ExtendedResource {
	// todo RDTQuantity deepcopy
	return ps
}

func (ps *Resource) PrintInfo() {
	klog.V(utils.DBG).Info("L3 cache capacity: ", ps.info.L3CacheCapacity)
	klog.V(utils.DBG).Info("MB capacity: ", ps.info.MBCapacity)
	klog.V(utils.DBG).Info("BE score: ", ps.info.BEScore)
	klog.V(utils.DBG).Info("BE L3 capacity baseline: ", ps.info.BEL3CapacityBaseline)
	klog.V(utils.DBG).Info("BE MB capacity baseline: ", ps.info.BEMBCapacityBaseline)
	for class, classResource := range ps.info.classes {
		klog.V(utils.DBG).Info("*** class name: ", class, " ***")
		klog.V(utils.DBG).Info("L3 request: ", classResource.Requests.l3)
		klog.V(utils.DBG).Info("L3 limit: ", classResource.Limits.l3)
		klog.V(utils.DBG).Info("MB request: ", classResource.Requests.mb)
		klog.V(utils.DBG).Info("MB limit: ", classResource.Limits.mb)
		klog.V(utils.DBG).Infof("Class %v has %v pods\n", class, len(classResource.PodList))
		for _, p := range classResource.PodList {
			klog.V(utils.DBG).Infof("Pod uid: %v\n", p)
		}
	}
}
