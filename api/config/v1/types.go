/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// ResourceIOArgs holds arguments used to configure ResourceIO plugin.
type ResourceIOArgs struct {
	metav1.TypeMeta `json:",inline"`

	ScoreStrategy      *string `json:"scoreStrategy,omitempty"`
	ResourceType       *string `json:"resourceType,omitempty"`
	BinPackingStrategy *string `json:"binPackingStrategy,omitempty"`
}
