/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package scheduler

import (
	"fmt"

	"k8s.io/kubernetes/pkg/scheduler/framework"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/util"
)

type Scorer interface {
	Score(*stateData, resource.Handle) (int64, error)
}

func getScorer(scoreStrategy util.ScoreStrategyType, resourceType util.ResourceType) (Scorer, error) {
	if resourceType != util.BlockIO && resourceType != util.NetworkIO && resourceType != util.RDT && resourceType != util.RDTQuantity {
		return nil, fmt.Errorf("unknown resource type %v", resourceType)
	}
	switch scoreStrategy {
	case util.LeastAllocated:
		return &LeastAllocatedScorer{resourceType: resourceType}, nil
	case util.MostAllocated:
		return &MostAllocatedScorer{resourceType: resourceType}, nil
	default:
		return nil, fmt.Errorf("unknown score strategy %v", scoreStrategy)
	}
}

type MostAllocatedScorer struct {
	resourceType util.ResourceType
}

func (scorer *MostAllocatedScorer) Score(state *stateData, rh resource.Handle) (int64, error) {
	if !state.nodeSupportIOI {
		return framework.MaxNodeScore, nil
	}
	ratio, err := rh.(resource.CacheHandle).NodePressureRatio(state.request, state.nodeResourceState)
	if err != nil {
		return 0, err
	}
	return int64(ratio * float64(framework.MaxNodeScore)), nil
}

type LeastAllocatedScorer struct {
	resourceType util.ResourceType
}

func (scorer *LeastAllocatedScorer) Score(state *stateData, rh resource.Handle) (int64, error) {
	if !state.nodeSupportIOI {
		return framework.MaxNodeScore, nil
	}
	ratio, err := rh.(resource.CacheHandle).NodePressureRatio(state.request, state.nodeResourceState)
	if err != nil {
		return 0, err
	}
	return int64((1.0 - ratio) * float64(framework.MaxNodeScore)), nil
}
