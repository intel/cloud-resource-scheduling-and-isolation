/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
)

var (
	genPlug *GenericPlugin
	err     error
)

func TestGenericPlugin_Configure(t *testing.T) {
	type fields struct {
		Plugin  CommonPlugin
		HostNic string
		cni     CNI
		groups  []Group
	}
	type args struct {
		groups []Group
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GenericPlugin{
				Plugin:  tt.fields.Plugin,
				HostNic: tt.fields.HostNic,
				cni:     tt.fields.cni,
				groups:  tt.fields.groups,
			}
			if err := g.Configure(tt.args.groups); (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenericPlugin_ConfigureEgressGroup(t *testing.T) {
	type args struct {
		group Group
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"Test1",
			args{Group{"2", 1, 400, 4}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op1 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{1500, nil}},
			}
			pt1 := gomonkey.ApplyFuncSeq(getMTU, op1)
			defer pt1.Reset()
			op2 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{nil}},
			}
			pt2 := gomonkey.ApplyFuncSeq(createIfb, op2)
			defer pt2.Reset()
			op3 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{nil}},
			}
			pt3 := gomonkey.ApplyFuncSeq(addIfbLimit, op3)
			defer pt3.Reset()
			if err := genPlug.ConfigureEgressGroup(tt.args.group); (err != nil) != tt.wantErr {
				t.Errorf("ConfigureEgressGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenericPlugin_SetLimit(t *testing.T) {
	type args struct {
		group string
		app   *AppInfo
		limit BWLimit
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"Test1",
			args{group: "0", app: nil},
			false,
		},
		{
			"Test2",
			args{group: "2", app: &AppInfo{AppName: "app1"}},
			false,
		},
		{
			"Test3",
			args{group: "2", limit: BWLimit{"1", "300", "300"}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op1 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{nil}},
				{Values: gomonkey.Params{nil}},
			}
			p1 := gomonkey.ApplyFuncSeq(delQdiscIfExist, op1)
			defer p1.Reset()
			op2 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{nil}},
				{Values: gomonkey.Params{nil}},
			}
			p2 := gomonkey.ApplyFuncSeq(addIfbLimit, op2)
			defer p2.Reset()
			if err := genPlug.SetLimit(tt.args.group, tt.args.app, tt.args.limit); (err != nil) != tt.wantErr {
				t.Errorf("SetLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
