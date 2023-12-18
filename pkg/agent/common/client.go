/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"encoding/json"

	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

func IOIConfigureLoglevel(ioType int32, info bool, debug bool) error {
	var configinfo utils.LogLevelConfigInfo
	configinfo.Info = info
	configinfo.Debug = debug

	data, jsonErr := json.Marshal(configinfo)
	if jsonErr != nil {
		klog.Error("Marshal is failed: ", jsonErr)
		return jsonErr
	}
	klog.V(utils.INF).Infof("configinfo is %v", configinfo)

	agent.ClientHandlerChan <- &pb.ConfigureRequest{Type: "LOGLEVEL", Configure: string(data), IoiType: ioType}

	return nil
}

func IOIConfigureInterval(ioType int32, interval int) error {
	var configInfo utils.IntervalConfigInfo
	configInfo.Interval = interval
	data, jsonErr := json.Marshal(configInfo)
	if jsonErr != nil {
		klog.Error("Marshal is failed: ", jsonErr)
		return jsonErr
	}

	agent.ClientHandlerChan <- &pb.ConfigureRequest{Type: "INTERVAL", Configure: string(data), IoiType: ioType}

	return nil
}

func getAverageResult(resultArr []agent.PodData) agent.PodData {
	var sum agent.PodData
	sum.T = resultArr[0].T
	sum.Generation = resultArr[0].Generation
	sum.PodEvents = make(map[string]agent.PodEventInfo)
	times := len(resultArr)
	if times <= 0 {
		klog.Error("len of resultArr is 0")
	}
	for _, result := range resultArr {
		for devId, podEvent := range result.PodEvents {
			if _, ok := sum.PodEvents[devId]; !ok {
				sum.PodEvents[devId] = make(map[string]agent.PodRunInfo)
			}
			for podId, podInfo := range podEvent {
				if _, ok := sum.PodEvents[devId][podId]; !ok {
					sum.PodEvents[devId][podId] = agent.PodRunInfo{
						Actual: &utils.Workload{
							InBps:  0,
							OutBps: 0,
						},
						RawActualBs: &map[string]*utils.Workload{
							"512b": {
								InBps:  0,
								OutBps: 0,
							},
							"1k": {
								InBps:  0,
								OutBps: 0,
							},
							"4k": {
								InBps:  0,
								OutBps: 0,
							},
							"8k": {
								InBps:  0,
								OutBps: 0,
							},
							"16k": {
								InBps:  0,
								OutBps: 0,
							},
							"32k": {
								InBps:  0,
								OutBps: 0,
							},
						},
					}
				}
				pWlActual := sum.PodEvents[devId][podId].Actual
				pWlActual.InBps += podInfo.Actual.InBps
				pWlActual.OutBps += podInfo.Actual.OutBps

				pWlBsActual := sum.PodEvents[devId][podId].RawActualBs
				for bs, bsInfo := range *(podInfo.RawActualBs) {
					(*pWlBsActual)[bs].InBps += bsInfo.InBps
					(*pWlBsActual)[bs].OutBps += bsInfo.OutBps
				}
			}
		}
	}
	for _, devInfo := range sum.PodEvents {
		for _, podInfo := range devInfo {
			dwl := podInfo.Actual
			dwl.InBps = dwl.InBps / float64(times)
			dwl.OutBps = dwl.OutBps / float64(times)
			for _, bs := range *(podInfo.RawActualBs) {
				bs.InBps = bs.InBps / float64(times)
				bs.OutBps = bs.OutBps / float64(times)
			}
		}
	}
	return sum
}
