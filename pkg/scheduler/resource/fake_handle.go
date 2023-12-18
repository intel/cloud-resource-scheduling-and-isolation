/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package resource

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
)

type FakeHandle struct {
	HandleBase
	client                  kubernetes.Interface
	UpdateNodeIOStatusFunc  func(n string, nodeIoBw *v1.NodeIOStatusStatus) error
	DeleteCacheNodeInfoFunc func(string) error
	sync.RWMutex
}

// ioiresource.Handle interface
func (h *FakeHandle) Name() string {
	return "FakeHandle"
}

func (h *FakeHandle) Run(c ExtendedCache, f informers.SharedInformerFactory, cli kubernetes.Interface) error {
	h.EC = c
	h.client = cli
	return nil
}

func (h *FakeHandle) CanAdmitPod(nodeName string, rawRequest interface{}) (bool, interface{}, error) {
	return true, nil, nil
}
func (h *FakeHandle) NodePressureRatio(request interface{}, pool interface{}) (float64, error) {
	return 0, nil
}

func (h *FakeHandle) GetRequestByAnnotation(pod *corev1.Pod, node string, init bool) (map[string]*pb.IOResourceRequest, interface{}, error) {
	return nil, nil, nil
}

func (h *FakeHandle) CriticalPod(annotations map[string]string) bool {
	return true
}

func (h *FakeHandle) InitBENum(nodeName string, rc v1.ResourceConfigSpec) {
}

func (h *FakeHandle) InitNodeStatus(nodeName string, rc v1.ResourceConfigSpec) {
}

// assume the IoIContext lock is acquired
func (h *FakeHandle) AddCacheNodeInfo(nodeName string, rc v1.ResourceConfigSpec) error {
	return nil
}

func (h *FakeHandle) UpdateDeviceNodeInfo(node string, old *v1.ResourceConfigSpec, new *v1.ResourceConfigSpec) (map[string]v1.Device, map[string]v1.Device, error) {
	return nil, nil, nil
}

func (h *FakeHandle) UpdatePVC(pvc *corev1.PersistentVolumeClaim, nodename string, devId string, storage int64, add bool) error {
	return nil
}

func (h *FakeHandle) DeleteCacheNodeInfo(nodeName string) error {
	return h.DeleteCacheNodeInfoFunc(nodeName)
}

func (h *FakeHandle) UpdateCacheNodeStatus(n string, nodeIoBw *v1.NodeIOStatusStatus) error {
	return h.UpdateNodeIOStatusFunc(n, nodeIoBw)
}

func (h *FakeHandle) NodeHasIoiLabel(node *corev1.Node) bool {
	_, ok := node.Labels["ioisolation"]
	return ok
}
