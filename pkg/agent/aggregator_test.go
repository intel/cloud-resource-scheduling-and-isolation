/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package agent

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	gomonkey "github.com/agiledragon/gomonkey/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	aggr "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	agent "sigs.k8s.io/IOIsolation/pkg/api/nodeagent"
)

func TestUpdateReservedPods(t *testing.T) {
	tests := []struct {
		name    string
		request *agent.UpdateReservedPodsRequest
		wantErr string
	}{
		{
			name: "unmatched node name",
			request: &agent.UpdateReservedPodsRequest{
				NodeName: "B",
			},
			wantErr: "the node name: B in request is not consistent with the one in AgentServer",
		},
		{
			name: "matched node name",
			request: &agent.UpdateReservedPodsRequest{
				NodeName: "A",
				ReservedPods: &aggr.PodList{
					Generation: 1,
				},
			},
			wantErr: "",
		},
	}

	srv := &AgentServer{
		Name: "A",
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("case: %v", tt.name), func(t *testing.T) {
			_, err := srv.UpdateReservedPods(context.TODO(), tt.request)
			if err != nil && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error: %q, but got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestProcessPodsFromAggregator(t *testing.T) {
	ioResource1 := &aggr.IOResourceRequest{
		IoType:       aggr.IOType_DiskIO,
		WorkloadType: utils.BeIndex,
		DevRequest: &aggr.DeviceRequirement{
			DevName: "disk1",
			Request: &aggr.Quantity{
				// Converted io request read/ingress bandwidth
				Inbps:   0.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
			Limit: &aggr.Quantity{
				Inbps:   0.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
			RawRequest: &aggr.Quantity{
				Inbps:   0.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
			RawLimit: &aggr.Quantity{
				Inbps:   0.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
		},
	}
	ioResource2 := &aggr.IOResourceRequest{
		IoType:       aggr.IOType_NetworkIO,
		WorkloadType: utils.GaIndex,
		DevRequest: &aggr.DeviceRequirement{
			DevName: "nic2",
			Request: &aggr.Quantity{
				// Converted io request read/ingress bandwidth
				Inbps:   300.0,
				Outbps:  300.0,
				Iniops:  300.0,
				Outiops: 300.0,
			},
			Limit: &aggr.Quantity{
				Inbps:   300.0,
				Outbps:  300.0,
				Iniops:  300.0,
				Outiops: 300.0,
			},
			RawRequest: &aggr.Quantity{
				Inbps:   300.0,
				Outbps:  300.0,
				Iniops:  300.0,
				Outiops: 300.0,
			},
			RawLimit: &aggr.Quantity{
				Inbps:   300.0,
				Outbps:  300.0,
				Iniops:  300.0,
				Outiops: 300.0,
			},
		},
	}
	ioResource3 := &aggr.IOResourceRequest{
		IoType:       aggr.IOType_RDT,
		WorkloadType: utils.GaIndex,
		DevRequest: &aggr.DeviceRequirement{
			DevName: "rdt",
			Request: &aggr.Quantity{
				// Converted io request read/ingress bandwidth
				Inbps:   0.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
			Limit: &aggr.Quantity{
				Inbps:   0.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
			RawRequest: &aggr.Quantity{
				Inbps:   0.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
			RawLimit: &aggr.Quantity{
				Inbps:   0.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
		},
	}

	ioResource4 := &aggr.IOResourceRequest{
		IoType:       aggr.IOType_RDTQuantity,
		WorkloadType: utils.GaIndex,
		DevRequest: &aggr.DeviceRequirement{
			DevName: "class1",
			Request: &aggr.Quantity{
				Inbps:   20.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
			Limit: &aggr.Quantity{
				Inbps:   30.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
			RawRequest: &aggr.Quantity{
				Inbps:   40.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
			RawLimit: &aggr.Quantity{
				Inbps:   50.0,
				Outbps:  0.0,
				Iniops:  0.0,
				Outiops: 0.0,
			},
		},
	}

	resPods := &aggr.PodList{
		Generation: 1,
		PodList: map[string]*aggr.PodRequest{
			"pod1": {
				Request: []*aggr.IOResourceRequest{ioResource1, ioResource2, ioResource3, ioResource4},
			},
		},
	}

	expectedDiskUpdate := &PodData{
		T:          2,
		Generation: resPods.Generation,
		PodEvents: map[string]PodEventInfo{
			"disk1": {
				"pod1": PodRunInfo{
					Request: &utils.Workload{
						InBps:   0.0,
						OutBps:  0.0,
						InIops:  0.0,
						OutIops: 0.0,
					},
					Limit: &utils.Workload{
						InBps:   0.0,
						OutBps:  0.0,
						InIops:  0.0,
						OutIops: 0.0,
					},
					RawRequest: &utils.Workload{
						InBps:   0.0,
						OutBps:  0.0,
						InIops:  0.0,
						OutIops: 0.0,
					},
					RawLimit: &utils.Workload{
						InBps:   0.0,
						OutBps:  0.0,
						InIops:  0.0,
						OutIops: 0.0,
					},
					WorkloadType: utils.BeIndex,
				},
			},
		},
	}
	expectedNetUpdate := &PodData{
		T:          4,
		Generation: resPods.Generation,
		PodEvents: map[string]PodEventInfo{
			"nic2": {
				"pod1": PodRunInfo{
					Request: &utils.Workload{
						InBps:   300.0,
						OutBps:  300.0,
						InIops:  300.0,
						OutIops: 300.0,
					},
					Limit: &utils.Workload{
						InBps:   300.0,
						OutBps:  300.0,
						InIops:  300.0,
						OutIops: 300.0,
					},
					RawRequest: &utils.Workload{
						InBps:   300.0,
						OutBps:  300.0,
						InIops:  300.0,
						OutIops: 300.0,
					},
					RawLimit: &utils.Workload{
						InBps:   300.0,
						OutBps:  300.0,
						InIops:  300.0,
						OutIops: 300.0,
					},
					WorkloadType: utils.GaIndex,
				},
			},
		},
	}
	expectedRdtUpdate := &PodData{
		5,
		resPods.Generation,
		make(map[string]PodEventInfo),
	}

	expectedRdtQuantityUpdate := &PodData{
		T:          6,
		Generation: resPods.Generation,
		PodEvents: map[string]PodEventInfo{
			"class1": {
				"pod1": PodRunInfo{
					Request: &utils.Workload{
						InBps:   20.0,
						OutBps:  0.0,
						InIops:  0.0,
						OutIops: 0.0,
					},
					Limit: &utils.Workload{
						InBps:   30.0,
						OutBps:  0.0,
						InIops:  0.0,
						OutIops: 0.0,
					},
					RawRequest: &utils.Workload{
						InBps:   40.0,
						OutBps:  0.0,
						InIops:  0.0,
						OutIops: 0.0,
					},
					RawLimit: &utils.Workload{
						InBps:   50.0,
						OutBps:  0.0,
						InIops:  0.0,
						OutIops: 0.0,
					},
					WorkloadType: utils.GaIndex,
				},
			},
		},
	}

	t.Run("test data from aggregator", func(t *testing.T) {
		diskUpdate, netUpdate, rdtUpdate, rdtQuantityUpdate := ProcessPodsFromAggregator(resPods)

		if !reflect.DeepEqual(diskUpdate, expectedDiskUpdate) {
			t.Errorf("diskUpdate = %+v, want %+v", diskUpdate, expectedDiskUpdate)
		}

		if !reflect.DeepEqual(netUpdate, expectedNetUpdate) {
			t.Errorf("netUpdate = %+v, want %+v", netUpdate, expectedNetUpdate)
		}

		if !reflect.DeepEqual(rdtUpdate, expectedRdtUpdate) {
			t.Errorf("rdtUpdate = %v, want %v", rdtUpdate, expectedRdtUpdate)
		}

		if !reflect.DeepEqual(rdtQuantityUpdate, expectedRdtQuantityUpdate) {
			t.Errorf("rdtQuantityUpdate = %v, want %v", rdtQuantityUpdate, expectedRdtQuantityUpdate)
		}
	})
}

func TestGetCertServer(t *testing.T) {
	expectedCACertFile := "root-ca.crt"
	expectedAgentKeyFile := "/etc/ioi/control/pki/na-server.key"
	expectedAgentCertFile := "/etc/ioi/control/pki/na-server.crt"

	actualCACertFile, actualAgentKeyFile, actualAgentCertFile := GetCertServer()

	if actualCACertFile != expectedCACertFile {
		t.Errorf("CACertFile = %s, want %s", actualCACertFile, expectedCACertFile)
	}

	if actualAgentKeyFile != expectedAgentKeyFile {
		t.Errorf("AgentKeyFile = %s, want %s", actualAgentKeyFile, expectedAgentKeyFile)
	}

	if actualAgentCertFile != expectedAgentCertFile {
		t.Errorf("AgentCertFile = %s, want %s", actualAgentCertFile, expectedAgentCertFile)
	}
}

func TestGetAgentServer(t *testing.T) {
	nodeName := "test-node"
	expectedServer := &AgentServer{
		Name: nodeName,
	}

	actualServer := GetAgentServer(nodeName)

	if !reflect.DeepEqual(actualServer, expectedServer) {
		t.Errorf("GetAgentServer() = %+v, want %+v", actualServer, expectedServer)
	}
}

func TestGetReservedPods(t *testing.T) {
	type args struct {
		nodeName string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs := []gomonkey.OutputCell{
				{Values: gomonkey.Params{&aggr.GetReservedPodsResponse{}, nil}},
			}

			patches := gomonkey.ApplyFuncSeq(client.GetReservedPods, outputs)

			GetReservedPods(tt.args.nodeName)

			patches.Reset()

		})
	}
}

func TestUpdateNodeIOStatus(t *testing.T) {
	type args struct {
		nodeName string
		IO       *aggr.IOBandwidth
		gen      int64
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test update node status",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UpdateNodeIOStatus(tt.args.nodeName, tt.args.IO, tt.args.gen); (err != nil) != tt.wantErr {
				t.Errorf("UpdateNodeIOStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
