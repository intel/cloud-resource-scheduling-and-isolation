/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"fmt"
	"math"

	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
)

const (
	ReservedMag      = 10
	ReservedPart     = 0.1
	LimitKeepValue   = -2
	LimitMax         = -1
	LimitCompress    = 0
	LimitKeep        = 1
	LimitDecompress  = 2
	transBlockSize   = "32k"
	transPackageSize = "4k"
	InFlag           = 1
	OutFlag          = 2
	MaxRegisterRetry = 10
)

const (
	DiskWorkloadType int = iota
	NetWorkloadType
	RDTWorkloadType
	RDTQuantityWorkloadType
)

type ComposeBWLimit struct {
	Id string
	Wl utils.Workload
}

type PodParam struct {
	inLimit  float64
	outLimit float64
	request  float64
	flag     int // request is ok, 0: greater than request, 1: lower than request
}

type RunPods struct {
	expect   float64
	request  float64
	podParam map[string]PodParam // use pod string
	flag     int                 // check if update data 1, have data, 0 no data
}

func CalculateGroupBw(podDWl map[string]*utils.Workload, devId string) utils.Workload {
	wl := utils.Workload{InBps: 0, OutBps: 0, TotalBps: 0}
	for _, val := range podDWl {
		wl.InBps += val.InBps
		wl.OutBps += val.OutBps
		wl.TotalBps += val.TotalBps
	}
	klog.V(utils.DBG).Infof("CalculateActual Actual: %v", wl)
	return wl
}

func GetPodList(w *agent.WorkloadGroup) []string {
	podList := []string{}
	for podId, podWl := range w.RequestPods {
		if podWl != nil {
			podList = append(podList, podId)
		}
	}
	klog.V(utils.DBG).Infof("GetPodList: %v", podList)
	return podList
}

// delete different pod between current pod list and pervious pod list
func DeleteOutdatePod(w *agent.WorkloadGroup, visited map[string]struct{}) {
	for podUid := range w.ActualPods {
		if _, ok := visited[podUid]; !ok {
			delete(w.ActualPods, podUid)
		}
	}
	for podUid := range w.RequestPods {
		if _, ok := visited[podUid]; !ok {
			delete(w.RequestPods, podUid)
		}
	}
	for podUid := range w.LimitPods {
		if _, ok := visited[podUid]; !ok {
			delete(w.LimitPods, podUid)
		}
	}
	for podUid := range w.RawLimit {
		if _, ok := visited[podUid]; !ok {
			delete(w.RawLimit, podUid)
		}
	}
	for podUid := range w.RawRequest {
		if _, ok := visited[podUid]; !ok {
			delete(w.RawRequest, podUid)
		}
	}
	for podUid := range w.PodsLimitSet {
		if _, ok := visited[podUid]; !ok {
			delete(w.PodsLimitSet, podUid)
		}
	}
}

func SelectPods(pRunPods *RunPods,
	flag int,
	devId string,
	policy *agent.Policy,
	LimitPods map[string]*utils.Workload,
	RequestPods map[string]*utils.Workload, podList []string) {

	var request, current float64

	klog.V(utils.DBG).Infof("LimitPods=%+v, RequestPods=%+v", LimitPods, RequestPods)
	switch policy.Select_method {
	case "All":
		for _, po := range podList {
			// In
			if flag == 1 {
				request = (*RequestPods[po]).InBps

				if _, ok := LimitPods[po]; ok {
					current = (*LimitPods[po]).InBps
				} else {
					current = 0
				}
				pRunPods.podParam[po] = PodParam{current, LimitKeepValue, request, 0}
			}
			// Out
			if flag == 2 {
				request = (*RequestPods[po]).OutBps

				if _, ok := LimitPods[po]; ok {
					current = (*LimitPods[po]).OutBps
				} else {
					current = 0
				}
				pRunPods.podParam[po] = PodParam{current, LimitKeepValue, request, 0}
			}
		}
	case "NONE":
		pRunPods.podParam = nil
	default:
		klog.Warningf("the Select_method of policy is not All, no pods selected")
	}

	klog.V(utils.DBG).Infof("After select pods wRunPods.podParam=%+v", pRunPods.podParam)
}

func CompressPods(w *agent.WorkloadGroup, policy *agent.Policy, pRunPods *RunPods, podsNum int, idx int32) {
	if pRunPods.expect == LimitMax || pRunPods.expect == LimitKeepValue {
		// Set all pod limit limitMax
		for index := range pRunPods.podParam {
			if podParam, ok := pRunPods.podParam[index]; ok {
				current1 := podParam
				current1.outLimit = pRunPods.expect
				pRunPods.podParam[index] = current1
			}
		}
	} else if pRunPods.expect >= 0 {
		switch policy.Compress_method {
		case "CALC_REQUEST_RATIO":
			CompressRequestRatio(pRunPods, w.GroupSettable, podsNum, idx)
		}
	}
}

