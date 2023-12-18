/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

func TestSaveLogLevel(t *testing.T) {
	type args struct {
		persist IPersist
		config  utils.LogLevelConfigInfo
	}
	pt2 := gomonkey.ApplyGlobalVar(&StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := NewDBPersist()
	err := db.CreateTable("COMMON", []string{})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}

	tests := []struct {
		name    string
		args    args
		want    utils.LogLevelConfigInfo
		wantErr bool
	}{
		{
			name: "TestSaveLogLevel",
			args: args{
				persist: db,
				config:  utils.LogLevelConfigInfo{Info: true, Debug: true},
			},
			want:    utils.LogLevelConfigInfo{Info: true, Debug: true},
			wantErr: false,
		},
	}

	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SaveLogLevel(tt.args.persist, tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("SaveLogLevel() error = %v, wantErr %v", err, tt.wantErr)
			}

			got, err := LoadLogLevel(tt.args.persist)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadLogLevel() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
