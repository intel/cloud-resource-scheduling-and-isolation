/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package v1

var (
	defaultScoreStrategy string = "LeastAllocated"
	defaultResourceType  string = "BlockIO"
)

// SetDefaults_ResourceIOArgs sets the default parameters for ResourceIO plugin.
func SetDefaults_ResourceIOArgs(obj *ResourceIOArgs) {
	if obj.ScoreStrategy == nil {
		obj.ScoreStrategy = &defaultScoreStrategy
	}

	if obj.ResourceType == nil {
		obj.ResourceType = &defaultResourceType
	}
}
