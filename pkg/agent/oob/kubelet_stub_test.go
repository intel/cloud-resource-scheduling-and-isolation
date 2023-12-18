/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package oob

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/agiledragon/gomonkey/v2"
	v1 "k8s.io/api/core/v1"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestNewKubeletStub(t *testing.T) {
	type args struct {
		addr    string
		port    int
		scheme  string
		timeout time.Duration
	}
	tests := []struct {
		name    string
		args    args
		want    KubeletStub
		wantErr bool
	}{
		{
			"Test1",
			args{"node1", 10250, "http", 30 * time.Second},
			&kubeletStub{
				httpClient: &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{[]byte{}}}}, RootCAs: x509.NewCertPool()}}},
				addr:       "node1",
				port:       10250,
				scheme:     "http",
				token:      "",
			},
			false,
		},
		{
			"Test2",
			args{"", 0, "http", time.Second},
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Test1" {
				op1 := []gomonkey.OutputCell{
					{Values: gomonkey.Params{[]byte{}, nil}},
					{Values: gomonkey.Params{[]byte{}, nil}},
				}
				pt1 := gomonkey.ApplyFuncSeq(os.ReadFile, op1)
				defer pt1.Reset()
			} else if tt.name == "Test2" {
				pt2 := gomonkey.ApplyGlobalVar(&defaultKubeletCertFile, "/opt/ioi/abc.crt")
				defer pt2.Reset()
			}
			got, err := NewKubeletStub(tt.args.addr, tt.args.port, tt.args.scheme, tt.args.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKubeletStub() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewKubeletStub() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_kubeletStub_GetAllPods(t *testing.T) {
	type fields struct {
		addr       string
		port       int
		scheme     string
		httpClient *http.Client
		token      string
	}
	tests := []struct {
		name    string
		fields  fields
		want    v1.PodList
		wantErr bool
	}{
		{
			"Test1",
			fields{"node1", 10250, "http", &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{[]byte{}}}}, RootCAs: x509.NewCertPool()}}}, ""},
			v1.PodList{},
			true,
		},
		{
			"Test2",
			fields{"node1", 10250, "http", &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{[]byte{}}}}, RootCAs: x509.NewCertPool()}}}, ""},
			v1.PodList{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := kubeletStub{
				addr:       tt.fields.addr,
				port:       tt.fields.port,
				scheme:     tt.fields.scheme,
				httpClient: tt.fields.httpClient,
				token:      tt.fields.token,
			}
			if tt.name == "Test2" {
				op1 := []gomonkey.OutputCell{
					{Values: gomonkey.Params{&http.Response{StatusCode: http.StatusBadRequest}, nil}},
				}
				pt1 := gomonkey.ApplyMethodSeq(k.httpClient, "Do", op1)
				defer pt1.Reset()
			}
			got, err := k.GetAllPods()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllPods() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAllPods() got = %v, want %v", got, tt.want)
			}
		})
	}
}
