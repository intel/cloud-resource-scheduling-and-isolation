/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeIOStatusSpec defines the desired state of NodeDiskIOStatus
type NodeIOStatusSpec struct {
	NodeName   string `json:"nodeName,omitempty"`
	Generation int64  `json:"generation,omitempty"`
	// a list of pod
	ReservedPods map[string]PodRequest `json:"reservedPods,omitempty"`
}

type PodRequest struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// NodeIOStatusStatus defines the observed state of NodeIOStatus
type NodeIOStatusStatus struct {
	ObservedGeneration   int64                                         `json:"observedGeneration,omitempty"`
	AllocatableBandwidth map[WorkloadIOType]DeviceAllocatableBandwidth `json:"allocatableBandwidth,omitempty"`
}

type DeviceAllocatableBandwidth struct {
	// key -> device id
	DeviceIOStatus map[string]IOAllocatableBandwidth `json:"deviceIOStatus,omitempty"`
}

type IOAllocatableBandwidth struct {
	// The IO status for each QoS pool
	IOPoolStatus map[WorkloadQoSType]IOStatus `json:"ioPoolStatus,omitempty"`
}

type IOStatus struct {
	// Total io bandwidth capacity
	Total float64 `json:"total,omitempty"`
	// Read io bandwidth capacity
	In float64 `json:"in,omitempty"`
	// Write io bandwidth capacity
	Out float64 `json:"out,omitempty"`
	// Pressure indicates if resource's pressure state
	Pressure float64 `json:"pressure,omitempty"`
	// Default is no entry
	ActualSize string `json:"actualSize,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NodeDiskIOStatusInfo is the Schema for the nodediskiostatusinfoes API
type NodeIOStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeIOStatusSpec   `json:"spec,omitempty"`
	Status NodeIOStatusStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeIOStatusList contains a list of NodeDiskIOStatusInfo
type NodeIOStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeIOStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeIOStatus{}, &NodeIOStatusList{})
}
