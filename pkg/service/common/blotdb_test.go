/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/klog/v2"
)

func TestLoad(t *testing.T) {
	type args struct {
		bucketName string
		key        string
	}

	pt2 := gomonkey.ApplyGlobalVar(&StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := NewDBPersist()
	err := db.CreateTable("DISK", []string{"interval"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}

	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "bolt Load",
			args: args{
				bucketName: "DISK.interval",
				key:        "h1",
			},
			want: map[string]string{
				"h1": "hhb",
			},
			wantErr: false,
		},
	}

	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confBytes, err := json.Marshal(tt.want["h1"])
			if err != nil {
				klog.Errorf("could not marshal config json: %v", err)
			}
			if err = db.Save(tt.args.bucketName, tt.args.key, confBytes); (err != nil) != tt.wantErr {
				t.Errorf("addGroup() error = %v, wantErr %v", err, tt.wantErr)
			}

			got, err1 := db.Load(tt.args.bucketName, tt.args.key)
			if (err1 != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			var uconfString string
			err = json.Unmarshal(got["h1"], &uconfString)
			if err != nil {
				klog.Errorf("could not unmarshal config json: %v", err)
			}

			if !reflect.DeepEqual(uconfString, tt.want["h1"]) {
				t.Errorf("Load() = %v, want %v", uconfString, tt.want)
			}

			err = db.Delete(tt.args.bucketName, tt.args.key)
			if err != nil {
				klog.Errorf("could not delete config: %v", err)
			}
		})
	}
}
