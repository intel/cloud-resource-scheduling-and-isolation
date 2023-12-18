/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/vishvananda/netlink"
)

func TestAdqPlugin_Configure(t *testing.T) {
	type fields struct {
		plugin CommonPlugin
		cni    CNI
		nic    string
		groups []Group
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
		{
			name:    "test",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdqPlugin{
				plugin: tt.fields.plugin,
				cni:    tt.fields.cni,
				nic:    tt.fields.nic,
				groups: tt.fields.groups,
			}
			if err := a.Configure(tt.args.groups); (err != nil) != tt.wantErr {
				t.Errorf("AdqPlugin.Configure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdqPlugin_RegisterApp(t *testing.T) {
	type fields struct {
		plugin CommonPlugin
		cni    CNI
		nic    string
		groups []Group
	}
	type args struct {
		app *AppInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "test",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		p := &AdqPlugin{}
		patches1 := gomonkey.ApplyMethod(reflect.TypeOf(p), "AddEgressFilter", func(_ *AdqPlugin, app *AppInfo) error {
			return nil
		})
		patches2 := gomonkey.ApplyMethod(reflect.TypeOf(p), "AddIngressFilter", func(_ *AdqPlugin, app *AppInfo) error {
			return nil
		})
		defer patches1.Reset()
		defer patches2.Reset()
		t.Run(tt.name, func(t *testing.T) {
			a := &AdqPlugin{
				plugin: tt.fields.plugin,
				cni:    tt.fields.cni,
				nic:    tt.fields.nic,
				groups: tt.fields.groups,
			}
			if err := a.RegisterApp(tt.args.app); (err != nil) != tt.wantErr {
				t.Errorf("AdqPlugin.RegisterApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdqPlugin_UnRegisterApp(t *testing.T) {
	type fields struct {
		plugin CommonPlugin
		cni    CNI
		nic    string
		groups []Group
	}
	type args struct {
		app *AppInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test",
			fields: fields{
				groups: []Group{
					{
						Id:        "g1",
						GroupType: GROUP_SHARE,
					},
				},
			},
			args: args{
				app: &AppInfo{
					Group: "g1",
					Devices: map[string]string{
						"dev1": "10.22.33.11",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdqPlugin{
				plugin: tt.fields.plugin,
				cni:    tt.fields.cni,
				nic:    tt.fields.nic,
				groups: tt.fields.groups,
			}
			p := &AdqPlugin{}
			patches1 := gomonkey.ApplyMethod(reflect.TypeOf(p), "DelADQFilter", func(_ *AdqPlugin, ip net.IP) error {
				return nil
			})
			patches2 := gomonkey.ApplyFunc(TCDeleteTbfHost, func(podNic string, ns string) error {
				return nil
			})
			patches3 := gomonkey.ApplyFunc(TCDeleteTbfContainer, func(podNic string, ns string) error {
				return nil
			})
			defer patches1.Reset()
			defer patches2.Reset()
			defer patches3.Reset()
			if err := a.UnRegisterApp(tt.args.app); (err != nil) != tt.wantErr {
				t.Errorf("AdqPlugin.UnRegisterApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdqPlugin_SetLimit(t *testing.T) {
	type fields struct {
		plugin CommonPlugin
		cni    CNI
		nic    string
		groups []Group
	}
	type args struct {
		group string
		app   *AppInfo
		limit BWLimit
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test1",
			fields: fields{
				groups: []Group{
					{
						Id:        "g1",
						GroupType: GROUP_SHARE,
					},
				},
			},
			args: args{
				app: &AppInfo{
					AppName: "app1",
					Group:   "g1",
					Devices: map[string]string{
						"dev1": "10.22.33.11",
					},
				},
				limit: BWLimit{
					Id:      "dev1",
					Ingress: "100",
					Egress:  "100",
				},
			},
			wantErr: false,
		},
		{
			name: "test2",
			fields: fields{
				groups: []Group{
					{
						Id:        "g1",
						GroupType: GROUP_SHARE,
					},
				},
			},
			args: args{
				app:   nil,
				group: "g1",
				limit: BWLimit{
					Id:      "dev1",
					Ingress: "100",
					Egress:  "100",
				},
			},
			wantErr: false,
		},
		{
			name: "test3",
			fields: fields{
				groups: []Group{
					{
						Id:        "g1",
						GroupType: GROUP_SHARE,
					},
				},
			},
			args: args{
				app: &AppInfo{
					AppName: "app1",
					Group:   "g1",
					Devices: map[string]string{
						"dev1": "10.22.33.11",
					},
				},
				limit: BWLimit{
					Id:      "dev1",
					Ingress: "1048576",
					Egress:  "1048576",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdqPlugin{
				plugin: tt.fields.plugin,
				cni:    tt.fields.cni,
				nic:    tt.fields.nic,
				groups: tt.fields.groups,
			}
			patches1 := gomonkey.ApplyFunc(getHostInterface, func(containerIfName string, namespace string) (netlink.Link, error) {
				return &netlink.Device{}, nil
			})
			patches2 := gomonkey.ApplyFunc(createTBF, func(rateInBits uint64, burstInBits uint64, linkIndex int) error {
				return nil
			})
			patches3 := gomonkey.ApplyFunc(addQiscInCnet, func(conif string, namespace string, limit uint64) error {
				return nil
			})
			patches4 := gomonkey.ApplyFunc(delQdiscIfExist, func(linkIndex int, tpe string) error {
				return nil
			})
			patches5 := gomonkey.ApplyFunc(TCDeleteTbfContainer, func(podNic string, namespace string) error {
				return fmt.Errorf("xx")
			})
			defer patches1.Reset()
			defer patches2.Reset()
			defer patches3.Reset()
			defer patches4.Reset()
			defer patches5.Reset()
			if err := a.SetLimit(tt.args.group, tt.args.app, tt.args.limit); (err != nil) != tt.wantErr {
				t.Errorf("AdqPlugin.SetLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

var groupSettable ConfigureRespInfo = ConfigureRespInfo{
	Groups: []GroupSettable{
		{
			Id:       "g1",
			Settable: false,
		},
		{
			Id:       "g2",
			Settable: false,
		},
		{
			Id:       "g3",
			Settable: false,
		},
	},
}

func TestNewAdqPlugin(t *testing.T) {
	type args struct {
		nic   string
		ip    string
		c     CNI
		group []Group
	}
	grpSettableStr, _ := json.Marshal(groupSettable)
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				group: []Group{
					{
						Id:        "g1",
						GroupType: GROUP_SEPARATE_LAZY,
					},
					{
						Id:        "g2",
						GroupType: GROUP_SEPARATE_DYNAMIC,
					},
					{
						Id:        "g3",
						GroupType: GROUP_SHARE,
					},
				},
			},
			want:    string(grpSettableStr),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := NewAdqPlugin(tt.args.nic, tt.args.ip, tt.args.c, tt.args.group)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAdqPlugin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NewAdqPlugin() got1 = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdqPlugin_DelADQFilter(t *testing.T) {
	type fields struct {
		plugin CommonPlugin
		cni    CNI
		nic    string
		groups []Group
	}
	type args struct {
		ip net.IP
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test",
			fields: fields{
				nic: "eth0",
				groups: []Group{
					{
						Id:        "g1",
						GroupType: GROUP_SHARE,
					},
				},
			},
			args: args{
				net.ParseIP("10.22.33.44"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdqPlugin{
				plugin: tt.fields.plugin,
				cni:    tt.fields.cni,
				nic:    tt.fields.nic,
				groups: tt.fields.groups,
			}
			if err := a.DelADQFilter(tt.args.ip); (err != nil) != tt.wantErr {
				t.Errorf("AdqPlugin.DelADQFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdqPlugin_AddEgressFilter(t *testing.T) {
	type fields struct {
		plugin CommonPlugin
		cni    CNI
		nic    string
		groups []Group
	}
	type args struct {
		app *AppInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "add egress filter",
			fields: fields{
				groups: []Group{
					{
						Id:        "123",
						GroupType: GROUP_SHARE,
					},
				},
			},
			args: args{
				app: &AppInfo{
					Group: "123",
					Devices: map[string]string{
						"eth0": "127.0.0.1",
					},
					AppName: "pod1",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			a := &AdqPlugin{
				plugin: tt.fields.plugin,
				cni:    tt.fields.cni,
				nic:    tt.fields.nic,
				groups: tt.fields.groups,
			}
			if err := a.AddEgressFilter(tt.args.app); (err != nil) != tt.wantErr {
				t.Errorf("AdqPlugin.AddEgressFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdqPlugin_AddIngressFilter(t *testing.T) {
	type fields struct {
		plugin CommonPlugin
		cni    CNI
		nic    string
		groups []Group
	}
	type args struct {
		app *AppInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "add ingress filter",
			fields: fields{
				groups: []Group{
					{
						Id:        "123",
						GroupType: GROUP_SHARE,
					},
				},
			},
			args: args{
				app: &AppInfo{
					Group: "123",
					Devices: map[string]string{
						"eth0": "127.0.0.1",
					},
					AppName: "pod1",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdqPlugin{
				plugin: tt.fields.plugin,
				cni:    tt.fields.cni,
				nic:    tt.fields.nic,
				groups: tt.fields.groups,
			}
			if err := a.AddIngressFilter(tt.args.app); (err != nil) != tt.wantErr {
				t.Errorf("AdqPlugin.AddIngressFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
