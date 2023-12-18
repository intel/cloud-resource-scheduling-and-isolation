/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"reflect"
	"testing"
)

func TestCommonPlugin_Configure(t *testing.T) {
	type args struct {
		groups []Group
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "configure",
			args: args{groups: make([]Group, 0)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &CommonPlugin{}
			if err := g.Configure(tt.args.groups); (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommonPlugin_RegisterApp(t *testing.T) {
	type args struct {
		app *AppInfo
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Register App ",
			args: args{app: nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &CommonPlugin{}
			if err := g.RegisterApp(tt.args.app); (err != nil) != tt.wantErr {
				t.Errorf("RegisterApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommonPlugin_SetLimit(t *testing.T) {
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
			name: "SetLimit",
			args: args{
				group: "",
				app:   nil,
				limit: BWLimit{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &CommonPlugin{}
			if err := g.SetLimit(tt.args.group, tt.args.app, tt.args.limit); (err != nil) != tt.wantErr {
				t.Errorf("SetLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommonPlugin_UnRegisterApp(t *testing.T) {
	type args struct {
		app *AppInfo
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "UnRegisterApp",
			args: args{app: nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &CommonPlugin{}
			if err := g.UnRegisterApp(tt.args.app); (err != nil) != tt.wantErr {
				t.Errorf("UnRegisterApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCommonPlugin(t *testing.T) {
	tests := []struct {
		name string
		want CommonPlugin
	}{
		{
			name: "NewCommonPlugin",
			want: CommonPlugin{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewCommonPlugin(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewCommonPlugin() = %v, want %v", got, tt.want)
			}
		})
	}
}
