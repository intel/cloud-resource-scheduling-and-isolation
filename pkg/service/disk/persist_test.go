/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

func TestSaveInterval(t *testing.T) {
	type args struct {
		d      *DiskEngine
		config utils.IntervalConfigInfo
	}

	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("DISK", []string{"test"})
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
				d: &DiskEngine{
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

func TestAddAppInfo(t *testing.T) {
	type args struct {
		d       *DiskEngine
		appInfo AppInfo
	}

	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("DISK", []string{"APPS"})
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
				d: &DiskEngine{
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
