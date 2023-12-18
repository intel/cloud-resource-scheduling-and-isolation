/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package resource

import (
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

type ExtendedCache interface {
	SetExtendedResource(nodeName string, val ExtendedResource)
	GetExtendedResource(nodeName string) ExtendedResource
	DeleteExtendedResource(nodeName string)
	PrintCacheInfo()
}

type ResourceCache struct {
	Resources map[string]ExtendedResource
}

func NewExtendedCache() ExtendedCache {
	c := &ResourceCache{
		Resources: make(map[string]ExtendedResource),
	}

	return c
}

func (cache *ResourceCache) SetExtendedResource(nodeName string, val ExtendedResource) {
	cache.Resources[nodeName] = val
}

func (cache *ResourceCache) GetExtendedResource(nodeName string) ExtendedResource {
	val, ok := cache.Resources[nodeName]
	if !ok {
		return nil
	}
	return val
}

func (cache *ResourceCache) DeleteExtendedResource(nodeName string) {
	delete(cache.Resources, nodeName)
}

func (cache *ResourceCache) PrintCacheInfo() {
	klog.V(utils.DBG).Info("Print Cache Info")
	for node, er := range cache.Resources {
		klog.V(utils.DBG).Infof("==========Node: %s=============", node)
		er.PrintInfo()
	}
}
