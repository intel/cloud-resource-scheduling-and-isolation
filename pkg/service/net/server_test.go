/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/vishvananda/netlink"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
	"sigs.k8s.io/IOIsolation/pkg/service"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

func init() {
	common.StorePath = "../../../build/test.db"
}

func TestParseConfigureRequestString(t *testing.T) {
	tests := []struct {
		name      string
		configure string
		want      ConfigureInfo
		want1     error
	}{
		{
			name: "parse content is valid",
			configure: `{
    "cni": "flannel",
    "gateway": "10.243.0.1",
    "nodeips": [{
      "InterfaceName": "eth0",
      "IpAddr": "10.249.231.103", 
      "MacAddr": "test", 
      "InterfaceType": "test"
    }],
    "groups": [
      {
        "id": "1",
        "groupType": 0,
		"pool": 600,
        "queues": 4
      },
      {
        "id": "2",
		"groupType": 1,
		"pool": 400,
        "queues": 4
      }
    ]
   }`,
			want: ConfigureInfo{
				Cni:     "flannel",
				Gateway: "10.243.0.1",
				NodeIps: []NICInfo{{InterfaceName: "eth0", IpAddr: "10.249.231.103", MacAddr: "test", InterfaceType: "test"}},
				Groups:  []Group{{Id: "1", GroupType: 0, Pool: 600, Queues: 4}, {Id: "2", GroupType: 1, Pool: 400, Queues: 4}},
			},
			want1: nil,
		},
		{
			name:      "parse content is empty",
			configure: "",
			want:      ConfigureInfo{},
			want1:     errors.New("empty configure request"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfigureRequestString(tt.configure)
			fmt.Println(got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseConfigureRequestString() got = %v, want %v", got, tt.want)
			}
			t.Log(err)
		})
	}
}

