/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent/metrics"
)

func TestNetMetric_updateNetBwAll(t *testing.T) {
	NetRequestSumBw.Reset()
	type args struct {
		metric *prometheus.GaugeVec
		node   string
		nic    string
		nbw    []*utils.Workload
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test case 1",
			args: args{
				metric: NetRequestSumBw,
				node:   "node1",
				nic:    "nic1",
				nbw: []*utils.Workload{
					{OutBps: 10, InBps: 20},
					{OutBps: 30, InBps: 40},
					{OutBps: 50, InBps: 60},
				},
			},
		},
		// Add more test cases here
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBw := make(map[string]PodInfos)
			limBw := make(map[string]PodInfos)
			tranBw := make(map[string]PodInfos)

			e := &NetMetric{
				Cache:    make(map[string]NodeNetBw),
				PodCache: make(map[string]*PodBw),
			}

			e.Cache["node1"] = NodeNetBw{
				"test": &NetBw{
					NetRequestBw: []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
					NetAllocBw:   []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
					NetTotalBw:   []*utils.Workload{{InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}},
				},
			}
			e.PodCache["node1"] = &PodBw{
				PodRequestBw: reqBw,
				PodLimitBw:   limBw,
				PodTranBw:    tranBw,
			}

			// Fix: Remove the extra comma after the closing brace of the assignment
			e.updateNetBwAll(tt.args.metric, tt.args.node, tt.args.nic, tt.args.nbw)
			// Add assertions here to verify the expected behavior
		})
	}
}

func TestNetMetric_updateNetBw(t *testing.T) {
	type fields struct {
		// Initialize the fields of the NetMetric struct here
	}
	type args struct {
		data  *prometheus.GaugeVec
		node  string
		nic   string
		wtype string
		rwsum string
		bw    float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test case 1",
			args: args{
				data:  NetRequestSumBw,
				node:  "node1",
				nic:   "nic1",
				wtype: "write",
				rwsum: "sum",
				bw:    100.0,
			},
		},
		// Add more test cases here
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &NetMetric{
				// Initialize the fields of the NetMetric struct here
			}

			e.updateNetBw(tt.args.data, tt.args.node, tt.args.nic, tt.args.wtype, tt.args.rwsum, tt.args.bw)
			// Add assertions here to verify the expected behavior
		})
	}
}

func TestNetMetric_updatePodBw(t *testing.T) {
	type fields struct {
		// Initialize the fields of the NetMetric struct here
	}
	type args struct {
		data  *prometheus.GaugeVec
		node  string
		nic   string
		pod   string
		wtype string
		rw    string
		bw    float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test case 1",
			args: args{
				data:  NetPodRequestBw,
				node:  "node1",
				nic:   "nic1",
				pod:   "pod1",
				wtype: "write",
				rw:    "sum",
				bw:    100.0,
			},
		},
		// Add more test cases here
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &NetMetric{
				// Initialize the fields of the NetMetric struct here
			}

			e.updatePodBw(tt.args.data, tt.args.node, tt.args.nic, tt.args.pod, tt.args.wtype, tt.args.rw, tt.args.bw)
			// Add assertions here to verify the expected behavior
		})
	}
}
func TestNetMetric_InitMetric(t *testing.T) {
	e := &NetMetric{}
	metrics.GetMetricEngine().PrometheusReg = prometheus.NewRegistry()

	tests := []struct {
		name string
	}{
		{
			name: "test case 1",
		},
		// Add more test cases here
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.InitMetric()
			// Add assertions here to verify the expected behavior
		})
	}
}

func TestNetMetric_FlushData(t *testing.T) {
	e := &NetMetric{
		Cache:    make(map[string]NodeNetBw),
		PodCache: make(map[string]*PodBw),
	}

	// Initialize the Cache and PodCache with test data
	e.Cache["node1"] = NodeNetBw{
		"test": &NetBw{
			NetRequestBw: []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
			NetAllocBw:   []*utils.Workload{{InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}, {InBps: 10, OutBps: 10, TotalBps: 10}},
			NetTotalBw:   []*utils.Workload{{InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}, {InBps: 100, OutBps: 100, TotalBps: 100}},
		},
	}
	e.PodCache["node1"] = &PodBw{
		PodRequestBw: make(map[string]PodInfos),
		PodLimitBw:   make(map[string]PodInfos),
		PodTranBw:    make(map[string]PodInfos),
	}

	// Add test data to PodRequestBw, PodLimitBw, and PodTranBw
	e.PodCache["node1"].PodRequestBw["nic1"] = PodInfos{
		"pod1": PodInfo{
			PodName:  "pod1",
			Type:     "type1",
			Workload: &utils.Workload{OutBps: 10, InBps: 20},
		},
	}
	e.PodCache["node1"].PodLimitBw["nic1"] = PodInfos{
		"pod1": PodInfo{
			PodName:  "pod1",
			Type:     "type1",
			Workload: &utils.Workload{OutBps: 50, InBps: 60},
		},
	}
	e.PodCache["node1"].PodTranBw["nic1"] = PodInfos{
		"pod1": PodInfo{
			PodName:  "pod1",
			Type:     "type1",
			Workload: &utils.Workload{OutBps: 90, InBps: 100},
		},
	}
	e.PodCache["node1"].PodRequestBw["nic2"] = PodInfos{
		"pod2": PodInfo{
			PodName:  "pod2",
			Type:     "type2",
			Workload: &utils.Workload{OutBps: 30, InBps: 40},
		},
	}
	e.PodCache["node1"].PodLimitBw["nic2"] = PodInfos{
		"pod2": PodInfo{
			PodName:  "pod2",
			Type:     "type2",
			Workload: &utils.Workload{OutBps: 70, InBps: 80},
		},
	}
	e.PodCache["node1"].PodTranBw["nic2"] = PodInfos{
		"pod2": PodInfo{
			PodName:  "pod2",
			Type:     "type2",
			Workload: &utils.Workload{OutBps: 110, InBps: 120},
		},
	}

	// Call the FlushData method
	e.FlushData()

	// Add assertions here to verify the expected behavior

	// Add more assertions to verify the expected behavior
}
