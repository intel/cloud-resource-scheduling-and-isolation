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

func TestIOIRegisterRDTQuantity(t *testing.T) {
	type args struct {
		containerInfo agent.ContainerInfo
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test register rdt quantity",
			args: args{
				containerInfo: agent.ContainerInfo{
					Operation:             1,
					PodName:               "pod1",
					ContainerName:         "container1",
					PodUid:                "pod1",
					ContainerId:           "1",
					Namespace:             "default",
					ContainerCGroupPath:   "/xxx",
					RDTQuantityAnnotation: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"50MBps\",\"l3cache\":\"100MiB\"},\"class\":\"class2\"}",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			IOIRegisterRDTQuantity(tt.args.containerInfo)
		})
	}
}

func TestIOIUnRegisterRDTQuantity(t *testing.T) {
	requests := make(chan interface{}, 1)
	agent.ClientHandlerChan = requests

	containerInfo := agent.ContainerInfo{
		Operation:             1,
		PodName:               "pod1",
		ContainerName:         "container1",
		ContainerId:           "1",
		Namespace:             "default",
		ContainerCGroupPath:   "/xxx",
		RDTQuantityAnnotation: "{\"requests\":{\"mb\":\"30MBps\",\"l3cache\":\"40MiB\"},\"limits\":{\"mb\":\"50MBps\",\"l3cache\":\"100MiB\"},\"class\":\"class2\"}",
	}

	IOIUnRegisterRDTQuantity(containerInfo)

	expectedRequest := &pb.UnRegisterAppRequest{
		AppId:   "1",
		IoiType: utils.RDTQuantityIOIType,
	}

	assert.Equal(t, expectedRequest, <-agent.ClientHandlerChan)
}
