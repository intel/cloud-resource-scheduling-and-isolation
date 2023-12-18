/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package scheduler

import (
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/networkio"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/util"
)

func TestMostAllocatedScorer_Score(t *testing.T) {
	type fields struct {
		resourceType util.ResourceType
	}
	type args struct {
		state *stateData
		rh    resource.Handle
	}
	rh := networkio.New()
	c := fake.NewSimpleClientset()
	err := rh.Run(resource.NewExtendedCache(), nil, c)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int64
		wantErr bool
	}{
		{
			name: "calculate score",
			fields: fields{
				resourceType: "NetworkIO",
			},
			args: args{
				state: &stateData{
					nodeSupportIOI: true,
				},
				rh: rh,
			},
			want:    10,
			wantErr: false,
		},
		{
			name: "calculate score max",
			fields: fields{
				resourceType: "NetworkIO",
			},
			args: args{
				state: &stateData{
					nodeSupportIOI: false,
				},
				rh: rh,
			},
			want:    100,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := &MostAllocatedScorer{
				resourceType: tt.fields.resourceType,
			}
			h := &networkio.Handle{}
			patches := gomonkey.ApplyMethod(reflect.TypeOf(h), "NodePressureRatio", func(_ *networkio.Handle, x interface{}, y interface{}) (float64, error) {
				return 0.1, nil
			})
			defer patches.Reset()
			got, err := scorer.Score(tt.args.state, tt.args.rh)
			if (err != nil) != tt.wantErr {
				t.Errorf("MostAllocatedScorer.Score() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MostAllocatedScorer.Score() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLeastAllocatedScorer_Score(t *testing.T) {
	type fields struct {
		resourceType util.ResourceType
	}
	type args struct {
		state *stateData
		rh    resource.Handle
	}
	rh := networkio.New()
	c := fake.NewSimpleClientset()
	err := rh.Run(resource.NewExtendedCache(), nil, c)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int64
		wantErr bool
	}{
		{
			name: "calculate score",
			fields: fields{
				resourceType: "NetworkIO",
			},
			args: args{
				state: &stateData{
					nodeSupportIOI: true,
				},
				rh: rh,
			},
			want:    90,
			wantErr: false,
		},
		{
			name: "calculate score max",
			fields: fields{
				resourceType: "NetworkIO",
			},
			args: args{
				state: &stateData{
					nodeSupportIOI: false,
				},
				rh: rh,
			},
			want:    100,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := &LeastAllocatedScorer{
				resourceType: tt.fields.resourceType,
			}
			h := &networkio.Handle{}
			patches := gomonkey.ApplyMethod(reflect.TypeOf(h), "NodePressureRatio", func(_ *networkio.Handle, x interface{}, y interface{}) (float64, error) {
				return 0.1, nil
			})
			defer patches.Reset()
			got, err := scorer.Score(tt.args.state, tt.args.rh)
			if (err != nil) != tt.wantErr {
				t.Errorf("LeastAllocatedScorer.Score() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("LeastAllocatedScorer.Score() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getScorer(t *testing.T) {
	type args struct {
		scoreStrategy util.ScoreStrategyType
		resourceType  util.ResourceType
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "wrong resource type",
			args: args{
				resourceType: "xxx",
			},
			wantErr: true,
		},
		{
			name: "MostAllocated",
			args: args{
				resourceType:  "NetworkIO",
				scoreStrategy: util.MostAllocated,
			},
			wantErr: false,
		},
		{
			name: "LeastAllocated",
			args: args{
				resourceType:  "NetworkIO",
				scoreStrategy: util.LeastAllocated,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getScorer(tt.args.scoreStrategy, tt.args.resourceType)
			if (err != nil) != tt.wantErr {
				t.Errorf("getScorer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