func TestParseRegisterAppRequestString(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    RegisterAppInfo
		wantErr error
	}{
		{
			name: "parse content is valid",
			s: `{
			"group": "0",
			"netns": "netns_asdf",
			"devices" : {
				"eth0" : "10.243.3.1"		
			}
		}`,
			want: RegisterAppInfo{
				Group: "0",
				NetNS: "netns_asdf",
				Devices: map[string]string{
					"eth0": "10.243.3.1",
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRegisterAppRequestString(tt.s)
			if err != tt.wantErr {
				t.Errorf("ParseRegisterAppRequestString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRegisterAppRequestString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseServiceInfoRequestString(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    ServiceInfo
		wantErr bool
	}{
		{
			"Test1",
			args{"{\"ip\":\"10.244.0.55\"}"},
			ServiceInfo{"10.244.0.55"},
			false,
		},
		{
			"Test2",
			args{""},
			ServiceInfo{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseServiceInfoRequestString(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseServiceInfoRequestString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseServiceInfoRequestString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNetEngine_getCurBandwidth(t *testing.T) {
	type fields struct {
		ServiceEngine service.ServiceEngine
		si            ServerInfo
		option        Option
		groupSettable string
		apps          map[string]*AppInfo
		groups        []Group
		groupLimit    map[string]BWLimit
		nics          []NICInfo
		plugins       map[string]Plugin
		cnis          []CNI
		Interval      utils.IntervalConfigInfo
		prevMonitor   map[string]SR
		podsVethCache map[string][]Veth
	}
	type args struct {
		prev map[string]SR
		curr map[string]SR
		apps map[string]*AppInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string][]Veth
		wantErr bool
	}{
		{
			name: "Test1",
			fields: fields{
				podsVethCache: make(map[string][]Veth),
			},
			args: args{
				prev: map[string]SR{"cni0": {144444734, 123292732}},
				curr: map[string]SR{"cni0": {144874749, 123292987}},
				apps: map[string]*AppInfo{
					"app1": {AppName: "app1", Group: "2", NetNS: "/var/run/netns/cni-1", Devices: map[string]string{"eth0": "10.23.23.45"}},
				},
			},
			want:    map[string][]Veth{"app1": {Veth{"cni0", "eth0", SR{430015, 255}}}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := &netlink.GenericLink{}
			v1.Attrs().Name = "cni0"
			outputs := []gomonkey.OutputCell{
				{Values: gomonkey.Params{v1, nil}},
			}
			patches := gomonkey.ApplyFuncSeq(getHostInterface, outputs)
			defer patches.Reset()

			s := &NetEngine{
				ServiceEngine: tt.fields.ServiceEngine,
				si:            tt.fields.si,
				option:        tt.fields.option,
				groupSettable: tt.fields.groupSettable,
				Apps:          tt.fields.apps,
				groups:        tt.fields.groups,
				groupLimit:    tt.fields.groupLimit,
				nics:          tt.fields.nics,
				plugins:       tt.fields.plugins,
				cnis:          tt.fields.cnis,
				Interval:      tt.fields.Interval,
				prevMonitor:   tt.fields.prevMonitor,
				podsVethCache: tt.fields.podsVethCache,
			}
			got, err := s.getCurBandwidth(tt.args.prev, tt.args.curr, tt.args.apps)
			if (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.getCurBandwidth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NetEngine.getCurBandwidth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNetEngine_GetServiceInfo(t *testing.T) {
	type fields struct {
		ServiceEngine service.ServiceEngine
		si            ServerInfo
		option        Option
		groupSettable string
		apps          map[string]*AppInfo
		groups        []Group
		groupLimit    map[string]BWLimit
		nics          []NICInfo
		plugins       map[string]Plugin
		cnis          []CNI
		Interval      utils.IntervalConfigInfo
		prevMonitor   map[string]SR
		podsVethCache map[string][]Veth
	}
	type args struct {
		ctx context.Context
		req *pb.ServiceInfoRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.ServiceInfoResponse
		wantErr bool
	}{
		{
			name: "GetService Info for network",
			args: args{req: &pb.ServiceInfoRequest{
				IoiType: 0,
				Req:     "",
			}},
			want:    &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &NetEngine{
				ServiceEngine: tt.fields.ServiceEngine,
				si:            tt.fields.si,
				option:        tt.fields.option,
				groupSettable: tt.fields.groupSettable,
				Apps:          tt.fields.apps,
				groups:        tt.fields.groups,
				groupLimit:    tt.fields.groupLimit,
				nics:          tt.fields.nics,
				plugins:       tt.fields.plugins,
				cnis:          tt.fields.cnis,
				Interval:      tt.fields.Interval,
				prevMonitor:   tt.fields.prevMonitor,
				podsVethCache: tt.fields.podsVethCache,
			}
			got, err := s.GetServiceInfo(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.GetServiceInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NetEngine.GetServiceInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIoisolationServer_getSndRcv(t *testing.T) {
	type fields struct {
		ServiceEngine service.ServiceEngine
		si            ServerInfo
		option        Option
		groupSettable string
		apps          map[string]*AppInfo
		groups        []Group
		groupLimit    map[string]BWLimit
		nics          []NICInfo
		plugins       map[string]Plugin
		cnis          []CNI
		Interval      utils.IntervalConfigInfo
		prevMonitor   map[string]SR
		podsVethCache map[string][]Veth
	}
	tests := []struct {
		name    string
		fields  fields
		want    map[string]SR
		wantErr bool
	}{
		{
			name:    "test1",
			want:    map[string]SR{"enp89s0": {591756846, 1780143807}, "cni0": {144444734, 123292732}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		s := &NetEngine{
			ServiceEngine: tt.fields.ServiceEngine,
			si:            tt.fields.si,
			option:        tt.fields.option,
			groupSettable: tt.fields.groupSettable,
			Apps:          tt.fields.apps,
			groups:        tt.fields.groups,
			groupLimit:    tt.fields.groupLimit,
			nics:          tt.fields.nics,
			plugins:       tt.fields.plugins,
			cnis:          tt.fields.cnis,
			Interval:      tt.fields.Interval,
			prevMonitor:   tt.fields.prevMonitor,
			podsVethCache: tt.fields.podsVethCache,
		}
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.getSndRcv()
			if (err != nil) != tt.wantErr {
				t.Errorf("getSndRcv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNetEngine_SetAppLimit(t *testing.T) {
	type fields struct {
		ServiceEngine service.ServiceEngine
		persist       common.IPersist
		si            ServerInfo
		option        Option
		groupSettable string
		Apps          map[string]*AppInfo
		groups        []Group
		groupLimit    map[string]BWLimit
		nics          []NICInfo
		cnis          []CNI
		Interval      utils.IntervalConfigInfo
		plugins       map[string]Plugin
		prevMonitor   map[string]SR
		podsVethCache map[string][]Veth
	}
	type args struct {
		ctx context.Context
		req *pb.SetAppLimitRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.SetAppLimitResponse
		wantErr bool
	}{
		{
			name: "empty app id",
			fields: fields{
				Apps: map[string]*AppInfo{
					"app1": {
						AppName:    "app1",
						CgroupPath: "",
					},
				},
			},
			args: args{
				ctx: context.TODO(),
				req: &pb.SetAppLimitRequest{
					AppId: "",
					Limit: []*pb.BWLimit{
						{
							Id:  "123.123",
							In:  "30",
							Out: "30",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty BW limit",
			fields: fields{
				Apps: map[string]*AppInfo{
					"case": {
						AppName:    "case",
						CgroupPath: "",
					},
				},
			},
			args: args{
				req: &pb.SetAppLimitRequest{
					AppId: "0.case2",
					Limit: []*pb.BWLimit{},
				},
			},
			wantErr: true,
		},
		{
			name: "test3",
			fields: fields{
				Apps: map[string]*AppInfo{
					"case": {
						AppName:    "case",
						CgroupPath: "",
					},
				},
			},
			args: args{
				req: &pb.SetAppLimitRequest{
					AppId: "0.case",
					Limit: []*pb.BWLimit{{
						Id:  "",
						In:  "30",
						Out: "23",
					},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NetEngine{
				ServiceEngine: tt.fields.ServiceEngine,
				persist:       tt.fields.persist,
				si:            tt.fields.si,
				option:        tt.fields.option,
				groupSettable: tt.fields.groupSettable,
				Apps:          tt.fields.Apps,
				groups:        tt.fields.groups,
				groupLimit:    tt.fields.groupLimit,
				nics:          tt.fields.nics,
				cnis:          tt.fields.cnis,
				Interval:      tt.fields.Interval,
				plugins:       tt.fields.plugins,
				prevMonitor:   tt.fields.prevMonitor,
				podsVethCache: tt.fields.podsVethCache,
			}
			_, err := n.SetAppLimit(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.SetAppLimit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestNetEngine_RegisterApp(t *testing.T) {
	type fields struct {
		ServiceEngine service.ServiceEngine
		persist       common.IPersist
		si            ServerInfo
		option        Option
		groupSettable string
		Apps          map[string]*AppInfo
		groups        []Group
		groupLimit    map[string]BWLimit
		nics          []NICInfo
		cnis          []CNI
		Interval      utils.IntervalConfigInfo
		plugins       map[string]Plugin
		prevMonitor   map[string]SR
		podsVethCache map[string][]Veth
	}
	type args struct {
		ctx context.Context
		req *pb.RegisterAppRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.RegisterAppResponse
		wantErr bool
	}{
		{
			name:    "test register app",
			args:    args{req: &pb.RegisterAppRequest{AppInfo: ""}},
			want:    &pb.RegisterAppResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name: "app already exists",
			args: args{
				req: &pb.RegisterAppRequest{
					AppInfo: `{
						"group": "0",
						"netns": "netns_asdf",
						"devices" : {
							"eth0" : "",
							"eth1" : ""
						},
						"appname": "app1"
					}`,
					AppName: "app1",
				},
			},
			fields: fields{
				Apps: map[string]*AppInfo{
					"app1": {
						AppName: "app1",
						Group:   "0",
						NetNS:   "netns_asdf",
						Devices: map[string]string{
							"eth0": "",
							"eth1": "",
						},
					},
				},
			},
			want:    &pb.RegisterAppResponse{Status: pb.ResponseState_FAIL, AppId: "app1"},
			wantErr: true,
		},
		{
			name: "parse error",
			args: args{
				req: &pb.RegisterAppRequest{
					AppInfo: `{
						"group1"
					}`,
					AppName: "app1",
				},
			},
			fields: fields{
				Apps: map[string]*AppInfo{},
			},
			want:    &pb.RegisterAppResponse{Status: pb.ResponseState_FAIL, AppId: "app1"},
			wantErr: true,
		},
		{
			name: "group is nil",
			args: args{
				req: &pb.RegisterAppRequest{
					AppInfo: `{
						"group": "",
						"netns": "",
						"devices" : {
							"eth0" : "",
							"eth1" : ""
						},
						"cgrouppath": "test"
					}`,
					AppName: "app1",
				},
			},
			fields: fields{
				Apps: map[string]*AppInfo{},
			},
			want:    &pb.RegisterAppResponse{Status: pb.ResponseState_FAIL, AppId: "app1"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NetEngine{
				ServiceEngine: tt.fields.ServiceEngine,
				persist:       tt.fields.persist,
				si:            tt.fields.si,
				option:        tt.fields.option,
				groupSettable: tt.fields.groupSettable,
				Apps:          tt.fields.Apps,
				groups:        tt.fields.groups,
				groupLimit:    tt.fields.groupLimit,
				nics:          tt.fields.nics,
				cnis:          tt.fields.cnis,
				Interval:      tt.fields.Interval,
				plugins:       tt.fields.plugins,
				prevMonitor:   tt.fields.prevMonitor,
				podsVethCache: tt.fields.podsVethCache,
			}
			got, err := n.RegisterApp(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.RegisterApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NetEngine.RegisterApp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNetEngine_deleteAppInfo(t *testing.T) {
	type fields struct {
		ServiceEngine service.ServiceEngine
		persist       common.IPersist
		si            ServerInfo
		option        Option
		groupSettable string
		Apps          map[string]*AppInfo
		groups        []Group
		groupLimit    map[string]BWLimit
		nics          []NICInfo
		cnis          []CNI
		Interval      utils.IntervalConfigInfo
		plugins       map[string]Plugin
		prevMonitor   map[string]SR
		podsVethCache map[string][]Veth
	}
	type args struct {
		AppId string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "delete app for no apps",
			fields: fields{
				Apps: map[string]*AppInfo{},
			},
			args:    args{AppId: "app1"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NetEngine{
				ServiceEngine: tt.fields.ServiceEngine,
				persist:       tt.fields.persist,
				si:            tt.fields.si,
				option:        tt.fields.option,
				groupSettable: tt.fields.groupSettable,
				Apps:          tt.fields.Apps,
				groups:        tt.fields.groups,
				groupLimit:    tt.fields.groupLimit,
				nics:          tt.fields.nics,
				cnis:          tt.fields.cnis,
				Interval:      tt.fields.Interval,
				plugins:       tt.fields.plugins,
				prevMonitor:   tt.fields.prevMonitor,
				podsVethCache: tt.fields.podsVethCache,
			}
			if err := n.deleteAppInfo(tt.args.AppId); (err != nil) != tt.wantErr {
				t.Errorf("NetEngine.deleteAppInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
