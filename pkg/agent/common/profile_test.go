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
	utils "sigs.k8s.io/IOIsolation/pkg"
)

func Test_getFioTestFile(t *testing.T) {
	type args struct {
		mnt string
		bs  string
		rw  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Test1",
			args{"/opt", "4k", "randread"},
			"/opt/4k_randread",
		},
		{
			"Test2",
			args{"/opt/", "4k", "randread"},
			"/opt/4k_randread",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getFioTestFile(tt.args.mnt, tt.args.bs, tt.args.rw); got != tt.want {
				t.Errorf("getFioTestFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getFioFileSize(t *testing.T) {
	type args struct {
		sz int64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Test1",
			args{378380828},
			"20G",
		},
		{
			"Test2",
			args{3948576},
			"1G",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getFioFileSize(tt.args.sz); got != tt.want {
				t.Errorf("getFioFileSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSingleWorkload(t *testing.T) {
	type fields struct {
		TestName  string
		DiskName  string
		RW        string
		BlockSize string
		OupDir    string
		OupFile   string
		Ratio     int
		Size      string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			"Test1",
			fields{TestName: "test1", DiskName: "./test", RW: "randread", BlockSize: "4k", Size: "20G", OupDir: "/opt/", OupFile: "test.json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fio := &FIO{
				TestName:  tt.fields.TestName,
				DiskName:  tt.fields.DiskName,
				RW:        tt.fields.RW,
				BlockSize: tt.fields.BlockSize,
				OupDir:    tt.fields.OupDir,
				OupFile:   tt.fields.OupFile,
				Ratio:     tt.fields.Ratio,
				Size:      tt.fields.Size,
			}
			op1 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{nil, nil}},
			}
			pt1 := gomonkey.ApplyFuncSeq(os.Stat, op1)
			defer pt1.Reset()
			op2 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{nil, nil}},
			}
			pt2 := gomonkey.ApplyFuncSeq(utils.RunBash, op2)
			defer pt2.Reset()
			fio.TestSingleWorkload()
		})
	}
}

func TestHybridSingleWorkload(t *testing.T) {
	type fields struct {
		TestName  string
		DiskName  string
		RW        string
		BlockSize string
		OupDir    string
		OupFile   string
		Ratio     int
		Size      string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			"Test1",
			fields{TestName: "test1", DiskName: "./test", RW: "randread", BlockSize: "4k", Size: "20G", OupDir: "/opt/", OupFile: "test.json", Ratio: 50},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fio := &FIO{
				TestName:  tt.fields.TestName,
				DiskName:  tt.fields.DiskName,
				RW:        tt.fields.RW,
				BlockSize: tt.fields.BlockSize,
				OupDir:    tt.fields.OupDir,
				OupFile:   tt.fields.OupFile,
				Ratio:     tt.fields.Ratio,
				Size:      tt.fields.Size,
			}
			op1 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{nil, nil}},
			}
			pt1 := gomonkey.ApplyFuncSeq(os.Stat, op1)
			defer pt1.Reset()
			op2 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{nil, nil}},
			}
			pt2 := gomonkey.ApplyFuncSeq(utils.RunBash, op2)
			defer pt2.Reset()
			fio.TestHybridSingleWorkload()
		})
	}
}

