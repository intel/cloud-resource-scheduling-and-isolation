/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"os"
	"path/filepath"
	"testing"
)

func getContainerGroupPath() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, "../../../test/rdtdata/containerCgroup")
	return path
}

func Test_getContainerCpuUtilization(t *testing.T) {
	cgroupControllerPath = getContainerGroupPath()
	type args struct {
		cgroupPath string
	}
	tests := []struct {
		name    string
		args    args
		want    float64
		wantErr bool
	}{
		{
			name: "cgroup v2",
			args: args{
				cgroupPath: getContainerGroupPath(),
			},
			want:    0,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getContainerCpuUtilization(tt.args.cgroupPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("getContainerCpuUtilization() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getContainerCpuUtilization() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getCgroupV1CpuTime(t *testing.T) {
	type args struct {
		cgroupPath string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "cgroup v1",
			args: args{
				cgroupPath: getContainerGroupPath(),
			},
			want:    3907906171712,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getCgroupV1CpuTime(tt.args.cgroupPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("getCgroupV1CpuTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getCgroupV1CpuTime() = %v, want %v", got, tt.want)
			}
		})
	}
}
