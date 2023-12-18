/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package agent

import (
	"testing"

	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
)

func TestAgent_InitStaticCR(t *testing.T) {
	type fields struct {
		engines       map[string]DataEngine
		ehs           []EventHandler
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		IOInfo        v1.NodeStaticIOInfo
		EnginesSwitch int
		conn          *grpc.ClientConn
		mtls          bool
	}

	ag := GetAgent()

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:    "init static cr",
			fields:  fields{engines: ag.engines, ehs: ag.ehs},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Agent{
				engines:        tt.fields.engines,
				ehs:            tt.fields.ehs,
				CoreClient:     tt.fields.CoreClient,
				Clientset:      tt.fields.Clientset,
				IOInfo:         tt.fields.IOInfo,
				EnginesSwitch:  tt.fields.EnginesSwitch,
				aggregatorConn: tt.fields.conn,
				mtls:           tt.fields.mtls,
			}
			if err := c.InitStaticCR(); (err != nil) != tt.wantErr {
				t.Errorf("Agent.InitStaticCR() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckEngine(t *testing.T) {
	type args struct {
		engine int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "check engine",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckEngine(tt.args.engine); got != tt.want {
				t.Errorf("CheckEngine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgent_InitClient(t *testing.T) {
	type fields struct {
		engines       map[string]DataEngine
		ehs           []EventHandler
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		IOInfo        v1.NodeStaticIOInfo
		EnginesSwitch int
		conn          *grpc.ClientConn
		mtls          bool
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Agent{
				engines:        tt.fields.engines,
				ehs:            tt.fields.ehs,
				CoreClient:     tt.fields.CoreClient,
				Clientset:      tt.fields.Clientset,
				IOInfo:         tt.fields.IOInfo,
				EnginesSwitch:  tt.fields.EnginesSwitch,
				aggregatorConn: tt.fields.conn,
				mtls:           tt.fields.mtls,
			}
			if err := c.InitClient(); (err != nil) != tt.wantErr {
				t.Errorf("Agent.InitClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgent_Stop(t *testing.T) {
	type fields struct {
		engines       map[string]DataEngine
		ehs           []EventHandler
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		IOInfo        v1.NodeStaticIOInfo
		EnginesSwitch int
		conn          *grpc.ClientConn
		mtls          bool
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "stop engine",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Agent{
				engines:        tt.fields.engines,
				ehs:            tt.fields.ehs,
				CoreClient:     tt.fields.CoreClient,
				Clientset:      tt.fields.Clientset,
				IOInfo:         tt.fields.IOInfo,
				EnginesSwitch:  tt.fields.EnginesSwitch,
				aggregatorConn: tt.fields.conn,
				mtls:           tt.fields.mtls,
			}
			if err := c.Stop(); (err != nil) != tt.wantErr {
				t.Errorf("Agent.Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgent_RegisterEventHandler(t *testing.T) {
	type fields struct {
		engines       map[string]DataEngine
		ehs           []EventHandler
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		IOInfo        v1.NodeStaticIOInfo
		EnginesSwitch int
		conn          *grpc.ClientConn
		mtls          bool
	}
	type args struct {
		ch        chan EventData
		key       string
		f         EventFunc
		msg       string
		engine    EventCacheEngine
		needCache bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Register EventHandler in agent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Agent{
				engines:        tt.fields.engines,
				ehs:            tt.fields.ehs,
				CoreClient:     tt.fields.CoreClient,
				Clientset:      tt.fields.Clientset,
				IOInfo:         tt.fields.IOInfo,
				EnginesSwitch:  tt.fields.EnginesSwitch,
				aggregatorConn: tt.fields.conn,
				mtls:           tt.fields.mtls,
			}
			c.RegisterEventHandler(tt.args.ch, tt.args.key, tt.args.f, tt.args.msg, tt.args.engine, tt.args.needCache)
		})
	}
}

func TestAgent_startClient4Aggregator(t *testing.T) {
	type fields struct {
		engines       map[string]DataEngine
		ehs           []EventHandler
		CoreClient    *kubernetes.Clientset
		Clientset     *versioned.Clientset
		IOInfo        v1.NodeStaticIOInfo
		EnginesSwitch int
		conn          *grpc.ClientConn
		mtls          bool
	}
	type args struct {
		nodeName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "start client for aggregator",
			fields: fields{
				mtls: false,
			},
			args: args{
				nodeName: "test-node",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Agent{
				engines:        tt.fields.engines,
				ehs:            tt.fields.ehs,
				CoreClient:     tt.fields.CoreClient,
				Clientset:      tt.fields.Clientset,
				IOInfo:         tt.fields.IOInfo,
				EnginesSwitch:  tt.fields.EnginesSwitch,
				aggregatorConn: tt.fields.conn,
				mtls:           tt.fields.mtls,
			}
			err := c.StartClient4Aggregator(tt.args.nodeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("startClient4Aggregator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
