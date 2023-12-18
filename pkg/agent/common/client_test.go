/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

func TestGetAverageResult(t *testing.T) {
	resultArr := []agent.PodData{
		{
			T:          1,
			Generation: 1,
			PodEvents: map[string]agent.PodEventInfo{
				"dev1": {
					"pod1": {
						Actual: &utils.Workload{
							InBps:  10,
							OutBps: 20,
						},
						RawActualBs: &map[string]*utils.Workload{
							"512b": {
								InBps:  5,
								OutBps: 10,
							},
							"1k": {
								InBps:  10,
								OutBps: 20,
							},
						},
					},
				},
			},
		},
		{
			T:          2,
			Generation: 2,
			PodEvents: map[string]agent.PodEventInfo{
				"dev1": {
					"pod1": {
						Actual: &utils.Workload{
							InBps:  20,
							OutBps: 30,
						},
						RawActualBs: &map[string]*utils.Workload{
							"512b": {
								InBps:  10,
								OutBps: 15,
							},
							"1k": {
								InBps:  20,
								OutBps: 30,
							},
						},
					},
				},
			},
		},
	}

	expectedResult := agent.PodData{
		PodEvents: map[string]agent.PodEventInfo{
			"dev1": {
				"pod1": {
					Actual: &utils.Workload{
						InBps:  15,
						OutBps: 25,
					},
					RawActualBs: &map[string]*utils.Workload{
						"512b": {
							InBps:  7.5,
							OutBps: 12.5,
						},
						"1k": {
							InBps:  15,
							OutBps: 25,
						},
					},
				},
			},
		},
	}

	averageResult := getAverageResult(resultArr)
	assert.Equal(t, expectedResult.PodEvents["dev1"]["pod1"].Actual, averageResult.PodEvents["dev1"]["pod1"].Actual)
	peRawActualBs := expectedResult.PodEvents["dev1"]["pod1"].RawActualBs
	aeRawActualBs := averageResult.PodEvents["dev1"]["pod1"].RawActualBs
	assert.Equal(t, (*peRawActualBs)["512b"], (*aeRawActualBs)["512b"])
	assert.Equal(t, (*peRawActualBs)["1k"], (*aeRawActualBs)["1k"])
}

func TestIOIConfigureLoglevel(t *testing.T) {
	IoType := []int32{utils.DiskIOIType, utils.NetIOIType, utils.RDTIOIType, utils.RDTQuantityIOIType}
	info := true
	debug := false

	// Assert that rdtClientHandlerChan received the correct ConfigureRequest
	expectedRequest := &pb.ConfigureRequest{
		Type:      "LOGLEVEL",
		Configure: `{"info":true,"debug":false}`,
	}

	for _, ioType := range IoType {
		err := IOIConfigureLoglevel(ioType, info, debug)

		if err != nil {
			t.Errorf("Expected nil error, but got %v", err)
		}

		expectedRequest.IoiType = ioType
		actualRequest := <-agent.ClientHandlerChan
		if !reflect.DeepEqual(actualRequest, expectedRequest) {
			t.Errorf("Expected %v, but got %v", expectedRequest, actualRequest)
		}
	}
}

func TestIOIConfigureInterval(t *testing.T) {
	IoType := []int32{utils.DiskIOIType, utils.NetIOIType, utils.RDTIOIType}
	interval := 10

	// Assert that rdtClientHandlerChan received the correct ConfigureRequest
	expectedRequest := &pb.ConfigureRequest{
		Type:      "INTERVAL",
		Configure: `{"interval":10}`,
	}

	for _, ioType := range IoType {
		err := IOIConfigureInterval(ioType, interval)

		if err != nil {
			t.Errorf("Expected nil error, but got %v", err)
		}

		expectedRequest.IoiType = ioType
		actualRequest := <-agent.ClientHandlerChan
		if !reflect.DeepEqual(actualRequest, expectedRequest) {
			t.Errorf("Expected %v, but got %v", expectedRequest, actualRequest)
		}
	}
}