func CompressRequestRatio(pRunPods *RunPods, gSettable bool, podsNum int, idx int32) {
	if pRunPods.podParam != nil {
		for index := range pRunPods.podParam {
			if podParam, ok := pRunPods.podParam[index]; ok {
				current1 := podParam

				if idx == utils.BtIndex {
					fsLimit := pRunPods.expect * podParam.request
					current1.outLimit = math.Min(fsLimit, current1.inLimit)
				}
				if idx == utils.BeIndex {
					limit := pRunPods.expect / float64(podsNum)
					current1.outLimit = limit
				}
				if idx == utils.GaIndex {
					current1.outLimit = podParam.request
				}

				pRunPods.podParam[index] = current1
			}
		}
	}
}

func SetLimit(w *agent.WorkloadGroup, policy *agent.Policy, expect utils.Workload, request utils.Workload, dev DevInfo, idx int32) {
	podList := GetPodList(w)
	podsNum := len(podList)
	var rRunPods RunPods
	var wRunPods RunPods

	rRunPods.podParam = make(map[string]PodParam)
	wRunPods.podParam = make(map[string]PodParam)

	klog.V(utils.INF).Infof("SetLimit index:%d expect:%v request:%v policy:%v", idx, expect, request, policy)
	rRunPods.expect = expect.InBps
	rRunPods.request = request.InBps
	rRunPods.flag = 1
	SelectPods(&rRunPods, InFlag, dev.Id, policy, w.LimitPods, w.RequestPods, podList)

	wRunPods.expect = expect.OutBps
	wRunPods.request = request.OutBps
	wRunPods.flag = 1
	SelectPods(&wRunPods, OutFlag, dev.Id, policy, w.LimitPods, w.RequestPods, podList)

	// read compress/decompress
	CompressPods(w, policy, &rRunPods, podsNum, idx)

	// write compress/decompress
	CompressPods(w, policy, &wRunPods, podsNum, idx)

	if wRunPods.flag == 1 || rRunPods.flag == 1 {
		ComposeLimitData(w, rRunPods, wRunPods, dev, idx)
	}
}

func ComposeLimitData(w *agent.WorkloadGroup, rRunPods RunPods, wRunPods RunPods, dev DevInfo, idx int32) {
	klog.V(utils.DBG).Infof("ComposeLimitData: idx=%d, MajorMinor=%s", idx, dev.DevMajorMinor)
	klog.V(utils.DBG).Infof("ComposeLimitData: rRunPods=%v, wRunPods=%v", rRunPods, wRunPods)
	var limit ComposeBWLimit
	// set group level limit
	if w.GroupSettable && idx == utils.BeIndex {
		if rRunPods.flag == 1 {
			limit.Wl.InBps = rRunPods.expect
		}
		if wRunPods.flag == 1 {
			limit.Wl.OutBps = wRunPods.expect
		}
		limit.Id = dev.DevMajorMinor
		appid := fmt.Sprintf("%d", idx)

		if dev.DevType == "NIC" {
			klog.V(utils.DBG).Infof("setlimit for nic group appid=%s, limit=%v", appid, limit)
			IOISetLimitNet(appid, &limit, dev.NInfo)
			w.GroupLimit = &limit.Wl
		}
	}

	// set pod level limit
	pods := w.RequestPods
	for po := range pods {
		if rRunPods.flag == 1 {
			rPodParam, exist := rRunPods.podParam[po]
			if exist {
				limit.Wl.InBps = rPodParam.outLimit
			} else {
				limit.Wl.InBps = LimitKeepValue
			}
		} else {
			limit.Wl.InBps = LimitKeepValue
		}

		if wRunPods.flag == 1 {
			wPodParam, exist := wRunPods.podParam[po]
			if exist {
				limit.Wl.OutBps = wPodParam.outLimit
			} else {
				limit.Wl.OutBps = LimitKeepValue
			}
		} else {
			limit.Wl.OutBps = LimitKeepValue
		}

		limit.Id = dev.DevMajorMinor
		appid := fmt.Sprintf("%d.%s", idx, po)
		klog.V(utils.DBG).Infof("setLimit for %s pod appid=%s, limit=%v", dev.DevType, appid, limit)
		if dev.DevType == "DISK" {
			IOISetLimitDisk(appid, &limit, w)
		}
		if dev.DevType == "NIC" {
			IOISetLimitNet(appid, &limit, dev.NInfo)
		}
		w.PodsLimitSet[po] = &limit.Wl
	}
}
