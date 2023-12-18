/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package engines

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"sigs.k8s.io/IOIsolation/pkg/agent"
)

func TestPolicyEngine_AnalysisPolicyConfig(t *testing.T) {
	plp := &corev1.ConfigMap{
		Data: map[string]string{
			"DiskGroup": "DiskGArules:Policy1\nDiskBTrules:Policy1\nDiskBErules:Policy1",
			"NetGroup":  "NetGArules:Policy1\nNetBTrules:Policy1\nNetBErules:Policy1",
			"Policy1":   "Throttle:POOL_SIZE\nSelect_method:All\nCompress_method:CALC_REQUEST_RATIO\nDecompress_method:CALC_REQUEST_RATIO\nReport_Flag:GUARANTEE_ELASTICITY\n",
		},
	}
	pc := &agent.PolicyConfig{
		"DiskBTrules": agent.Policy{
			Throttle:          "POOL_SIZE",
			Select_method:     "All",
			Compress_method:   "CALC_REQUEST_RATIO",
			Decompress_method: "CALC_REQUEST_RATIO",
			Report_Flag:       "GUARANTEE_ELASTICITY",
		},
		"DiskGArules": agent.Policy{
			Throttle:          "POOL_SIZE",
			Select_method:     "All",
			Compress_method:   "CALC_REQUEST_RATIO",
			Decompress_method: "CALC_REQUEST_RATIO",
			Report_Flag:       "GUARANTEE_ELASTICITY",
		},
		"NetBTrules": agent.Policy{
			Throttle:          "POOL_SIZE",
			Select_method:     "All",
			Compress_method:   "CALC_REQUEST_RATIO",
			Decompress_method: "CALC_REQUEST_RATIO",
			Report_Flag:       "GUARANTEE_ELASTICITY",
		},
		"NetGArules": agent.Policy{
			Throttle:          "POOL_SIZE",
			Select_method:     "All",
			Compress_method:   "CALC_REQUEST_RATIO",
			Decompress_method: "CALC_REQUEST_RATIO",
			Report_Flag:       "GUARANTEE_ELASTICITY",
		},
	}

	type args struct {
		pl *corev1.ConfigMap
	}
	tests := []struct {
		name string
		args args
		want *agent.PolicyConfig
	}{
		{
			name: "test get policy config",
			args: args{pl: plp},
			want: pc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &PolicyEngine{}
			got := e.AnalysisPolicyConfig(tt.args.pl)
			if !reflect.DeepEqual((*got)["NetGArules"].Throttle, (*tt.want)["NetGArules"].Throttle) {
				t.Errorf("PolicyEngine.AnalysisPolicyConfig() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual((*got)["NetGArules"].Select_method, (*tt.want)["NetGArules"].Select_method) {
				t.Errorf("PolicyEngine.AnalysisPolicyConfig() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual((*got)["NetGArules"].Compress_method, (*tt.want)["NetGArules"].Compress_method) {
				t.Errorf("PolicyEngine.AnalysisPolicyConfig() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual((*got)["NetGArules"].Decompress_method, (*tt.want)["NetGArules"].Decompress_method) {
				t.Errorf("PolicyEngine.AnalysisPolicyConfig() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual((*got)["NetGArules"].Report_Flag, (*tt.want)["NetGArules"].Report_Flag) {
				t.Errorf("PolicyEngine.AnalysisPolicyConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyEngine_Uninitialize(t *testing.T) {
	tests := []struct {
		name    string
		e       *PolicyEngine
		wantErr bool
	}{
		{
			name:    "test policy engine uninitialize",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &PolicyEngine{}
			if err := e.Uninitialize(); (err != nil) != tt.wantErr {
				t.Errorf("PolicyEngine.Uninitialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPolicyEngine_CreatePolicyFactory(t *testing.T) {
	tests := []struct {
		name string
		p    *PolicyEngine
		want informers.SharedInformerFactory
	}{
		{
			name: "test start policy config",
			p:    &PolicyEngine{},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.CreatePolicyFactory()
			if got == tt.want {
				t.Errorf("PolicyEngine.StartPolicyConfig() fail")
			}
		})
	}
}
