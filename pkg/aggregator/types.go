/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package aggregator

import (
	"time"

	"google.golang.org/grpc"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/pkg/api/nodeagent"
)

type Config struct {
	IsMtls         bool
	Direct         bool
	UpdateInterval time.Duration
}
type Endpoint struct {
	IP   string
	Port string
}

type NodeInfo4Agent struct {
	reservedPod      map[string]v1.PodRequest // pod uid -> pod info
	latestGeneration int64
	client           nodeagent.NodeAgentClient
	conn             *grpc.ClientConn
}
