/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
	"sigs.k8s.io/IOIsolation/pkg/service"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

var loglevel = utils.LogLevelConfigInfo{
	Info:  true,
	Debug: true,
}

func TestNewJsonStruct(t *testing.T) {
	tests := []struct {
		name string
		want *JsonStruct
	}{
		{
			name: "test json",
			want: &JsonStruct{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewJsonStruct(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewJsonStruct() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiskEngine_RegisterApp(t *testing.T) {
	type fields struct {
		ServiceEngine  service.ServiceEngine
		Apps           map[string]*AppInfo
		ContainerToPod map[uint64]string
		EbpfBuckets    map[uint64]uint64
		Interval       utils.IntervalConfigInfo
		EbpfEnabled    bool
		prevRawData    *RawData
	}
	type args struct {
		ctx context.Context
		req *pb.RegisterAppRequest
	}

	diskCgroupInfo := utils.DiskCgroupInfo{
		CgroupPath: "/sys/fs/cgroup/test",
		DeviceId:   []string{"eth0", "eth1"},
	}

	data, jsonErr := json.Marshal(diskCgroupInfo)
	if jsonErr != nil {
		klog.Error("Marshal is failed: ", jsonErr)
		return
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.RegisterAppResponse
		wantErr bool
	}{
		{
			name:    "test1",
			args:    args{req: &pb.RegisterAppRequest{AppInfo: ""}},
			want:    &pb.RegisterAppResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name: "App already registed",
			args: args{req: &pb.RegisterAppRequest{AppInfo: "app1", AppName: "app1"}},
			fields: fields{
				Apps: map[string]*AppInfo{
					"app1": {
						AppName: "app1",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "the device id is not valid",
			args: args{
				req: &pb.RegisterAppRequest{
					AppInfo: `{"cgrouppath": ""}`,
				},
			},
			want:    &pb.RegisterAppResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name: "the cgroup path is not exist",
			args: args{
				req: &pb.RegisterAppRequest{
					AppInfo: string(data),
					AppName: "app1",
				},
			},
			want:    &pb.RegisterAppResponse{Status: pb.ResponseState_FAIL, AppId: "app1"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DiskEngine{
				ServiceEngine:  tt.fields.ServiceEngine,
				Apps:           tt.fields.Apps,
				Interval:       tt.fields.Interval,
				ContainerToPod: tt.fields.ContainerToPod,
				EbpfBuckets:    tt.fields.EbpfBuckets,
				EbpfEnabled:    tt.fields.EbpfEnabled,
				prevRawData:    tt.fields.prevRawData,
			}
			got, err := s.RegisterApp(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.RegisterApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DiskEngine.RegisterApp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiskEngine_UnRegisterApp(t *testing.T) {
	type fields struct {
		ServiceEngine  service.ServiceEngine
		Apps           map[string]*AppInfo
		Interval       utils.IntervalConfigInfo
		ContainerToPod map[uint64]string
		EbpfBuckets    map[uint64]uint64
		EbpfEnabled    bool
		prevRawData    *RawData
	}
	type args struct {
		ctx context.Context
		req *pb.UnRegisterAppRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.UnRegisterAppResponse
		wantErr bool
	}{
		{
			name:    "test1",
			args:    args{req: &pb.UnRegisterAppRequest{AppId: "1"}},
			fields:  fields{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DiskEngine{
				ServiceEngine:  tt.fields.ServiceEngine,
				Apps:           tt.fields.Apps,
				ContainerToPod: tt.fields.ContainerToPod,
				EbpfBuckets:    tt.fields.EbpfBuckets,
				Interval:       tt.fields.Interval,
				EbpfEnabled:    tt.fields.EbpfEnabled,
				prevRawData:    tt.fields.prevRawData,
			}
			got, err := s.UnRegisterApp(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.UnRegisterApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DiskEngine.UnRegisterApp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_diskCreateMsgEbpf(t *testing.T) {
	type args struct {
		ioiType int32
		data    map[string]RW
	}
	data1 := make(map[string]RW)
	var rw RW
	r := make(map[string]bdata)
	r["testDisk"] = []float64{1, 1, 1, 1, 1, 1, 1, 1, 1}
	w := make(map[string]bdata)
	w["testDisk"] = []float64{1, 1, 1, 1, 1, 1, 1, 1, 1}
	rw.R = r
	rw.W = w
	data1["testPod"] = rw
	tests := []struct {
		name string
		args args
		want *pb.AppsBandwidth
	}{
		{
			name: "test1",
			args: args{ioiType: 1, data: data1},
			want: &pb.AppsBandwidth{
				IoiType: 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := diskCreateMsgEbpf(tt.args.ioiType, tt.args.data); !reflect.DeepEqual(got.IoiType, tt.want.IoiType) {
				t.Errorf("createMsg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_diskCreateMsgCgroup(t *testing.T) {
	type args struct {
		ioiType int32
		data    map[string]RW
	}
	data1 := make(map[string]RW)
	var rw RW
	r := make(map[string]bdata)
	r["testDisk"] = []float64{1, 1, 1, 1, 1, 1}
	w := make(map[string]bdata)
	w["testDisk"] = []float64{1, 1, 1, 1, 1, 1}
	rw.R = r
	rw.W = w
	data1["testPod"] = rw
	tests := []struct {
		name string
		args args
		want *pb.AppsBandwidth
	}{
		{
			name: "test1",
			args: args{ioiType: 1, data: data1},
			want: &pb.AppsBandwidth{
				IoiType: 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := diskCreateMsgCgroup(tt.args.ioiType, tt.args.data); !reflect.DeepEqual(got.IoiType, tt.want.IoiType) {
				t.Errorf("createMsg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDiskServiceInfoRequestString(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    []DiskServiceInfo
		wantErr bool
	}{
		{
			name:    "test1",
			want:    []DiskServiceInfo{},
			wantErr: true,
		},
		{
			name:    "test2",
			args:    args{s: "test string"},
			want:    []DiskServiceInfo{},
			wantErr: true,
		},
		{
			name:    "test3",
			args:    args{s: "test string"},
			want:    []DiskServiceInfo{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		if tt.name == "test2" {
			p1 := gomonkey.ApplyFunc(json.Unmarshal, func(data []byte, v any) error {
				return errors.New("test")
			})
			defer p1.Reset()
		} else if tt.name == "test3" {
			p1 := gomonkey.ApplyFunc(json.Unmarshal, func(data []byte, v any) error {
				return nil
			})
			defer p1.Reset()
		}
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDiskServiceInfoRequestString(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDiskServiceInfoRequestString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseDiskServiceInfoRequestString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPartSize(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "test1",
		},
	}
	for _, tt := range tests {
		if tt.name == "test1" {
			p1 := gomonkey.ApplyFunc(utils.RunBash, func(v1 string) (string, error) {
				return "0 0 0 0 0 0 0 0 0 0 0", errors.New("test")
			})
			defer p1.Reset()
			p2 := gomonkey.ApplyFunc(strconv.ParseInt, func(s string, base int, bitSize int) (i int64, err error) {
				return 0, errors.New("test2")
			})
			defer p2.Reset()
		}
		t.Run(tt.name, func(t *testing.T) {
			if got := getPartSize(tt.args.path); got != tt.want {
				t.Errorf("getPartSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_judgePartProfile(t *testing.T) {
	type args struct {
		mnt string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test1",
			args: args{mnt: "/boot/"},
			want: false,
		},
		{
			name: "test2",
			args: args{mnt: "test"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := judgePartProfile(tt.args.mnt); got != tt.want {
				t.Errorf("judgePartProfile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getADiskInfo(t *testing.T) {
	type args struct {
		diskName string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string][]utils.BlockDevice
		wantErr bool
	}{
		{
			name: "test1",
		},
		{
			name:    "test2",
			wantErr: true,
		},
		{
			name:    "test3",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		if tt.name == "test1" {
			p1 := gomonkey.ApplyFunc(utils.RunBash, func(v1 string) (string, error) {
				return "", nil
			})
			defer p1.Reset()
			p2 := gomonkey.ApplyFunc(json.Unmarshal, func(data []byte, v any) error {
				return nil
			})
			defer p2.Reset()
		} else if tt.name == "test2" {
			p1 := gomonkey.ApplyFunc(utils.RunBash, func(v1 string) (string, error) {
				return "", errors.New("test err")
			})
			defer p1.Reset()
		} else {
			p1 := gomonkey.ApplyFunc(utils.RunBash, func(v1 string) (string, error) {
				return "", nil
			})
			defer p1.Reset()
			p2 := gomonkey.ApplyFunc(json.Unmarshal, func(data []byte, v any) error {
				return errors.New("test err")
			})
			defer p2.Reset()
		}
		t.Run(tt.name, func(t *testing.T) {
			got, err := getADiskInfo(tt.args.diskName)
			if (err != nil) != tt.wantErr {
				t.Errorf("getADiskInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getADiskInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDiskType(t *testing.T) {
	type args struct {
		id int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test1",
			args: args{id: 0},
			want: "emptyDir",
		},
		{
			name: "test2",
			args: args{id: 1},
			want: "non-emptyDir",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDiskType(tt.args.id); got != tt.want {
				t.Errorf("getDiskType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkDiskDetails(t *testing.T) {
	type args struct {
		diskList []string
	}
	tests := []struct {
		name string
		args args
		want map[string]utils.BlockDevice
	}{
		{
			name: "test1",
			want: map[string]utils.BlockDevice{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkDiskDetails(tt.args.diskList); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("checkDiskDetails() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getMapPath(t *testing.T) {
	type args struct {
		mnt string
		dk  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test1",
			args: args{mnt: "/"},
			want: "/dir_",
		},
		{
			name: "test2",
			args: args{mnt: "test"},
			want: "test/dir_",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getMapPath(tt.args.mnt, tt.args.dk); got != tt.want {
				t.Errorf("getMapPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getAllDiskInfo(t *testing.T) {
	tests := []struct {
		name    string
		want    map[string][]utils.BlockDevice
		wantErr bool
	}{
		{
			name:    "test all diskinfos",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getAllDiskInfo()
			if (err != nil) != tt.wantErr {
				t.Errorf("getAllDiskInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestConvertData(t *testing.T) {
	type args struct {
		bps  float64
		iops float64
		data []float64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test convert data bs < 512",
			args: args{
				bps:  123.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data bs > 512 and bs < 1024",
			args: args{
				bps:  1023.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data bs = 1024",
			args: args{
				bps:  1024.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data bs > 1024 and bs < 4096 ",
			args: args{
				bps:  1025.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data bs = 4096",
			args: args{
				bps:  4096.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data bs > 4096 and bs < 8192 ",
			args: args{
				bps:  4097.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data bs = 8192",
			args: args{
				bps:  8192.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data bs > 8192 and bs < 16384 ",
			args: args{
				bps:  8193.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data bs = 16384",
			args: args{
				bps:  16384.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data 16384 < bs < 32768 ",
			args: args{
				bps:  16385.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		{
			name: "test convert data bs >= 32768 ",
			args: args{
				bps:  32768.0,
				iops: 1,
				data: []float64{0, 0, 0, 0, 0, 0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ConvertData(tt.args.bps, tt.args.iops, tt.args.data)
		})
	}
}

func TestSolveEquations(t *testing.T) {
	type args struct {
		a1 float64
		b1 float64
		c1 float64
		a2 float64
		b2 float64
		c2 float64
	}
	tests := []struct {
		name  string
		args  args
		want  float64
		want1 float64
	}{
		{
			name: "test solve equations",
			args: args{
				a1: 0.0,
				b1: 0.0,
				c1: 0.0,
				a2: 0.0,
				b2: 0.0,
				c2: 0.0,
			},
			want:  0.0,
			want1: 0.0,
		},
		{
			name: "test solve equations 1",
			args: args{
				a1: 4096.0,
				b1: 8192.0,
				c1: 10485760,
				a2: 1,
				b2: 1,
				c2: 2475.5,
			},
			want:  9793536.0,
			want1: 692224.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := SolveEquations(tt.args.a1, tt.args.b1, tt.args.c1, tt.args.a2, tt.args.b2, tt.args.c2)
			if got != tt.want {
				t.Errorf("SolveEquations() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("SolveEquations() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestListDir(t *testing.T) {
	type args struct {
		dirname string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test list dir",
			args: args{
				dirname: "/tmp",
			},
			want: []string{},
		},
		{
			name: "test list dir tmp1",
			args: args{
				dirname: "/tmp1",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch1 := gomonkey.ApplyFunc(os.ReadDir, func(name string) ([]fs.DirEntry, error) {
				if tt.args.dirname == "/tmp" {
					return []fs.DirEntry{}, nil
				} else {
					return []fs.DirEntry{}, errors.New("not get")
				}
			})
			defer patch1.Reset()

			if got := ListDir(tt.args.dirname); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetReadWrite(t *testing.T) {
	type args struct {
		cgroupPath string
		DeviceMM   string
	}
	type fields struct {
		ServiceEngine  service.ServiceEngine
		Apps           map[string]*AppInfo
		ContainerToPod map[uint64]string
		EbpfBuckets    map[uint64]uint64
		Interval       utils.IntervalConfigInfo
		EbpfEnabled    bool
		prevRawData    *RawData
	}
	tests := []struct {
		name   string
		args   args
		fields fields
		want   int
		want1  int
		want2  int
		want3  int
	}{
		{
			name: "test get and read write",
			args: args{
				cgroupPath: "",
				DeviceMM:   "",
			},
			fields: fields{},
			want:   -1,
			want1:  -1,
			want2:  -1,
			want3:  -1,
		},
		{
			name: "test get and read write with true data",
			args: args{
				cgroupPath: "/tmp",
				DeviceMM:   "259:0",
			},
			want:  0,
			want1: 984368607232,
			want2: 0,
			want3: 240213838,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patchListDir := gomonkey.ApplyFunc(ListDir, func(name string) []string {
				return []string{"pod1"}
			})
			defer patchListDir.Reset()

			if tt.args.cgroupPath != "" {
				// create directory
				path := tt.args.cgroupPath + "/" + "pod1/"
				var _, err1 = os.Stat(path)
				if os.IsNotExist(err1) {
					// Create the directory
					err1 = os.Mkdir(path, 0777)
					if err1 != nil {
						t.Errorf("%s", err1)
					}
				}

				filename := path + "io.stat"
				var _, err = os.Stat(filename)
				if os.IsNotExist(err) {
					file, err := os.Create(filename)
					if err != nil {
						t.Errorf("%s", err)
					}
					fmt.Fprintf(file, "259:0 rbytes=0 wbytes=984368607232 rios=0 wios=240213838 dbytes=0 dios=0")
					file.Close()
				} else {
					klog.Infof("File:%s already exists!", filename)
				}
			}

			s := &DiskEngine{
				ServiceEngine:  tt.fields.ServiceEngine,
				Apps:           tt.fields.Apps,
				ContainerToPod: tt.fields.ContainerToPod,
				EbpfBuckets:    tt.fields.EbpfBuckets,
				Interval:       tt.fields.Interval,
				EbpfEnabled:    tt.fields.EbpfEnabled,
				prevRawData:    tt.fields.prevRawData,
			}
			got, got1, got2, got3 := s.GetReadWrite(tt.args.cgroupPath, tt.args.DeviceMM)
			if got != tt.want {
				t.Errorf("GetReadWrite() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetReadWrite() got1 = %v, want1 %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("GetReadWrite() got2 = %v, want %v", got2, tt.want2)
			}
			if got3 != tt.want3 {
				t.Errorf("GetReadWrite() got3 = %v, want3 %v", got3, tt.want3)
			}
		})
	}
}

func TestDiskIOServer_GetCurBandwidth(t *testing.T) {
	type fields struct {
		ServiceEngine  service.ServiceEngine
		Apps           map[string]*AppInfo
		ContainerToPod map[uint64]string
		EbpfBuckets    map[uint64]uint64
		Interval       utils.IntervalConfigInfo
		EbpfEnabled    bool
		prevRawData    *RawData
	}
	type args struct {
		preRawData *RawData
		curRawData *RawData
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]RW
	}{
		{
			name: "test get cur bandwidth",
			fields: fields{
				Interval: utils.IntervalConfigInfo{
					Interval: 1,
				},
			},
			args: args{
				preRawData: &RawData{
					rbps: &map[string]map[string]int{
						"app1": {
							"disk1": 1,
						},
					},
					wbps: &map[string]map[string]int{
						"app1": {
							"disk1": 1,
						},
					},
					riops: &map[string]map[string]int{
						"app1": {
							"disk1": 1,
						},
					},
					wiops: &map[string]map[string]int{
						"app1": {
							"disk1": 1,
						},
					},
				},
				curRawData: &RawData{
					rbps: &map[string]map[string]int{
						"app1": {
							"disk1": 2,
						},
					},
					wbps: &map[string]map[string]int{
						"app1": {
							"disk1": 2,
						},
					},
					riops: &map[string]map[string]int{
						"app1": {
							"disk1": 2,
						},
					},
					wiops: &map[string]map[string]int{
						"app1": {
							"disk1": 2,
						},
					},
				},
			},
			want: map[string]RW{
				"app1": {
					R: map[string]bdata{
						"disk1": {1, 0, 0, 0, 0, 0},
					},
					W: map[string]bdata{
						"disk1": {1, 0, 0, 0, 0, 0},
					},
				},
			},
		},
		{
			name:   "test get cur bandwidth no previous data",
			fields: fields{Interval: utils.IntervalConfigInfo{Interval: 1}},
			args: args{
				preRawData: &RawData{
					rbps: &map[string]map[string]int{
						"app1": {},
					},
					wbps: &map[string]map[string]int{
						"app1": {},
					},
					riops: &map[string]map[string]int{
						"app1": {},
					},
					wiops: &map[string]map[string]int{
						"app1": {},
					},
				},
				curRawData: &RawData{
					rbps: &map[string]map[string]int{
						"app1": {
							"disk1": 2,
						},
					},
					wbps: &map[string]map[string]int{
						"app1": {
							"disk1": 2,
						},
					},
					riops: &map[string]map[string]int{
						"app1": {
							"disk1": 2,
						},
					},
					wiops: &map[string]map[string]int{
						"app1": {
							"disk1": 2,
						},
					},
				},
			},
			want: map[string]RW{
				"app1": {
					R: map[string]bdata{
						"disk1": {2, 0, 0, 0, 0, 0},
					},
					W: map[string]bdata{
						"disk1": {2, 0, 0, 0, 0, 0},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DiskEngine{
				ServiceEngine:  tt.fields.ServiceEngine,
				Apps:           tt.fields.Apps,
				ContainerToPod: tt.fields.ContainerToPod,
				EbpfBuckets:    tt.fields.EbpfBuckets,
				Interval:       tt.fields.Interval,
				EbpfEnabled:    tt.fields.EbpfEnabled,
				prevRawData:    tt.fields.prevRawData,
			}
			if got := s.GetCurBandwidth(tt.args.preRawData, tt.args.curRawData); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DiskIOServer.GetCurBandwidth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkValidDevice(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test1",
			args: args{path: " "},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkValidDevice(tt.args.path); got != tt.want {
				t.Errorf("checkValidDevice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseDiskAppkString(t *testing.T) {
	type args struct {
		mockString string
	}
	tests := []struct {
		name    string
		args    args
		want    utils.DiskCgroupInfo
		wantErr bool
	}{
		{
			name:    "test1",
			args:    args{mockString: ""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDiskAppkString(tt.args.mockString)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDiskAppkString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDiskAppkString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkBinds(t *testing.T) {
	type args struct {
		path1 string
		path2 string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test1",
			args: args{path1: "", path2: ""},
			want: true,
		},
		{
			name: "test2",
			args: args{path1: "", path2: ""},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var outputs []gomonkey.OutputCell
			if tt.name == "test1" {
				outputs = []gomonkey.OutputCell{
					{Values: gomonkey.Params{"1", nil}},
					{Values: gomonkey.Params{"1", nil}},
				}
			} else if tt.name == "test2" {
				outputs = []gomonkey.OutputCell{
					{Values: gomonkey.Params{"1", errors.New("test2")}},
					{Values: gomonkey.Params{"2", errors.New("test2")}},
				}
			}
			patches := gomonkey.ApplyFuncSeq(utils.RunBash, outputs)
			if got := checkBinds(tt.args.path1, tt.args.path2); got != tt.want {
				t.Errorf("checkBinds() = %v, want %v", got, tt.want)
			}
			patches.Reset()
		})
	}
}

func TestDiskEngine_SetAppLimit(t *testing.T) {
	type fields struct {
		ServiceEngine  service.ServiceEngine
		Apps           map[string]*AppInfo
		ContainerToPod map[uint64]string
		EbpfBuckets    map[uint64]uint64
		Interval       utils.IntervalConfigInfo
		EbpfEnabled    bool
		prevRawData    *RawData
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
					"case": {
						AppName:    "case",
						CgroupPath: "",
					},
				},
			},
			args: args{
				ctx: context.TODO(),
				req: &pb.SetAppLimitRequest{
					AppId: "",
					Limit: []*pb.BWLimit{{
						Id:  "123.123",
						In:  "30",
						Out: "",
					}},
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
			s := &DiskEngine{
				ServiceEngine:  tt.fields.ServiceEngine,
				Apps:           tt.fields.Apps,
				ContainerToPod: tt.fields.ContainerToPod,
				EbpfBuckets:    tt.fields.EbpfBuckets,
				Interval:       tt.fields.Interval,
				EbpfEnabled:    tt.fields.EbpfEnabled,
				prevRawData:    tt.fields.prevRawData,
			}
			_, err := s.SetAppLimit(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.SetAppLimit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDiskEngine_Configure(t *testing.T) {
	j1, _ := json.Marshal(&loglevel)
	j2 := []byte("1")
	type fields struct {
		ServiceEngine  service.ServiceEngine
		Apps           map[string]*AppInfo
		ContainerToPod map[uint64]string
		EbpfBuckets    map[uint64]uint64
		Interval       utils.IntervalConfigInfo
		EbpfEnabled    bool
		prevRawData    *RawData
	}
	type args struct {
		ctx context.Context
		req *pb.ConfigureRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.ConfigureResponse
		wantErr bool
	}{
		{
			name:    "test1",
			args:    args{req: &pb.ConfigureRequest{Type: utils.LOGLEVEL, Configure: ""}},
			want:    &pb.ConfigureResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name:    "test2",
			args:    args{req: &pb.ConfigureRequest{Type: utils.LOGLEVEL, Configure: string(j1)}},
			want:    &pb.ConfigureResponse{Status: pb.ResponseState_OK},
			wantErr: true,
		},
		{
			name:    "test3",
			args:    args{req: &pb.ConfigureRequest{Type: utils.LOGLEVEL, Configure: string(j2)}},
			want:    &pb.ConfigureResponse{Status: pb.ResponseState_FAIL},
			wantErr: true,
		},
		{
			name:    "test4",
			args:    args{req: &pb.ConfigureRequest{Type: "", Configure: ""}},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DiskEngine{
				ServiceEngine:  tt.fields.ServiceEngine,
				Apps:           tt.fields.Apps,
				ContainerToPod: tt.fields.ContainerToPod,
				EbpfBuckets:    tt.fields.EbpfBuckets,
				Interval:       tt.fields.Interval,
				EbpfEnabled:    tt.fields.EbpfEnabled,
				prevRawData:    tt.fields.prevRawData,
			}
			_, err := s.Configure(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.Configure() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestFilterData(t *testing.T) {
	prev := &pb.AppsBandwidth{
		IoiType: 1,
		Payload: &pb.AppsBandwidth_DiskIoBw{
			DiskIoBw: &pb.AppDiskIOBandwidth{
				AppBwInfo: map[string]*pb.AppBandwidthInfo{
					"app1": {
						BwInfo: []*pb.BandwidthInfo{
							{
								Id:     "0",
								IsRead: true,
								Bandwidths: map[string]string{
									"512b": "100",
									"1k":   "200",
									"4k":   "300",
									"8k":   "400",
									"16k":  "500",
									"32k":  "600",
								},
							},
							{
								Id:     "1",
								IsRead: false,
								Bandwidths: map[string]string{
									"512b": "700",
									"1k":   "800",
									"4k":   "900",
									"8k":   "1000",
									"16k":  "1100",
									"32k":  "1200",
								},
							},
						},
					},
				},
			},
		},
	}

	cur := &pb.AppsBandwidth{
		IoiType: 1,
		Payload: &pb.AppsBandwidth_DiskIoBw{
			DiskIoBw: &pb.AppDiskIOBandwidth{
				AppBwInfo: map[string]*pb.AppBandwidthInfo{
					"app1": {
						BwInfo: []*pb.BandwidthInfo{
							{
								Id:     "0",
								IsRead: true,
								Bandwidths: map[string]string{
									"512b": "100",
									"1k":   "200",
									"4k":   "300",
									"8k":   "400",
									"16k":  "500",
									"32k":  "600",
								},
							},
							{
								Id:     "1",
								IsRead: false,
								Bandwidths: map[string]string{
									"512b": "700",
									"1k":   "800",
									"4k":   "900",
									"8k":   "1000",
									"16k":  "1100",
									"32k":  "1200",
								},
							},
						},
					},
				},
			},
		},
	}

	want := false
	got, err := FilterData(prev, cur)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDiskEngine_deleteAppInfo(t *testing.T) {
	type fields struct {
		ServiceEngine  service.ServiceEngine
		persist        common.IPersist
		Apps           map[string]*AppInfo
		Interval       utils.IntervalConfigInfo
		ContainerToPod map[uint64]string
		EbpfBuckets    map[uint64]uint64
		EbpfEnabled    bool
		prevRawData    *RawData
		prevMsg        *pb.AppsBandwidth
		LoopDevMap     map[string]string
	}
	type args struct {
		AppId string
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
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test delete app info without enable ebpf",
			fields: fields{
				Apps: map[string]*AppInfo{
					"app1": {
						AppName:    "app1",
						CgroupPath: "/tmp",
					},
				},
				persist: db,
			},
			args: args{
				AppId: "app1",
			},
			wantErr: false,
		},
		{
			name: "test delete app info after enabling ebpf",
			fields: fields{
				Apps: map[string]*AppInfo{
					"app1": {
						AppName:    "app1",
						CgroupPath: "/tmp",
					},
				},
				persist:     db,
				EbpfEnabled: true,
				ContainerToPod: map[uint64]string{
					1: "pod1",
				},
			},
			args: args{
				AppId: "app1",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DiskEngine{
				ServiceEngine:  tt.fields.ServiceEngine,
				persist:        tt.fields.persist,
				Apps:           tt.fields.Apps,
				Interval:       tt.fields.Interval,
				ContainerToPod: tt.fields.ContainerToPod,
				EbpfBuckets:    tt.fields.EbpfBuckets,
				EbpfEnabled:    tt.fields.EbpfEnabled,
				prevRawData:    tt.fields.prevRawData,
				prevMsg:        tt.fields.prevMsg,
				LoopDevMap:     tt.fields.LoopDevMap,
			}
			if err := AddAppInfo(s, *(tt.fields.Apps["app1"])); (err != nil) != tt.wantErr {
				t.Errorf("AddAppInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := s.deleteAppInfo(tt.args.AppId); (err != nil) != tt.wantErr {
				t.Errorf("DiskEngine.deleteAppInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
