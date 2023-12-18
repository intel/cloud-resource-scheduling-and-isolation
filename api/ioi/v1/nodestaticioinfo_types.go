/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeviceType string
type WorkloadQoSType string
type WorkloadIOType string

const (
	DefaultDevice DeviceType      = "default"
	OtherDevice   DeviceType      = "others"
	WorkloadBE    WorkloadQoSType = "BE"
	WorkloadGA    WorkloadQoSType = "GA"
	WorkloadBT    WorkloadQoSType = "BT"

	BlockIO     WorkloadIOType = "BlockIO"
	NetworkIO   WorkloadIOType = "NetworkIO"
	RDT         WorkloadIOType = "RDT"
	RDTQuantity WorkloadIOType = "RDTQuantity"
)

// NodeStaticIOInfoSpec defines the desired state of NodeStaticIOInfo
type NodeStaticIOInfoSpec struct {
	NodeName       string                        `json:"nodeName"`
	ResourceConfig map[string]ResourceConfigSpec `json:"resourceConfig,omitempty"`
	EndPoint       string                        `json:"endpoint,omitempty"`
}

type ResourceConfigSpec struct {
	// BEPool means best effert QoS block io bandwidth percentage
	// +optional
	BEPool int `json:"bePool,omitempty"`
	// GAPool means guaranteed QoS block io bandwidth percentage
	// +optional
	GAPool int `json:"gaPool,omitempty"`
	// BTPool means burstable QoS block io bandwidth percentage
	// +optional
	BTPool int `json:"btPool,omitempty"`
	// Devices' measured profiles
	// +optional
	Devices map[string]Device `json:"devices,omitempty"`
	// Supported hardware classes
	// +optional
	Classes []string `json:"classes,omitempty"`
}

type Device struct {
	// Device name
	Name string `json:"name"`
	// Default or not
	Type DeviceType `json:"type,omitempty"`
	// Read/Ingress io bandwidth limit in 32k blocksize
	DefaultIn float64 `json:"defaultIn,omitempty"`
	// Write/Egress io bandwidth limit in 32k blocksize
	DefaultOut float64 `json:"defaultOut,omitempty"`
	// Different block size's io read bandwidth mapping coefficents
	ReadRatio map[string]float64 `json:"readRatio,omitempty"`
	// Different block size's io write bandwidth mapping coefficents
	WriteRatio map[string]float64 `json:"writeRatio,omitempty"`
	// Profile result of read/ingress io bandwidth capacity
	CapacityIn float64 `json:"capacityIn,omitempty"`
	// Profile result of write/egress io bandwidth capacity
	CapacityOut float64 `json:"capacityOut,omitempty"`
	// Profile result mixed read/ingress and write/egress io bandwidth capacity
	CapacityTotal float64 `json:"capacityTotal,omitempty"`
	// Disk's capacity, only for disk device
	DiskSize string `json:"diskSize,omitempty"`
	// Disk's mount path, only for disk device
	MountPath string `json:"mountPath,omitempty"`
}

type DeviceState struct {
	Name string `json:"name"`
	// State machine: NotEnabled -> Profiling -> Ready
	State string `json:"state"`
	// Write Reason if profiling error
	Reason string `json:"reason"`
}

// NodeStaticIOInfoStatus defines the observed state of NodeStaticIOInfo
type NodeStaticIOInfoStatus struct {
	DeviceStates map[string]DeviceState `json:"deviceStates,omitempty"`
}

// +genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodeStaticIOInfo is the Schema for the NodeStaticIOInfoes API
type NodeStaticIOInfo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeStaticIOInfoSpec   `json:"spec,omitempty"`
	Status NodeStaticIOInfoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeStaticIOInfoList contains a list of NodeStaticIOInfo
type NodeStaticIOInfoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeStaticIOInfo `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeStaticIOInfo{}, &NodeStaticIOInfoList{})
}
