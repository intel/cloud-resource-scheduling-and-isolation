/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

var (
	iface *net.Interface
)

func TestMain(m *testing.M) {
	iface, err = GetDefaultGatewayInterface()
	if err != nil {
		return
	}
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test.db")
	defer pt2.Reset()
	//option := Option{EnableSubscribe: true}
	//srv, err = NewIoisolationServer(option)
	//defer srv.Close()
	defer os.Remove("../../../build/test.db")
	if err != nil {
		return
	}
	genPlug = &GenericPlugin{}
	genPlug.groups = []Group{{Id: "0", GroupType: GROUP_SEPARATE_LAZY}, {Id: "1", GroupType: GROUP_SEPARATE_DYNAMIC}, {Id: "2", GroupType: GROUP_SHARE}}
	m.Run()
}

func TestGetDefaultGatewayInterface(t *testing.T) {
	tests := []struct {
		name    string
		wantErr error
	}{
		{
			name: "Get host NIC info",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := GetDefaultGatewayInterface()
			if err == nil {
				fmt.Printf("nic name is %s\n", v.Name)
			}
		})
	}
}

func TestDetectBackendCilium(t *testing.T) {
	expectedBackend := "cilium_host"
	backend, err := DetectBackendCilium()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if backend != expectedBackend {
		t.Errorf("Expected backend to be %s, but got %s", expectedBackend, backend)
	}
}

func TestDetectCNI(t *testing.T) {
	t.Run("calico", func(t *testing.T) {
		DetectCNI()
	})

}
