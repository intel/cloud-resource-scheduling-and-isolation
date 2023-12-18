/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package csiengine

import (
	"testing"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
)

func TestCsiEngine_Uninitialize(t *testing.T) {
	type fields struct {
		csi map[string]IOICsi
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "test csi engine uninitialized",
			fields: fields{
				csi: make(map[string]IOICsi),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &CsiEngine{
				csi: tt.fields.csi,
			}
			if err := e.Uninitialize(); (err != nil) != tt.wantErr {
				t.Errorf("CsiEngine.Uninitialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCsiEngine_Initialize(t *testing.T) {
	type fields struct {
		csi map[string]IOICsi
	}
	type args struct {
		coreClient *kubernetes.Clientset
		client     *versioned.Clientset
		mtls       bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test csi engine initialized",
			fields: fields{
				csi: make(map[string]IOICsi),
			},
			args: args{
				coreClient: &kubernetes.Clientset{},
				client:     &versioned.Clientset{},
				mtls:       false,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &CsiEngine{
				csi: tt.fields.csi,
			}
			if err := e.Initialize(tt.args.coreClient, tt.args.client, tt.args.mtls); (err == nil) != tt.wantErr {
				t.Errorf("CsiEngine.Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
