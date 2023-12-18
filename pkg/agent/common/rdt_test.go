/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

func TestIOIRegisterRDT(t *testing.T) {
	requests := make(chan interface{}, 1)
	agent.ClientHandlerChan = requests
	containerInfo := agent.ContainerInfo{
		Operation:           1,
		PodName:             "pod1",
		ContainerName:       "container1",
		ContainerId:         "1",
		Namespace:           "default",
		RDTAnnotation:       "class1",
		ContainerCGroupPath: "/xxx",
	}

	IOIRegisterRDT(containerInfo)

	expectedRequest := &pb.RegisterAppRequest{
		AppName: "1",
		IoiType: utils.RDTIOIType,
		AppInfo: "{\"containername\":\"container1\",\"cgrouppath\":\"/xxx\",\"rdtclass\":\"class1\",\"poduid\":\"\",\"podname\":\"\",\"rdtclassinfo\":{}}",
	}

	assert.Equal(t, expectedRequest, <-agent.ClientHandlerChan)
}

func TestIOIUnRegisterRDT(t *testing.T) {
	requests := make(chan interface{}, 1)
	agent.ClientHandlerChan = requests
	containerInfo := agent.ContainerInfo{
		Operation:           1,
		PodName:             "pod1",
		ContainerName:       "container1",
		ContainerId:         "1",
		Namespace:           "default",
		RDTAnnotation:       "class1",
		ContainerCGroupPath: "/xxx",
	}

	IOIUnRegisterRDT(containerInfo)

	expectedRequest := &pb.UnRegisterAppRequest{
		AppId:   "1",
		IoiType: utils.RDTIOIType,
	}

	assert.Equal(t, expectedRequest, <-agent.ClientHandlerChan)
}
