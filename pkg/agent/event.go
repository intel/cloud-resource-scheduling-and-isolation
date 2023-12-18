/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package agent

import utils "sigs.k8s.io/IOIsolation/pkg"

var (
	chanSize          = 500
	ProfileChan       = make(chan EventData, chanSize)
	PolicyChan        = make(chan EventData, chanSize)
	AdminChan         = make(chan EventData, chanSize)
	PodInfoChan       = make(chan EventData, chanSize)
	ContainerInfoChan = make(chan EventData, chanSize)
	ClassInfoChan     = make(chan EventData, chanSize)
	DisksProfiled     = make(chan utils.BlockDevice, chanSize)
	HeartBeatChan     = make(chan EventData, chanSize)
)

// Event
type EventData interface {
	Type() string
	Expect() int
}

type EventFunc func(data EventData) error

// EventCacheEngine
type EventCacheEngine interface {
	HandleEvent(data EventData, f EventFunc) error
}
type EventHandle struct {
	engine                   EventCacheEngine
	needCacheBeforeExecution bool
	key                      string
	f                        EventFunc
	msg                      string
}

type EventHandler struct {
	ch      chan EventData
	handles []EventHandle
}