func Test_getMax(t *testing.T) {
	type args struct {
		a int64
		b int64
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			"Test1",
			args{a: 23, b: 46},
			46,
		},
		{
			"Test2",
			args{a: 46, b: 23},
			46,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getMax(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("getMax() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_initDiskCnt(t *testing.T) {
	type args struct {
		profile Profile
		dsInfo  map[string]utils.BlockDevice
	}
	tests := []struct {
		name string
		args args
		want Profile
	}{
		{
			"Test 1",
			args{profile: Profile{Disks: map[string]DiskProfiler{}}, dsInfo: map[string]utils.BlockDevice{"abc_123": {Name: "disk1", Model: "abc", Serial: "123", MajMin: "259:0", Type: "emptyDir", Mount: "/", AvailSize: 345632435}}},
			Profile{Disks: map[string]DiskProfiler{"abc_123": {Path: "disk1", Id: "abc_123", DeviceMM: "259:0", Status: 1, Type: "emptyDir", Mount: "/", Size: 345632435, SingleWorkload: []Bandwidth{{Blocksize: "512B", Read: 0, Write: 0}, {Blocksize: "1k", Read: 0, Write: 0},
				{Blocksize: "4k", Read: 0, Write: 0}, {Blocksize: "8k", Read: 0, Write: 0}, {Blocksize: "16k", Read: 0, Write: 0},
				{Blocksize: "32k", Read: 0, Write: 0}},
				HybridWorkload: Bandwidth{50, "32k", 0, 0}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if initDiskCnt(&tt.args.profile, &tt.args.dsInfo); !reflect.DeepEqual(tt.args.profile, tt.want) {
				t.Errorf("initDiskCnt() = %v, want %v", tt.args.profile, tt.want)
			}
		})
	}
}

func Test_addNewDisksToFile(t *testing.T) {
	type args struct {
		pDsInfo  *map[string]utils.BlockDevice
		filePath string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test1",
			args: args{
				pDsInfo:  &map[string]utils.BlockDevice{"abc_123": {Name: "disk1", Model: "abc", Serial: "123", MajMin: "259:0", Type: "emptyDir", Mount: "/", AvailSize: 345632435}},
				filePath: "/tmp/test.json",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createDisksToFile(tt.args.pDsInfo, tt.args.filePath)

			if err := addNewDisksToFile(tt.args.pDsInfo, tt.args.filePath); (err != nil) != tt.wantErr {
				t.Errorf("addNewDisksToFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJsonStruct_Load(t *testing.T) {
	type args struct {
		filename string
		v        interface{}
	}

	pDsInfo := &map[string]utils.BlockDevice{"abc_123": {Name: "disk1", Model: "abc", Serial: "123", MajMin: "259:0", Type: "emptyDir", Mount: "/", AvailSize: 345632435}}
	filePath := "/tmp/test.json"

	tests := []struct {
		name string
		jst  *JsonStruct
		args args
	}{
		{
			name: "Test json struct load",
			jst:  &JsonStruct{},
			args: args{filename: "/tmp/test.json", v: &Profile{}},
		},
		{
			name: "Test json struct load fail",
			jst:  &JsonStruct{},
			args: args{filename: "/tmp/test1.json", v: &Profile{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createDisksToFile(pDsInfo, filePath)
			jst := &JsonStruct{}
			jst.Load(tt.args.filename, tt.args.v)
		})
	}
}

func TestGetDiskStatus(t *testing.T) {
	type args struct {
		filename string
	}

	pDsInfo := &map[string]utils.BlockDevice{"abc_123": {Name: "disk1", Model: "abc", Serial: "123", MajMin: "259:0", Type: "emptyDir", Mount: "/", AvailSize: 345632435}}
	filePath := "/tmp/test.json"

	tests := []struct {
		name    string
		args    args
		want    map[string]int
		wantErr bool
	}{
		{
			name:    "TestGetDiskStatus",
			args:    args{filename: "/tmp/test.json"},
			want:    map[string]int{"abc_123": 1},
			wantErr: false,
		},
		{
			name:    "TestGetDiskStatus fail",
			args:    args{filename: "/tmp/test1.json"},
			want:    map[string]int{"abc_123": 1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createDisksToFile(pDsInfo, filePath)

			got, err := GetDiskStatus(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDiskStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("GetDiskStatus() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestParseAllDiskProfileForController(t *testing.T) {
	type args struct {
		filename     string
		deviceStatus map[string]int
	}
	tests := []struct {
		name string
		args args
		want utils.DiskInfos
	}{
		{
			name: "TestParseAllDiskProfileForController",
			args: args{filename: "/tmp/test.json", deviceStatus: map[string]int{"abc_123": 1}},
			want: utils.DiskInfos{
				"abc_123": utils.DiskInfo{
					Name:       "disk1",
					MajorMinor: "259:0",
					Type:       "emptyDir",
					MountPoint: "/",
					Capacity:   "345632435",
					TotalBPS:   0,
					TotalRBPS:  0,
					TotalWBPS:  0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAllDiskProfileForController(tt.args.filename, tt.args.deviceStatus)
			if !reflect.DeepEqual(got["abc_123"].Type, tt.want["abc_123"].Type) {
				t.Errorf("ParseAllDiskProfileForController() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAnalysisDiskProfile(t *testing.T) {
	type args struct {
		deviceStatus map[string]int
		filename     string
	}
	tests := []struct {
		name string
		args args
		want *utils.DiskInfos
	}{
		{
			name: "TestAnalysisDiskProfile",
			args: args{
				deviceStatus: map[string]int{"abc_123": 1},
				filename:     "/tmp/test.json",
			},
			want: &utils.DiskInfos{
				"abc_123": utils.DiskInfo{
					Name:       "disk1",
					MajorMinor: "259:0",
					Type:       "emptyDir",
					MountPoint: "/",
					Capacity:   "345632435",
					TotalBPS:   0,
					TotalRBPS:  0,
					TotalWBPS:  0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalysisDiskProfile(tt.args.deviceStatus, tt.args.filename)
			if !reflect.DeepEqual((*got)["abc_123"].Type, (*tt.want)["abc_123"].Type) {
				t.Errorf("AnalysisDiskProfile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFIO_ParsingFIOResults(t *testing.T) {
	type fields struct {
		TestName  string
		DiskName  string
		RW        string
		BlockSize string
		OupDir    string
		OupFile   string
		Ratio     int
		Size      string
	}
	type args struct {
		mode string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []int64
		wantErr bool
	}{
		{
			name: "TestParsingFIOResults single workload",
			fields: fields{
				TestName:  "test1",
				DiskName:  "./test",
				RW:        "randread",
				BlockSize: "4k",
				Size:      "20G",
				OupDir:    "/tmp/",
				OupFile:   "test.json",
				Ratio:     50,
			},
			args:    args{mode: "SingleWorkload"},
			want:    []int64{1},
			wantErr: false,
		},
		{
			name: "TestParsingFIOResults Hybrid Single Workload",
			fields: fields{
				TestName:  "test1",
				DiskName:  "./test",
				RW:        "randread",
				BlockSize: "4k",
				Size:      "20G",
				OupDir:    "/tmp/",
				OupFile:   "test.json",
				Ratio:     50,
			},
			args:    args{mode: "HybridSingleWorkload"},
			want:    []int64{1, 1},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fio := &FIO{
				TestName:  tt.fields.TestName,
				DiskName:  tt.fields.DiskName,
				RW:        tt.fields.RW,
				BlockSize: tt.fields.BlockSize,
				OupDir:    tt.fields.OupDir,
				OupFile:   tt.fields.OupFile,
				Ratio:     tt.fields.Ratio,
				Size:      tt.fields.Size,
			}
			pDsInfo := &map[string]utils.BlockDevice{"abc_123": {Name: "disk1", Model: "abc", Serial: "123", MajMin: "259:0", Type: "emptyDir", Mount: "/", AvailSize: 345632435}}
			filePath := "/tmp/test.json"
			createDisksToFile(pDsInfo, filePath)

			got, err := fio.ParsingFIOResults(tt.args.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("FIO.ParsingFIOResults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FIO.ParsingFIOResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNewDisks(t *testing.T) {
	type args struct {
		pDsInfo *map[string]utils.BlockDevice
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test get new disks",
			args: args{
				pDsInfo: &map[string]utils.BlockDevice{"abc_123": {Name: "disk1", Model: "abc", Serial: "123", MajMin: "259:0", Type: "emptyDir", Mount: "/", AvailSize: 345632435}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GetNewDisks(tt.args.pDsInfo, "/tmp/test.json")
		})
	}

	tests2 := []struct {
		name string
		args args
	}{
		{
			name: "test get new disks",
			args: args{
				pDsInfo: &map[string]utils.BlockDevice{"abc_123": {Name: "disk1", Model: "abc", Serial: "123", MajMin: "259:0", Type: "emptyDir", Mount: "/", AvailSize: 345632435}},
			},
		},
	}
	for _, tt := range tests2 {
		e := os.Remove("/tmp/test.json")
		if e != nil {
			t.Fatal(e)
		}
		t.Run(tt.name, func(t *testing.T) {
			GetNewDisks(tt.args.pDsInfo, "/tmp/test.json")
		})
	}
}

func TestWriteFailDisk(t *testing.T) {
	type args struct {
		disk utils.BlockDevice
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "TestWriteFailDisk",
			args: args{
				disk: utils.BlockDevice{Name: "disk1", Model: "abc", Serial: "123", MajMin: "259:0", Type: "emptyDir", Mount: "/", AvailSize: 345632435},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WriteFailDisk(tt.args.disk)
		})
	}
}
