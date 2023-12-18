/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package resource

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/IOIsolation/pkg/api/aggregator"
)

type FakeResource struct {
	AddPodFunc    func(*v1.Pod, interface{}, kubernetes.Interface) ([]*aggregator.IOResourceRequest, bool, error)
	RemovePodFunc func(*v1.Pod, kubernetes.Interface) error
	AdmitPodFunc  func(*v1.Pod) (interface{}, error)
}

func (f *FakeResource) Name() string {
	return "FakeResource"
}

func (f *FakeResource) AddPod(pod *v1.Pod, request interface{}, client kubernetes.Interface) ([]*aggregator.IOResourceRequest, bool, error) {
	return f.AddPodFunc(pod, request, client)
}

func (f *FakeResource) RemovePod(pod *v1.Pod, client kubernetes.Interface) error {
	return f.RemovePodFunc(pod, client)
}

func (f *FakeResource) Clone() ExtendedResource {
	return &FakeResource{}
}

func (f *FakeResource) AdmitPod(pod *v1.Pod) (interface{}, error) {
	return f.AdmitPodFunc(pod)
}

func (f *FakeResource) PrintInfo() {
}
