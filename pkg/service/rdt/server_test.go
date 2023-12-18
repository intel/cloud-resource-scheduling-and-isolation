/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
	"sigs.k8s.io/yaml"
)

func packAppInfo(rdtInfo utils.RDTAppInfo) string {
	data, err := json.Marshal(&rdtInfo)
	if err != nil {
		return ""
	}
	return string(data)
}

func unmarshalConfig(t *testing.T, rawconfig string) RDTConfig {
	config := RDTConfig{}
	err := yaml.Unmarshal([]byte(rawconfig), &config)
	if err != nil {
		t.Fatalf("failed to parse rdt config: %v", err)
	}
	return config
}

func TestRdtEngine_RegisterApp(t *testing.T) {
	type fields struct {
		apps   map[string]*AppInfo
		config RDTConfig
		Name   string
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
			name: "parse string failed",
			fields: fields{
				apps: make(map[string]*AppInfo),
			},
			args: args{
				req: &pb.RegisterAppRequest{
					AppName: "f9968b095fe236a3",
					AppInfo: "qqqq",
				},
			},
			want: &pb.RegisterAppResponse{
				AppId:  "f9968b095fe236a3",
				Status: pb.ResponseState_FAIL,
			},
			wantErr: true,
		},
		{
			name: "do not have any apps, and app name is empty",
			fields: fields{
				apps: make(map[string]*AppInfo),
			},
			args: args{
				req: &pb.RegisterAppRequest{
					AppName: "",
					AppInfo: packAppInfo(utils.RDTAppInfo{
						ContainerName: "container",
						CgroupPath:    "/sys/fs/cgroup",
						RDTClass:      "guaranteed",
					}),
				},
			},
			want: &pb.RegisterAppResponse{
				AppId:  "",
				Status: pb.ResponseState_FAIL,
			},
			wantErr: true,
		},
		{
			name: "do not have any apps, and container name is empty",
			fields: fields{
				apps: make(map[string]*AppInfo),
			},
			args: args{
				req: &pb.RegisterAppRequest{
					AppName: "f9968b095fe236a3",
					AppInfo: packAppInfo(utils.RDTAppInfo{
						ContainerName: "",
						CgroupPath:    "/sys/fs/cgroup",
						RDTClass:      "guaranteed",
					}),
				},
			},
			want: &pb.RegisterAppResponse{
				AppId:  "f9968b095fe236a3",
				Status: pb.ResponseState_FAIL,
			},
			wantErr: true,
		},
		{
			name: "do not have any apps, and cgroup path is empty",
			fields: fields{
				apps: make(map[string]*AppInfo),
			},
			args: args{
				req: &pb.RegisterAppRequest{
					AppName: "f9968b095fe236a3",
					AppInfo: packAppInfo(utils.RDTAppInfo{
						ContainerName: "container",
						CgroupPath:    "",
						RDTClass:      "guaranteed",
					}),
				},
			},
			want: &pb.RegisterAppResponse{
				AppId:  "f9968b095fe236a3",
				Status: pb.ResponseState_FAIL,
			},
			wantErr: true,
		},
		{
			name: "do not have any apps, and cgroup path is invalid",
			fields: fields{
				apps: make(map[string]*AppInfo),
			},
			args: args{
				req: &pb.RegisterAppRequest{
					AppName: "f9968b095fe236a3",
					AppInfo: packAppInfo(utils.RDTAppInfo{
						ContainerName: "container",
						CgroupPath:    "//\\\\",
						RDTClass:      "guaranteed",
					}),
				},
			},
			want: &pb.RegisterAppResponse{
				AppId:  "f9968b095fe236a3",
				Status: pb.ResponseState_FAIL,
			},
			wantErr: true,
		},
		{
			name: "do not have any apps, and cgroup path is a symbolic link",
			fields: fields{
				apps: make(map[string]*AppInfo),
			},
			args: args{
				req: &pb.RegisterAppRequest{
					AppName: "f9968b095fe236a3",
					AppInfo: packAppInfo(utils.RDTAppInfo{
						ContainerName: "container",
						CgroupPath:    "src",
						RDTClass:      "guaranteed",
					}),
				},
			},
			want: &pb.RegisterAppResponse{
				AppId:  "f9968b095fe236a3",
				Status: pb.ResponseState_FAIL,
			},
			wantErr: true,
		},
		{
			name: "do not have any apps, and rdt class is empty",
			fields: fields{
				apps: make(map[string]*AppInfo),
			},
			args: args{
				req: &pb.RegisterAppRequest{
					AppName: "f9968b095fe236a3",
					AppInfo: packAppInfo(utils.RDTAppInfo{
						ContainerName: "container",
						CgroupPath:    "/sys/fs/cgroup",
						RDTClass:      "",
					}),
				},
			},
			want: &pb.RegisterAppResponse{
				AppId:  "f9968b095fe236a3",
				Status: pb.ResponseState_FAIL,
			},
			wantErr: true,
		},
		{
			name: "app registration successful",
			fields: fields{
				apps: make(map[string]*AppInfo),
			},
			args: args{
				req: &pb.RegisterAppRequest{
					AppName: "f9968b095fe236a3",
					AppInfo: packAppInfo(utils.RDTAppInfo{
						ContainerName: "container",
						CgroupPath:    "/sys/fs/cgroup",
						RDTClass:      "guaranteed",
					}),
				},
			},
			want: &pb.RegisterAppResponse{
				AppId:  "f9968b095fe236a3",
				Status: pb.ResponseState_FAIL,
			},
			wantErr: true,
		},
		{
			name: "app already registered",
			fields: fields{
				apps: map[string]*AppInfo{
					"f9968b095fe236a3": {
						AppName: "f9968b095fe236a3",
						RdtAppInfo: utils.RDTAppInfo{
							ContainerName: "container",
							CgroupPath:    "/sys/fs/cgroup",
							RDTClass:      "guaranteed",
						},
					},
				},
			},
			args: args{
				req: &pb.RegisterAppRequest{
					AppName: "f9968b095fe236a3",
					AppInfo: packAppInfo(utils.RDTAppInfo{
						ContainerName: "container",
						CgroupPath:    "/sys/fs/cgroup",
						RDTClass:      "guaranteed",
					}),
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &RdtEngine{
				Apps:   tt.fields.apps,
				config: tt.fields.config,
			}
			got, err := s.RegisterApp(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("IoisolationServer.RegisterApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IoisolationServer.RegisterApp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRdtEngine_UnRegisterApp(t *testing.T) {
	type fields struct {
		apps   map[string]*AppInfo
		config RDTConfig
		Name   string
	}
	type args struct {
		ctx context.Context
		req *pb.UnRegisterAppRequest
	}

	pt1 := gomonkey.ApplyGlobalVar(&common.StoreDirectory, "../../build/")
	defer pt1.Reset()
	pt2 := gomonkey.ApplyGlobalVar(&common.StorePath, "../../../build/test2.db")
	defer pt2.Reset()
	db := common.NewDBPersist()
	err := db.CreateTable("RDT", []string{"APPS"})
	if err != nil {
		klog.Warningf("Create buckets fail: %s", err)
	}
	defer db.Close()
	defer os.Remove("../../../build/test2.db")

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.UnRegisterAppResponse
		wantErr bool
	}{
		{
			name: "app id is empty",
			fields: fields{
				apps: map[string]*AppInfo{
					"f9968b095fe236a3": {
						AppName: "f9968b095fe236a3",
						RdtAppInfo: utils.RDTAppInfo{
							ContainerName: "container",
							CgroupPath:    "/sys/fs/cgroup",
							RDTClass:      "guaranteed",
						},
					},
				},
			},
			args: args{
				req: &pb.UnRegisterAppRequest{
					AppId: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "app not found",
			fields: fields{
				apps: map[string]*AppInfo{
					"f9968b095fe236a3": {
						AppName: "f9968b095fe236a3",
						RdtAppInfo: utils.RDTAppInfo{
							ContainerName: "container",
							CgroupPath:    "/sys/fs/cgroup",
							RDTClass:      "guaranteed",
						},
					},
				},
			},
			args: args{
				req: &pb.UnRegisterAppRequest{
					AppId: "11111",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "app unregistration successful",
			fields: fields{
				apps: map[string]*AppInfo{
					"f9968b095fe236a3": {
						AppName: "f9968b095fe236a3",
						RdtAppInfo: utils.RDTAppInfo{
							ContainerName: "container",
							CgroupPath:    "/sys/fs/cgroup",
							RDTClass:      "guaranteed",
						},
					},
				},
			},
			args: args{
				req: &pb.UnRegisterAppRequest{
					AppId: "f9968b095fe236a3",
				},
			},
			want:    &pb.UnRegisterAppResponse{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &RdtEngine{
				Apps:    tt.fields.apps,
				config:  tt.fields.config,
				persist: db,
			}
			got, err := s.UnRegisterApp(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("RdtEngine.UnRegisterApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RdtEngine.UnRegisterApp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newMockMountsFile() error {
	baseDir, err := os.MkdirTemp("", "goresctrl.test.")
	if err != nil {
		return err
	}
	mountInfoPath = filepath.Join(baseDir, "mounts")
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	resctrlPath := filepath.Join(dir, "../../../test/rdtdata/resctrl")
	data := "resctrl " + resctrlPath + " resctrl rw,relatime 0 0\n"
	if err := os.WriteFile(mountInfoPath, []byte(data), 0644); err != nil {
		if err := os.RemoveAll(mountInfoPath); err != nil {
			return err
		}
		return err
	}
	return nil
}

func TestRdtEngine_checkRDTConfig(t *testing.T) {
	err := newMockMountsFile()
	if err != nil {
		t.Fatalf("failed to create new mock mounts file: %v", err)
	}
	if resctrlL3Info, err = getRdtL3Info(); err != nil {
		t.Fatalf("Failed to get rdt l3 info: %v", err)
	}
	type fields struct {
		apps   map[string]*AppInfo
		config RDTConfig
		Name   string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "check rdt config success",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			wantErr: false,
		},
		{
			name: "There is an error class in the classes",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guarant
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			wantErr: true,
		},
		{
			name: "adjuestment does not have threshold.",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			wantErr: true,
		},
		{
			name: "There is an error class in the cpuUtilization",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guarant: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			wantErr: true,
		},
		{
			name: "There is an error class in the L3CacheMiss",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      l3CacheMiss:
        guarant: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			wantErr: true,
		},
		{
			name: "There is an error class in the adjustmentResource",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
    adjustmentResource:
      l3Allocation:
        guarant: "0xfe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			wantErr: true,
		},
		{
			name: "There is an error bitmask in the adjustmentResource 1",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xf01"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			wantErr: true,
		},
		{
			name: "There is an error bitmask in the adjustmentResource 2",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xf10"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			wantErr: true,
		},
		{
			name: "There is an error bitmask in the adjustmentResource 3",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "fe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &RdtEngine{
				Apps:   tt.fields.apps,
				config: tt.fields.config,
			}
			if err := s.checkRDTConfig(); (err != nil) != tt.wantErr {
				t.Errorf("IoisolationServer.checkRDTConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRdtEngine_adjustResource(t *testing.T) {
	err := newMockMountsFile()
	if err != nil {
		t.Fatalf("failed to create new mock mounts file: %v", err)
	}
	if resctrlL3Info, err = getRdtL3Info(); err != nil {
		t.Fatalf("Failed to get rdt l3 info: %v", err)
	}
	type fields struct {
		apps   map[string]*AppInfo
		config RDTConfig
		Name   string
	}
	type args struct {
		adjustment string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "adjust resource success",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			args: args{
				adjustment: "adjustment1",
			},
			wantErr: false,
		},
		{
			name: "adjust resource failed",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xf10"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`,
				),
			},
			args: args{
				adjustment: "adjustment1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &RdtEngine{
				Apps:   tt.fields.apps,
				config: tt.fields.config,
			}
			if err := s.adjustResource(tt.args.adjustment); (err != nil) != tt.wantErr {
				t.Errorf("IoisolationServer.adjustResource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_createMsg(t *testing.T) {
	type args struct {
		ioiType int32
		data    map[string]float64
	}
	tests := []struct {
		name string
		args args
		want *pb.AppsBandwidth
	}{
		{
			name: "create msg failed 1",
			args: args{
				ioiType: 1,
				data: map[string]float64{
					"besteffort": 100,
					"burstable":  100,
					"guaranteed": 100,
				},
			},
			want: nil,
		},
		{
			name: "create msg failed 2",
			args: args{
				ioiType: 3,
				data:    nil,
			},
			want: &pb.AppsBandwidth{
				IoiType: 3,
				Payload: &pb.AppsBandwidth_RdtInfo{
					RdtInfo: &pb.RDTInfo{
						ClassInfo: nil,
					}},
			},
		},
		{
			name: "create msg success",
			args: args{
				ioiType: 3,
				data: map[string]float64{
					"besteffort": 100,
					"burstable":  100,
					"guaranteed": 100,
				},
			},
			want: &pb.AppsBandwidth{
				IoiType: 3,
				Payload: &pb.AppsBandwidth_RdtInfo{
					RdtInfo: &pb.RDTInfo{
						ClassInfo: map[string]float64{
							"besteffort": 100,
							"burstable":  100,
							"guaranteed": 100,
						},
					}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createMsg(tt.args.ioiType, tt.args.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createMsg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRdtEngine_getRDTclassScore(t *testing.T) {
	type fields struct {
		apps   map[string]*AppInfo
		config RDTConfig
		Name   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    map[string]float64
		wantErr bool
	}{
		{
			name: "get rdt class score success",
			fields: fields{
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`),
			},
			want: map[string]float64{
				"besteffort": 100,
				"burstable":  100,
				"guaranteed": 100,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &RdtEngine{
				Apps:   tt.fields.apps,
				config: tt.fields.config,
			}
			got, err := s.getRDTclassScore()
			if (err != nil) != tt.wantErr {
				t.Errorf("IoisolationServer.getRDTclassScore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IoisolationServer.getRDTclassScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRdtEngine_findAppropriateAdjustment(t *testing.T) {
	type fields struct {
		apps   map[string]*AppInfo
		config RDTConfig
		Name   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		want1   string
		wantErr bool
	}{
		{
			name: "find appropriate adjustment success",
			fields: fields{
				apps: map[string]*AppInfo{
					"f9968b095fe236a3": {
						AppName: "f9968b095fe236a3",
						RdtAppInfo: utils.RDTAppInfo{
							ContainerName: "container",
							CgroupPath:    getContainerGroupPath(),
							RDTClass:      "guaranteed",
						},
					},
				},
				config: unmarshalConfig(t,
					`
classes:
 - guaranteed
 - burstable
 - besteffort
updateInterval: 30
adjustments:
  adjustment1:
    threshold:
      cpuUtilization:
        guaranteed: 80
      l3CacheMiss:
        guaranteed: 110
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfe0"
        besteffort: "0x010"
  adjustment2:
    threshold:
      cpuUtilization:
        guaranteed: 50
    adjustmentResource:
      l3Allocation:
        guaranteed: "0xfc0"
        besteffort: "0x030"
`),
			},
			want:    false,
			want1:   "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &RdtEngine{
				Apps:   tt.fields.apps,
				config: tt.fields.config,
			}
			got, got1, err := s.findAppropriateAdjustment()
			if (err != nil) != tt.wantErr {
				t.Errorf("IoisolationServer.findAppropriateAdjustment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IoisolationServer.findAppropriateAdjustment() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("IoisolationServer.findAppropriateAdjustment() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
