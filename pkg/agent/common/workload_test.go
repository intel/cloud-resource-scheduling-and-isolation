/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"reflect"
	"testing"

	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
)

func TestCalculateGroupBw(t *testing.T) {
	podDWl := map[string]*utils.Workload{
		"pod1": {
			InBps:    100,
			OutBps:   200,
			TotalBps: 300,
		},
		"pod2": {
			InBps:    150,
			OutBps:   250,
			TotalBps: 400,
		},
	}

	devId := "device1"

	want := utils.Workload{
		InBps:    250,
		OutBps:   450,
		TotalBps: 700,
	}

	got := CalculateGroupBw(podDWl, devId)

	if got != want {
		t.Errorf("CalculateGroupBw() = %v, want %v", got, want)
	}
}

func TestGetPodList(t *testing.T) {
	w := &agent.WorkloadGroup{
		RequestPods: map[string]*utils.Workload{
			"pod1": {
				InBps:    100,
				OutBps:   200,
				TotalBps: 300,
			},
			"pod2": {
				InBps:    150,
				OutBps:   250,
				TotalBps: 400,
			},
			"pod3": nil,
		},
	}

	want := []string{"pod1", "pod2"}

	got := GetPodList(w)

	if len(got) != len(want) {
		t.Errorf("GetPodList() returned incorrect number of pods, got: %v, want: %v", len(got), len(want))
	}
}

func TestDeleteOutdatePod(t *testing.T) {
	w := &agent.WorkloadGroup{
		ActualPods: map[string]*utils.Workload{
			"pod1": {
				InBps:    100,
				OutBps:   200,
				TotalBps: 300,
			},
			"pod2": {
				InBps:    150,
				OutBps:   250,
				TotalBps: 400,
			},
		},
		RequestPods: map[string]*utils.Workload{
			"pod3": {
				InBps:    200,
				OutBps:   300,
				TotalBps: 500,
			},
			"pod4": {
				InBps:    250,
				OutBps:   350,
				TotalBps: 600,
			},
		},
		LimitPods: map[string]*utils.Workload{
			"pod5": {
				InBps:    300,
				OutBps:   400,
				TotalBps: 700,
			},
			"pod6": {
				InBps:    350,
				OutBps:   450,
				TotalBps: 800,
			},
		},
		RawLimit: map[string]*utils.Workload{
			"pod5": {
				InBps:    300,
				OutBps:   400,
				TotalBps: 700,
			},
			"pod6": {
				InBps:    350,
				OutBps:   450,
				TotalBps: 800,
			},
		},
		RawRequest: map[string]*utils.Workload{
			"pod5": {
				InBps:    300,
				OutBps:   400,
				TotalBps: 700,
			},
			"pod6": {
				InBps:    350,
				OutBps:   450,
				TotalBps: 800,
			},
		},
		PodsLimitSet: map[string]*utils.Workload{
			"pod5": {
				InBps:    300,
				OutBps:   400,
				TotalBps: 700,
			},
			"pod6": {
				InBps:    350,
				OutBps:   450,
				TotalBps: 800,
			},
		},
	}

	visited := map[string]struct{}{
		"pod1": {},
		"pod3": {},
		"pod5": {},
	}

	DeleteOutdatePod(w, visited)

	// Assert the updated workload groups
	expectedActualPods := map[string]*utils.Workload{
		"pod1": {
			InBps:    100,
			OutBps:   200,
			TotalBps: 300,
		},
	}
	if !reflect.DeepEqual(w.ActualPods, expectedActualPods) {
		t.Errorf("DeleteOutdatePod() failed, expected ActualPods: %v, got: %v", expectedActualPods, w.ActualPods)
	}

	expectedRequestPods := map[string]*utils.Workload{
		"pod3": {
			InBps:    200,
			OutBps:   300,
			TotalBps: 500,
		},
	}
	if !reflect.DeepEqual(w.RequestPods, expectedRequestPods) {
		t.Errorf("DeleteOutdatePod() failed, expected RequestPods: %v, got: %v", expectedRequestPods, w.RequestPods)
	}

	expectedLimitPods := map[string]*utils.Workload{
		"pod5": {
			InBps:    300,
			OutBps:   400,
			TotalBps: 700,
		},
	}
	if !reflect.DeepEqual(w.LimitPods, expectedLimitPods) {
		t.Errorf("DeleteOutdatePod() failed, expected LimitPods: %v, got: %v", expectedLimitPods, w.LimitPods)
	}
}

