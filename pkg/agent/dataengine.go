/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package agent

import (
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
)

// DataEngine
type DataEngine interface {
	Type() string
	Initialize(CoreClient *kubernetes.Clientset, client *versioned.Clientset, mtls bool) error
	Uninitialize() error
}

type CacheData struct {
	data EventData
	f    EventFunc
}

type IOEngine struct {
	Flag          int
	ExecutionFlag int
	WgDevices     map[string][]WorkloadGroup
	GpPolicy      []*Policy // group index
	CoreClient    *kubernetes.Clientset
	Clientset     *versioned.Clientset
	NodeName      string
	cacheDatas    []CacheData
}

func (e *IOEngine) SetFlag(flag int) {
	if (e.Flag & flag) != flag {
		e.Flag = e.Flag | flag
		// if e.IsExecution() {
		// 	// execute all cache data
		var newCache []CacheData
		for _, cacheData := range e.cacheDatas {
			if e.IsExpectStatus(cacheData.data) {
				err := cacheData.f(cacheData.data)
				if err != nil {
					log.Println(err)
				}
			} else {
				newCache = append(newCache, cacheData)
			}
		}
		e.cacheDatas = newCache
	}
}

func (e *IOEngine) IsExpectStatus(data EventData) bool {
	expectStatus := data.Expect()
	return (e.Flag & expectStatus) == expectStatus
}

func (e *IOEngine) IsExecution() bool {
	return e.Flag == e.ExecutionFlag
}

func (e *IOEngine) IsAdmin() bool {
	return (e.Flag & AdminFlag) == AdminFlag
}

func (e *IOEngine) AddWorkloadGroup(wg WorkloadGroup, DevId string) {
	klog.Infof("devId: %d, WorkloadType: %d", DevId, wg.WorkloadType)
	e.WgDevices[DevId] = append(e.WgDevices[DevId], wg)
}

func (e *IOEngine) InitializeWorkloadGroup(t int) {
	e.WgDevices = make(map[string][]WorkloadGroup)
}

func (e *IOEngine) HandleEvent(data EventData, ef EventFunc) error {
	if e.IsExpectStatus(data) {
		return ef(data)
	} else {
		e.cacheDatas = append(e.cacheDatas, CacheData{data, ef})
	}
	return nil
}
