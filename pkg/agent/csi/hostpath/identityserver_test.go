/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package hostpath

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
)

func Test_identityerver_GetPluginInfo(t *testing.T) {
	tests := []struct {
		name    string
		fields  *hostPath
		want    *csi.GetPluginInfoResponse
		wantErr string
	}{
		{
			name: "drivername is empty",
			fields: &hostPath{
				config: Config{},
			},
			want:    nil,
			wantErr: "Driver name not configured",
		},
		{
			name: "vendorversion is empty",
			fields: &hostPath{
				config: Config{
					DriverName: "localstorage.csi.k8s.io",
				},
			},
			want:    nil,
			wantErr: "Driver is missing version",
		},
		{
			name: "success",
			fields: &hostPath{
				config: Config{
					DriverName:    "localstorage.csi.k8s.io",
					VendorVersion: "0.1.0",
				},
			},
			want: &csi.GetPluginInfoResponse{
				Name:          "localstorage.csi.k8s.io",
				VendorVersion: "0.1.0",
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})
			if (err != nil) && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("identityServer.GetPluginInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("identityServer.GetPluginInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_identityerver_GetPluginCapabilities(t *testing.T) {

	caps := []*csi.PluginCapability{
		{
			Type: &csi.PluginCapability_Service_{
				Service: &csi.PluginCapability_Service{
					Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
				},
			},
		},
	}

	capWithTopology := append(caps, &csi.PluginCapability{
		Type: &csi.PluginCapability_Service_{
			Service: &csi.PluginCapability_Service{
				Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
			},
		},
	})
	tests := []struct {
		name   string
		fields *hostPath
		want   *csi.GetPluginCapabilitiesResponse
	}{
		{
			name: "enable topology",
			fields: &hostPath{
				config: Config{
					EnableTopology: true,
				},
			},
			want: &csi.GetPluginCapabilitiesResponse{Capabilities: capWithTopology},
		},
		{
			name: "disable topology",
			fields: &hostPath{
				config: Config{
					EnableTopology: false,
				},
			},
			want: &csi.GetPluginCapabilitiesResponse{Capabilities: caps},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := tt.fields.GetPluginCapabilities(context.Background(), &csi.GetPluginCapabilitiesRequest{})

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("identityServer.GetPluginCapabilities() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_identityerver_Probe(t *testing.T) {
	hp := &hostPath{}
	ret, _ := hp.Probe(context.Background(), &csi.ProbeRequest{})
	assert.Equal(t, &csi.ProbeResponse{}, ret)
}