func Test_SelectPods(t *testing.T) {
	pRunPods := &RunPods{
		podParam: make(map[string]PodParam),
	}
	flag := 1
	devId := "test"
	policy := agent.Policy{
		Select_method: "All",
	}
	LimitPods := make(map[string]*utils.Workload)
	RequestPods := make(map[string]*utils.Workload)
	podList := []string{"pod1", "pod2", "pod3"}

	// Test case 1
	LimitPods["pod1"] = &utils.Workload{
		InBps:  100.0,
		OutBps: 200.0,
	}
	RequestPods["pod1"] = &utils.Workload{
		InBps:  50.0,
		OutBps: 100.0,
	}
	LimitPods["pod2"] = &utils.Workload{
		InBps:  300.0,
		OutBps: 400.0,
	}
	RequestPods["pod2"] = &utils.Workload{
		InBps:  150.0,
		OutBps: 200.0,
	}
	LimitPods["pod3"] = &utils.Workload{
		InBps:  500.0,
		OutBps: 600.0,
	}
	RequestPods["pod3"] = &utils.Workload{
		InBps:  250.0,
		OutBps: 300.0,
	}

	SelectPods(pRunPods, flag, devId, &policy, LimitPods, RequestPods, podList)

	// Verify the results
	expected := map[string]PodParam{
		"pod1": {100.0, LimitKeepValue, 50.0, 0},
		"pod2": {300.0, LimitKeepValue, 150.0, 0},
		"pod3": {500.0, LimitKeepValue, 250.0, 0},
	}

	if !reflect.DeepEqual(pRunPods.podParam, expected) {
		t.Errorf("SelectPods() failed, got %+v, want %+v", pRunPods.podParam, expected)
	}

	// Test case 2
	flag = 2
	pRunPods.podParam = make(map[string]PodParam)
	LimitPods = make(map[string]*utils.Workload)
	RequestPods = make(map[string]*utils.Workload)

	LimitPods["pod1"] = &utils.Workload{
		InBps:  100.0,
		OutBps: 200.0,
	}
	RequestPods["pod1"] = &utils.Workload{
		InBps:  50.0,
		OutBps: 100.0,
	}
	LimitPods["pod2"] = &utils.Workload{
		InBps:  300.0,
		OutBps: 400.0,
	}
	RequestPods["pod2"] = &utils.Workload{
		InBps:  150.0,
		OutBps: 200.0,
	}
	LimitPods["pod3"] = &utils.Workload{
		InBps:  500.0,
		OutBps: 600.0,
	}
	RequestPods["pod3"] = &utils.Workload{
		InBps:  250.0,
		OutBps: 300.0,
	}

	SelectPods(pRunPods, flag, devId, &policy, LimitPods, RequestPods, podList)

	// Verify the results
	expected = map[string]PodParam{
		"pod1": {200.0, LimitKeepValue, 100.0, 0},
		"pod2": {400.0, LimitKeepValue, 200.0, 0},
		"pod3": {600.0, LimitKeepValue, 300.0, 0},
	}

	if !reflect.DeepEqual(pRunPods.podParam, expected) {
		t.Errorf("SelectPods() failed, got %+v, want %+v", pRunPods.podParam, expected)
	}

	// Test case 3
	policy.Select_method = "NONE"
	pRunPods.podParam = make(map[string]PodParam)

	SelectPods(pRunPods, flag, devId, &policy, LimitPods, RequestPods, podList)

	// Verify the results
	if len(pRunPods.podParam) != 0 {
		t.Errorf("SelectPods() failed, expected empty podParam map")
	}
}

func TestCompressPods(t *testing.T) {
	w := &agent.WorkloadGroup{
		GroupSettable: false,
	}

	pRunPods := &RunPods{
		podParam: map[string]PodParam{
			"pod1": {100.0, LimitKeepValue, 50.0, 0},
			"pod2": {200.0, LimitKeepValue, 100.0, 0},
			"pod3": {300.0, LimitKeepValue, 150.0, 0},
		},
		expect: LimitMax,
	}

	podsNum := 3
	idx := utils.BeIndex

	policy := agent.Policy{
		Compress_method: "CALC_REQUEST_RATIO",
	}

	CompressPods(w, &policy, pRunPods, podsNum, idx)

	// Verify the results
	expectedPodParam := map[string]PodParam{
		"pod1": {100.0, LimitMax, 50.0, 0},
		"pod2": {200.0, LimitMax, 100.0, 0},
		"pod3": {300.0, LimitMax, 150.0, 0},
	}

	if !reflect.DeepEqual(pRunPods.podParam, expectedPodParam) {
		t.Errorf("CompressPods() failed, got %+v, want %+v", pRunPods.podParam, expectedPodParam)
	}
}

