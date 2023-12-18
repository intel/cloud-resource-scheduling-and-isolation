/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"reflect"
	"testing"

	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

func TestParseReceiveMessageNet(t *testing.T) {
	BWs := map[string]*pb.AppNetworkBandwidthInfo{
		"pod1": {
			BwInfo: []*pb.NetworkBandwidthInfo{
				{
					Id:        "veth1",
					Value:     "1000000",
					IsIngress: true,
				},
			},
		},
		"pod2": {
			BwInfo: []*pb.NetworkBandwidthInfo{
				{
					Id:        "veth1",
					Value:     "3000000",
					IsIngress: true,
				},
			},
		},
	}

	resp := &pb.AppsBandwidth{
		IoiType: 2,
		Payload: &pb.AppsBandwidth_NetworkIoBw{
			NetworkIoBw: &pb.AppNetworkIOBandwidth{
				AppBwInfo: BWs,
			}},
	}

	NicInfos := map[string]utils.NicInfo{
		"12:34:56:78:23:24": {
			Name:        "veth1",
			Macaddr:     "12:34:56:78:23:24",
			SndGroupBPS: 100,
			RcvGroupBPS: 100,
			Type:        "masterNic",
		},
	}

	expectedResult := &agent.PodData{
		PodEvents: map[string]agent.PodEventInfo{
			"12:34:56:78:23:24": {
				"pod1": agent.PodRunInfo{
					Actual: &utils.Workload{
						InBps:  1000000 / 1000 / 1000 * 8,
						OutBps: 0,
					},
				},
				"pod2": agent.PodRunInfo{
					Actual: &utils.Workload{
						InBps:  3000000 / 1000 / 1000 * 8,
						OutBps: 0,
					},
				},
			},
		},
	}

	result := parseReceiveMessageNet(resp, NicInfos)

	rDev1Pod1 := result.PodEvents["12:34:56:78:23:24"]["pod1"].Actual
	eDev1Pod1 := expectedResult.PodEvents["12:34:56:78:23:24"]["pod1"].Actual
	rDev1Pod2 := result.PodEvents["12:34:56:78:23:24"]["pod2"].Actual
	eDev1Pod2 := expectedResult.PodEvents["12:34:56:78:23:24"]["pod2"].Actual

	if !reflect.DeepEqual(rDev1Pod1, eDev1Pod1) {
		t.Errorf("parseReceiveMessageDisk() = %+v, want %+v", rDev1Pod1, eDev1Pod1)
	}
	if !reflect.DeepEqual(rDev1Pod2, eDev1Pod2) {
		t.Errorf("parseReceiveMessageDisk() = %+v, want %+v", rDev1Pod2, eDev1Pod2)
	}
}
