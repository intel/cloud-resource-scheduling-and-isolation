/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"context"
	"os"
	"testing"

	"github.com/coreos/go-systemd/v22/dbus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"
	tu "sigs.k8s.io/IOIsolation/test/utils"
)

func Test_getNodeLabel(t *testing.T) {
	type args struct {
		client kubernetes.Interface
	}
	node1 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				"label1": "v1",
			},
		},
	}
	node2 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
			Labels: map[string]string{
				"label1":      "v1",
				"ioisolation": "disk",
			},
		},
	}
	client := fake.NewSimpleClientset()
	_, err := tu.MakeNode(client, node1)
	if err != nil {
		t.Error(err)
	}
	_, err = tu.MakeNode(client, node2)
	if err != nil {
		t.Error(err)
	}
	tests := []struct {
		name     string
		nodeName string
		args     args
		want     string
		wantErr  bool
	}{
		{
			name:     "test1",
			nodeName: "node1",
			args:     args{client: client},
			want:     "",
			wantErr:  true,
		},
		{
			name:     "test2",
			nodeName: "node2",
			args:     args{client: client},
			want:     "disk",
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("Node_Name", tt.nodeName)
			got, err := getNodeLabel(tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodeLabel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getNodeLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_setNodeLabel(t *testing.T) {
	type args struct {
		client kubernetes.Interface
	}
	os.Setenv("Node_Name", "node")
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node",
			Labels: map[string]string{
				"label":       "v1",
				"ioisolation": "rdt",
			},
		},
	}
	client := fake.NewSimpleClientset()
	_, err := tu.MakeNode(client, node)
	if err != nil {
		t.Error(err)
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name:    "test1",
			args:    args{client: client},
			want:    true,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := setNodeLabel(tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("setNodeLabel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("setNodeLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeLabelChanged(t *testing.T) {
	type args struct {
		client kubernetes.Interface
	}
	os.Setenv("Node_Name", "node")
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node",
			Labels: map[string]string{
				"label":       "v1",
				"ioisolation": "rdt",
			},
		},
	}
	client := fake.NewSimpleClientset()
	_, err := tu.MakeNode(client, node)
	if err != nil {
		t.Error(err)
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test1",
			args: args{client: client},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeLabelChanged(tt.args.client); got != tt.want {
				t.Errorf("nodeLabelChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkServiceRunning(t *testing.T) {
	type args struct {
		ctx      context.Context
		unitName string
		conn     *dbus.Conn
	}
	conn, err := dbus.NewWithContext(context.TODO())
	if err != nil {
		klog.Errorf("Failed to connect to systemd D-Bus: %v", err)
		return
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test1",
			args: args{
				ctx:      context.TODO(),
				unitName: "unitNameFake",
				conn:     conn,
			},
			want: false,
		},
		{
			name: "test2",
			args: args{
				ctx:      context.TODO(),
				unitName: "dbus.service",
				conn:     conn,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkServiceRunning(tt.args.ctx, tt.args.unitName, tt.args.conn); got != tt.want {
				t.Errorf("checkServiceRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}