func TestCompressRequestRatio(t *testing.T) {
	pRunPods := &RunPods{
		podParam: map[string]PodParam{
			"pod1": {100.0, LimitKeepValue, 50.0, 0},
			"pod2": {200.0, LimitKeepValue, 100.0, 0},
			"pod3": {300.0, LimitKeepValue, 150.0, 0},
		},
		expect: LimitMax,
	}

	// Verify the results
	type PodParams map[string]PodParam

	expectedPods := map[int32]PodParams{
		0: map[string]PodParam{
			"pod1": {100.0, 50.0, 50.0, 0},
			"pod2": {200.0, 100.0 * LimitMax, 100.0, 0},
			"pod3": {300.0, 150.0 * LimitMax, 150.0, 0},
		},
		1: map[string]PodParam{
			"pod1": {100.0, 50.0 * LimitMax, 50.0, 0},
			"pod2": {200.0, 100.0 * LimitMax, 100.0, 0},
			"pod3": {300.0, 150.0 * LimitMax, 150.0, 0},
		},
		2: map[string]PodParam{
			"pod1": {100.0, 0.3333333333333333 * LimitMax, 50.0, 0},
			"pod2": {200.0, 100.0 * LimitMax, 100.0, 0},
			"pod3": {300.0, 150.0 * LimitMax, 150.0, 0},
		},
	}

	gSettable := false
	podsNum := 3

	for idx := utils.GaIndex; idx < utils.GroupIndex; idx++ {
		CompressRequestRatio(pRunPods, gSettable, podsNum, idx)

		if !reflect.DeepEqual(pRunPods.podParam["pod1"], expectedPods[idx]["pod1"]) {
			t.Errorf("CompressRequestRatio() failed, idx=%d got %+v, want %+v", idx, pRunPods.podParam, expectedPods[idx])
		}
	}
}

func Test_SetLimit(t *testing.T) {
	w := agent.WorkloadGroup{

		LimitPods:   make(map[string]*utils.Workload),
		RequestPods: make(map[string]*utils.Workload),
	}

	policy := agent.Policy{
		Throttle:          "POOL_SIZE",
		Select_method:     "All",
		Compress_method:   "CALC_REQUEST_RATIO",
		Decompress_method: "CALC_REQUEST_RATIO",
	}

	expect := utils.Workload{
		InBps:  100.0,
		OutBps: 100.0,
	}

	request := utils.Workload{
		InBps:  50.0,
		OutBps: 50.0,
	}

	dev := DevInfo{
		Id: "12:34:45:56:67:78",
	}

	idx := utils.GaIndex

	SetLimit(&w, &policy, expect, request, dev, idx)

	// Add assertions here to validate the behavior of SetLimit function
}

func TestComposeLimitData(t *testing.T) {
	type args struct {
		w        *agent.WorkloadGroup
		rRunPods RunPods
		wRunPods RunPods
		dev      DevInfo
		idx      int32
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Test ComposeLimitData",
			args: args{
				w: &agent.WorkloadGroup{
					ActualPods: map[string]*utils.Workload{
						"pod1": {
							InBps:  100.0,
							OutBps: 200.0,
						},
						"pod2": {
							InBps:  150.0,
							OutBps: 250.0,
						},
					},
					GroupSettable: true,
				},
				rRunPods: RunPods{
					podParam: map[string]PodParam{
						"pod1": {100.0, LimitKeepValue, 50.0, 0},
						"pod2": {200.0, LimitKeepValue, 100.0, 0},
					},
					flag: 1,
				},
				wRunPods: RunPods{
					podParam: map[string]PodParam{
						"pod1": {100.0, LimitKeepValue, 50.0, 0},
						"pod2": {200.0, LimitKeepValue, 100.0, 0},
					},
					flag: 1,
				},
				dev: DevInfo{
					Id:      "12:34:45:56:67:78",
					DevType: "NIC1",
				},
				idx: utils.BeIndex,
			},
		},
		{
			name: "Test ComposeLimitData 2",
			args: args{
				w: &agent.WorkloadGroup{
					RequestPods: map[string]*utils.Workload{
						"pod1": {
							InBps:  100.0,
							OutBps: 200.0,
						},
						"pod2": {
							InBps:  150.0,
							OutBps: 250.0,
						},
					},
					PodsLimitSet: map[string]*utils.Workload{
						"pod1": {
							InBps:  100.0,
							OutBps: 200.0,
						},
						"pod2": {
							InBps:  150.0,
							OutBps: 250.0,
						},
					},
					GroupSettable: true,
				},
				rRunPods: RunPods{
					podParam: map[string]PodParam{
						"pod1": {100.0, LimitKeepValue, 50.0, 0},
						"pod2": {200.0, LimitKeepValue, 100.0, 0},
					},
					flag: 1,
				},
				wRunPods: RunPods{
					podParam: map[string]PodParam{
						"pod1": {100.0, LimitKeepValue, 50.0, 0},
						"pod2": {200.0, LimitKeepValue, 100.0, 0},
					},
					flag: 1,
				},
				dev: DevInfo{
					Id:      "12:34:45:56:67:78",
					DevType: "NIC1",
				},
				idx: utils.BeIndex,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ComposeLimitData(tt.args.w, tt.args.rRunPods, tt.args.wRunPods, tt.args.dev, tt.args.idx)
		})
	}
}
