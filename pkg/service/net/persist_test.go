/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

func TestInterval(t *testing.T) {
	type args struct {
		d      *NetEngine
		config utils.IntervalConfigInfo
	}

	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("NET", []string{"test"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}
	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	tests := []struct {
		name    string
		args    args
		want    utils.IntervalConfigInfo
		wantErr bool
	}{
		{
			name: "test load and save interval normal",
			args: args{
				d: &NetEngine{
					persist: db,
				},
				config: utils.IntervalConfigInfo{
					Interval: 10,
				},
			},
			want:    utils.IntervalConfigInfo{Interval: 10},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SaveInterval(tt.args.d, tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("SaveInterval() error = %v, wantErr %v", err, tt.wantErr)
			}

			got, err1 := LoadInterval(tt.args.d)
			if (err1 != nil) != tt.wantErr {
				t.Errorf("LoadInterval() error = %v, wantErr %v", err1, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppInfo(t *testing.T) {
	type args struct {
		d       *NetEngine
		appInfo AppInfo
	}

	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("NET", []string{"APPS"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}
	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test app info",
			args: args{
				d: &NetEngine{
					persist: db,
				},
				appInfo: AppInfo{
					AppName: "test",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddAppInfo(tt.args.d, tt.args.appInfo); (err != nil) != tt.wantErr {
				t.Errorf("AddAppInfo() error = %v, wantErr %v", err, tt.wantErr)
			}

			allApps, err := GetAllAppInfos(tt.args.d, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllAppInfos() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(allApps) != 1 {
				t.Errorf("GetAllAppInfos() = %v, want %v", len(allApps), 1)
			}
			err = DeleteAppInfo(tt.args.d, "test")
			if err != nil {
				t.Errorf("DeleteAppInfo() error = %v", err)
			}
		})
	}
}

func TestGroup(t *testing.T) {
	type args struct {
		n     *NetEngine
		group Group
	}

	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("NET", []string{"GROUPS"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}
	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	tests := []struct {
		name    string
		args    args
		want    Group
		wantErr bool
	}{
		{
			name: "test app info",
			args: args{
				n: &NetEngine{
					persist: db,
				},
				group: Group{
					Id:        "test",
					GroupType: 0,
					Pool:      10,
					Queues:    4,
				},
			},
			want: Group{
				Id:        "test",
				GroupType: 0,
				Pool:      10,
				Queues:    4,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddGroup(tt.args.n, tt.args.group); (err != nil) != tt.wantErr {
				t.Errorf("AddGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
			got, err1 := GetGroups(tt.args.n)
			if (err1 != nil) != tt.wantErr {
				t.Errorf("AddGroup() error = %v, wantErr %v", err1, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got[0], tt.want) {
				t.Errorf("AddGroup() = %v, want %v", got[0], tt.want)
			}
		})
	}
}

func TestGroupLimit(t *testing.T) {
	type args struct {
		n     *NetEngine
		group string
		limit BWLimit
	}

	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("NET", []string{"GROUPLIMIT"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}
	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	tests := []struct {
		name    string
		args    args
		want    map[string]BWLimit
		wantErr bool
	}{
		{
			name: "test group limit info",
			args: args{
				n: &NetEngine{
					persist: db,
				},
				group: "test",
				limit: BWLimit{
					Id:      "test",
					Ingress: "100",
					Egress:  "100",
				},
			},
			want: map[string]BWLimit{
				"test": {
					Id:      "test",
					Ingress: "100",
					Egress:  "100",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddGroupLimit(tt.args.n, tt.args.group, tt.args.limit); (err != nil) != tt.wantErr {
				t.Errorf("AddGroupLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
			got, err1 := GetGroupLimit(tt.args.n)
			if (err1 != nil) != tt.wantErr {
				t.Errorf("AddGroupLimit() error = %v, wantErr %v", err1, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddGroupLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddGroupSettable(t *testing.T) {
	type args struct {
		n        *NetEngine
		groupSet string
	}

	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("NET", []string{"test"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}
	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test group settable",
			args: args{
				n:        &NetEngine{persist: db},
				groupSet: "test",
			},
			want:    "test",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddGroupSettable(tt.args.n, tt.args.groupSet); (err != nil) != tt.wantErr {
				t.Errorf("AddGroupSettable() error = %v, wantErr %v", err, tt.wantErr)
			}

			got, err1 := GetGroupSettable(tt.args.n)
			if (err1 != nil) != tt.wantErr {
				t.Errorf("GetGroupSettable() error = %v, wantErr %v", err1, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetGroupSettable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNic(t *testing.T) {
	type args struct {
		n   *NetEngine
		nic NICInfo
	}

	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("NET", []string{"NICS"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}
	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	tests := []struct {
		name    string
		args    args
		want    []NICInfo
		wantErr bool
	}{
		{
			name: "test nic info",
			args: args{
				n: &NetEngine{
					persist: db,
				},
				nic: NICInfo{
					InterfaceName: "test",
					InterfaceType: "main",
					IpAddr:        "192.168.0.1",
					MacAddr:       "00:00:00:00:00:00",
				},
			},
			want: []NICInfo{
				{
					InterfaceName: "test",
					InterfaceType: "main",
					IpAddr:        "192.168.0.1",
					MacAddr:       "00:00:00:00:00:00",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddNic(tt.args.n, tt.args.nic); (err != nil) != tt.wantErr {
				t.Errorf("AddNic() error = %v, wantErr %v", err, tt.wantErr)
			}

			got, err1 := GetNics(tt.args.n)
			if (err1 != nil) != tt.wantErr {
				t.Errorf("GetNics() error = %v, wantErr %v", err1, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetNics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCni(t *testing.T) {
	type args struct {
		n   *NetEngine
		cni CNI
	}

	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("NET", []string{"CNIS"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}
	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	tests := []struct {
		name    string
		args    args
		want    []CNI
		wantErr bool
	}{
		{
			name: "test cni info",
			args: args{
				n: &NetEngine{
					persist: db,
				},
				cni: CNI{
					Name:    "test",
					Gateway: "test",
				},
			},
			want: []CNI{
				{
					Name:    "test",
					Gateway: "test",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddCni(tt.args.n, tt.args.cni); (err != nil) != tt.wantErr {
				t.Errorf("AddCni() error = %v, wantErr %v", err, tt.wantErr)
			}

			got, err1 := GetCNIs(tt.args.n)
			if (err1 != nil) != tt.wantErr {
				t.Errorf("GetCNIs() error = %v, wantErr %v", err1, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCNIs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServerInfo(t *testing.T) {
	type args struct {
		n  *NetEngine
		si ServerInfo
	}

	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("NET", []string{"test"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}
	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	tests := []struct {
		name    string
		args    args
		want    ServerInfo
		wantErr bool
	}{
		{
			name: "test server info",
			args: args{
				n: &NetEngine{
					persist: db,
				},
				si: ServerInfo{
					ServeTime:  "11:20",
					Generation: 1,
				},
			},
			want: ServerInfo{
				ServeTime:  "11:20",
				Generation: 1,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddServerInfo(tt.args.n, tt.args.si); (err != nil) != tt.wantErr {
				t.Errorf("AddServerInfo() error = %v, wantErr %v", err, tt.wantErr)
			}

			got, err1 := GetServerInfo(tt.args.n)
			if (err1 != nil) != tt.wantErr {
				t.Errorf("GetServerInfo() error = %v, wantErr %v", err1, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetServerInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
