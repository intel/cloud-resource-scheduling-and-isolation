/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package networkio

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"

	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	utils "sigs.k8s.io/IOIsolation/pkg"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	common "sigs.k8s.io/IOIsolation/pkg/scheduler/util"
)

type Handle struct {
	ioiresource.HandleBase
	sync.RWMutex
}

type PodAnnoThroughput struct {
	Requests NetIOConfig `json:"requests,omitempty"`
	Limits   NetIOConfig `json:"limits,omitempty"`
}

type NetIOConfig struct {
	Ingress string `json:"ingress,omitempty"`
	Egress  string `json:"egress,omitempty"`
}

// resource.Handle interface
func (h *Handle) Name() string {
	return "NetworkIOHandle"
}

func (h *Handle) Run(c ioiresource.ExtendedCache, f informers.SharedInformerFactory, cli kubernetes.Interface) error {
	h.EC = c
	return nil
}
func (h *Handle) CanAdmitPod(nodeName string, rawRequest interface{}) (bool, interface{}, error) {
	requests, ok := rawRequest.(map[string]*pb.IOResourceRequest)
	if !ok {
		return false, nil, errors.New("unable to convert state into networkIO's pod request format")
	}
	// prepare for node allocatable resource
	pools := make(map[string]map[string]*utils.IOPoolStatus)
	rs := h.EC.GetExtendedResource(nodeName)
	if rs == nil {
		return false, nil, fmt.Errorf("node %v not registered in cache", nodeName)
	}
	r, ok := rs.(*Resource)
	if !ok {
		return false, nil, fmt.Errorf("incorrect resource cached")
	}
	for dev, nicStatus := range r.info.NICsStatus {
		poolStatus := make(map[string]*utils.IOPoolStatus)
		poolStatus[utils.GA_Type] = nicStatus.GA
		poolStatus[utils.BE_Type] = nicStatus.BE
		poolStatus[utils.BE_Min_BW] = &utils.IOPoolStatus{
			In:    nicStatus.DefaultSpeed,
			Out:   nicStatus.DefaultSpeed,
			Total: nicStatus.DefaultSpeed * 2,
		}
		pools[dev] = poolStatus
	}
	// compare requested resource and node allocatable resource
	for dev, request := range requests {
		poolStatus, ok := pools[dev]
		if !ok {
			return false, nil, fmt.Errorf("device %v on Node %v is not registed: %v", dev, nodeName, h.Name())
		}
		if poolStatus == nil || len(poolStatus) < 3 {
			return false, nil, fmt.Errorf("internal %v error on Node %v", h.Name(), nodeName)
		}
		ga := poolStatus[utils.GA_Type]
		be := poolStatus[utils.BE_Type]
		bem := poolStatus[utils.BE_Min_BW]

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
	r, ok := request.(map[string]*pb.IOResourceRequest)
	if !ok {
		return 0, fmt.Errorf("unable to convert state into %v's pod request format", h.Name())
	}
	p, ok := pool.(map[string]map[string]*utils.IOPoolStatus)
	if !ok {
		return 0, fmt.Errorf("unable to convert state into %v's node resource pool format", h.Name())
	}
	nDev := len(r)
	for dev, req := range r {
		if req.WorkloadType == utils.BeIndex {
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
			devRatio += (req.DevRequest.Request.Inbps) / gapool.In
		} else {
			devRatio += 1
		}
		if gapool.Out != 0 {
			devRatio += (req.DevRequest.Request.Outbps) / gapool.Out
		} else {
			devRatio += 1
		}
		ratio += devRatio / 2.0
	}
	if nDev == 0 {
		return 0.5, nil
	}
	return ratio / float64(nDev), nil
}

func (h *Handle) GetRequestByAnnotation(pod *corev1.Pod, node string, init bool) (map[string]*pb.IOResourceRequest, interface{}, error) {
	requests := map[string]*pb.IOResourceRequest{}
	request := &pb.IOResourceRequest{}
	podAnno := PodAnnoThroughput{}
	rs := h.EC.GetExtendedResource(node)
	if rs == nil {
		klog.Error("node not registered in cache")
		return nil, nil, fmt.Errorf("node %v not registered in cache", node)
	}
	r, ok := rs.(*Resource)
	if !ok {
		klog.Error("incorrect resource cached")
		return nil, nil, fmt.Errorf("incorrect resource cached")
	}
	h.Lock()
	defer h.Unlock()
	masterNic := r.info.MasterNic
	if _, ok := r.info.NICsStatus[masterNic]; !ok {
		klog.Errorf("network io resource does not contain device info: %v", masterNic)
		return nil, nil, fmt.Errorf("network io resource does not contain device info: %v", masterNic)
	}
	if v, ok := pod.Annotations[utils.NetworkIOConfigAnno]; ok {
		if err := json.Unmarshal([]byte(v), &podAnno); err != nil {
			klog.Errorf("cannot parse to network io ingress/egress: %v, default to BE type", v)
		} else {
			request := h.parsePodNetworkIOAnnotations(podAnno, r.info)
			requests[masterNic] = request
			return requests, nil, nil
		}
	}
	//BE workloads
	request.IoType = pb.IOType_NetworkIO
	request.WorkloadType = utils.BeIndex
	request.DevRequest = &pb.DeviceRequirement{
		DevName: masterNic,
	}
	requests[masterNic] = request

	return requests, nil, nil
}

func (h *Handle) CriticalPod(annotations map[string]string) bool {
	podAnno := PodAnnoThroughput{}
	if v, ok := annotations[utils.NetworkIOConfigAnno]; ok {
		if err := json.Unmarshal([]byte(v), &podAnno); err != nil {
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
		if r.IoType != pb.IOType_NetworkIO || r.DevRequest == nil || r.DevRequest.DevName != dev {
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
		if _, ok := r.info.NICsStatus[dev]; !ok {
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
		if r.info.NICsStatus[dev].BE != nil {
			r.info.NICsStatus[dev].BE.Num = beNum
		} else {
			r.info.NICsStatus[dev].BE = &utils.IOPoolStatus{
				Num: beNum,
			}
		}
	}
	h.EC.PrintCacheInfo()
}

func (h *Handle) InitNodeStatus(nodeName string, rc v1.ResourceConfigSpec) {
	klog.V(utils.DBG).Info("Enter init node status")
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
		if _, ok := r.info.NICsStatus[dev]; !ok {
			klog.Errorf("device %v not registed in status cache", dev)
			continue
		}
		klog.V(utils.DBG).Infof("device %v info from cr: %v", dev, info)
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
		r.info.NICsStatus[dev].GA = &utils.IOPoolStatus{
			In:            float64((100-rc.BEPool)*int(info.CapacityIn)/100) - gaIn,
			Out:           float64((100-rc.BEPool)*int(info.CapacityOut)/100) - gaOut,
			Total:         float64((100-rc.BEPool)*int(info.CapacityTotal)/100) - gaIn - gaOut,
			UnderPressure: false,
		}
		if r.info.NICsStatus[dev].BE != nil {
			r.info.NICsStatus[dev].BE.Num = beNum
		} else {
			r.info.NICsStatus[dev].BE = &utils.IOPoolStatus{
				Num: beNum,
			}
		}
	}
	h.EC.PrintCacheInfo()
}

func (h *Handle) AddCacheNodeInfo(nodeName string, rc v1.ResourceConfigSpec) error {
	nodeInfo := &NodeInfo{
		NICsStatus: make(map[string]*NICInfo),
	}
	for dev, info := range rc.Devices {
		nicInfo := &NICInfo{
			GA: &utils.IOPoolStatus{},
			BE: &utils.IOPoolStatus{},
		}
		if info.Type == v1.DefaultDevice {
			nodeInfo.MasterNic = dev
		}
		nicInfo.Name = info.Name
		nicInfo.DefaultSpeed = info.DefaultIn
		nicInfo.BE.Total = float64(rc.BEPool * int(info.CapacityTotal) / 100)
		nicInfo.BE.In = float64(rc.BEPool * int(info.CapacityIn) / 100)
		nicInfo.BE.Out = float64(rc.BEPool * int(info.CapacityOut) / 100)
		nodeInfo.NICsStatus[dev] = nicInfo
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
	h.EC.DeleteExtendedResource(nodeName)
	return nil
}

func (h *Handle) UpdateCacheNodeStatus(n string, nodeIoBw *v1.NodeIOStatusStatus) error {
	klog.V(utils.DBG).Info("Net UpdateCacheNodeStatus from CR")
	for typ, bw := range nodeIoBw.AllocatableBandwidth {
		if len(bw.DeviceIOStatus) == 0 || typ != v1.NetworkIO {
			continue
		}
		rs := h.EC.GetExtendedResource(n)
		if rs == nil {
			return fmt.Errorf("node not registered in cache")
		}
		r, ok := rs.(*Resource)
		if !ok || r.info == nil {
			return fmt.Errorf("incorrect resource cached")
		}
		v, err := ioiresource.IoiContext.GetReservedPods(n)
		h.Lock()
		defer h.Unlock()
		if err == nil && nodeIoBw.ObservedGeneration == v.Generation {
			for dev, info := range r.info.NICsStatus {
				ga, be, update := h.calculateNetworkStatus(bw.DeviceIOStatus, dev)
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

func (h *Handle) calculateNetworkStatus(in map[string]v1.IOAllocatableBandwidth, dev string) (*utils.IOPoolStatus, *utils.IOPoolStatus, bool) {
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
	if ok && (v == "all" || v == "net") {
		return true
	} else if ok {
		resources := strings.Split(v, "-")
		for _, r := range resources {
			if r == "net" {
				return true
			}
		}
	}
	return false
}

// New Resource Handle
func New() ioiresource.Handle {
	return &Handle{}
}

func parseNetworkRequestBandwidth(config string) (resource.Quantity, error) {
	if len(config) > 0 && (config[len(config)-1] == 'B') {
		config = config[:len(config)-1]
	}
	bw, err := resource.ParseQuantity(config)
	return bw, err
}

func (h *Handle) parsePodNetworkIOAnnotations(podAnno PodAnnoThroughput, info *NodeInfo) *pb.IOResourceRequest {
	masterNic := info.MasterNic
	request := &pb.IOResourceRequest{
		IoType: pb.IOType_NetworkIO,
		DevRequest: &pb.DeviceRequirement{
			DevName: masterNic,
			Request: &pb.Quantity{},
			Limit:   &pb.Quantity{},
		},
	}
	inbw, inerr := parseNetworkRequestBandwidth(podAnno.Requests.Ingress)
	if inerr == nil {
		bwValue, _ := inbw.AsInt64()
		request.DevRequest.Request.Inbps = float64(bwValue) / utils.MB
	}

	outbw, outerr := parseNetworkRequestBandwidth(podAnno.Requests.Egress)
	if outerr == nil {
		bwValue, _ := outbw.AsInt64()
		request.DevRequest.Request.Outbps = float64(bwValue) / utils.MB
	}

	inlimbw, inlimerr := parseNetworkRequestBandwidth(podAnno.Limits.Ingress)
	if inlimerr == nil {
		bwValue, _ := inlimbw.AsInt64()
		request.DevRequest.Limit.Inbps = float64(bwValue) / utils.MB
	}
	outlimbw, outlimerr := parseNetworkRequestBandwidth(podAnno.Limits.Egress)
	if outlimerr == nil {
		bwValue, _ := outlimbw.AsInt64()
		request.DevRequest.Limit.Outbps = float64(bwValue) / utils.MB
	}
	if inerr == nil && inlimerr == nil && outerr == nil && outlimerr == nil &&
		request.DevRequest.Request.Inbps*request.DevRequest.Request.Outbps*request.DevRequest.Limit.Inbps*request.DevRequest.Limit.Outbps > 0 &&
		inbw.Equal(inlimbw) && outbw.Equal(outlimbw) {
		request.WorkloadType = utils.GaIndex
		if _, ok := info.NICsStatus[masterNic]; ok {
			request.DevRequest.Request.Inbps = math.Max(request.DevRequest.Request.Inbps, info.NICsStatus[masterNic].DefaultSpeed)
			request.DevRequest.Request.Outbps = math.Max(request.DevRequest.Request.Outbps, info.NICsStatus[masterNic].DefaultSpeed)
			request.DevRequest.Limit.Inbps = math.Max(request.DevRequest.Limit.Inbps, info.NICsStatus[masterNic].DefaultSpeed)
			request.DevRequest.Limit.Outbps = math.Max(request.DevRequest.Limit.Outbps, info.NICsStatus[masterNic].DefaultSpeed)
		}
	} else if inerr != nil && inlimerr != nil && outerr != nil && outlimerr != nil {
		request.WorkloadType = utils.BeIndex
	} else {
		request.WorkloadType = utils.GaIndex
		if inlimerr != nil {
			request.DevRequest.Limit.Inbps = utils.MAX_BT_LIMIT
		}
		if outlimerr != nil {
			request.DevRequest.Limit.Outbps = utils.MAX_BT_LIMIT
		}
		if _, ok := info.NICsStatus[masterNic]; ok {
			request.DevRequest.Request.Inbps = math.Max(request.DevRequest.Request.Inbps, info.NICsStatus[masterNic].DefaultSpeed)
			request.DevRequest.Request.Outbps = math.Max(request.DevRequest.Request.Outbps, info.NICsStatus[masterNic].DefaultSpeed)
			request.DevRequest.Limit.Inbps = math.Max(request.DevRequest.Limit.Inbps, info.NICsStatus[masterNic].DefaultSpeed)
			request.DevRequest.Limit.Outbps = math.Max(request.DevRequest.Limit.Outbps, info.NICsStatus[masterNic].DefaultSpeed)
		}
	}

	return request
}

func (h *Handle) UpdatePVC(pvc *corev1.PersistentVolumeClaim, nodename string, devId string, storage int64, add bool) error {
	return nil
}
