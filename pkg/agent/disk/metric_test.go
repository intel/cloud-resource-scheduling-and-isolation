/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

func TestDiskMetric_InitMetric(t *testing.T) {
	type fields struct {
		Cache    map[string]NodeDiskBw
		PodCache map[string]*PodBw
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "test init metric",
			fields: fields{
				Cache:    make(map[string]NodeDiskBw),
				PodCache: make(map[string]*PodBw),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &DiskMetric{
				Cache:    tt.fields.Cache,
				PodCache: tt.fields.PodCache,
			}
			e.InitMetric()
		})
	}
}

func TestDiskMetric_updatePodBwAll(t *testing.T) {
	type fields struct {
		Cache    map[string]NodeDiskBw
		PodCache map[string]*PodBw
	}
	type args struct {
		metric *prometheus.GaugeVec
		node   string
		disk   string
		pod    string
		wtype  string
		bs     string
		dif    *utils.Workload
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "test disk metric update pod bw all",
			fields: fields{
				Cache:    make(map[string]NodeDiskBw),
				PodCache: make(map[string]*PodBw),
			},
			args: args{
				metric: nil,
				node:   "node1",
				disk:   "disk1",
				pod:    "pod1",
				wtype:  "GA",
				bs:     "read",
				dif: &utils.Workload{
					InBps:  1,
					OutBps: 1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &DiskMetric{
				Cache:    tt.fields.Cache,
				PodCache: tt.fields.PodCache,
			}
			e.updatePodBwAll(tt.args.metric, tt.args.node, tt.args.disk, tt.args.pod, tt.args.wtype, tt.args.bs, tt.args.dif)
		})
	}
}
