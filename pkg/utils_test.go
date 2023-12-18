/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

func MakePVC(client kubernetes.Interface, ctx context.Context, claim *v1.PersistentVolumeClaim) error {

	_, err := client.CoreV1().PersistentVolumeClaims("default").Create(context.TODO(), claim, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pvc: %v, err=%v", claim.Name, err)
	}
	return nil
}

func MakeStorageClass(client kubernetes.Interface, ctx context.Context, name, provisionner string) error {
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			// Namespace: "default",
			Name: name,
		},
		Provisioner: provisionner,
	}
	_, err := client.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create storage class: %v, err=%v", name, err)
	}
	return nil
}

func TestSetLogLevel(t *testing.T) {
	type args struct {
		info  bool
		debug bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "set log level 1",
			args: args{true, true},
		},
		{
			name: "set log level 2",
			args: args{false, false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLogLevel(tt.args.info, tt.args.debug)
		})
	}
}

func TestContains(t *testing.T) {
	type args struct {
		arr []string
		num string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test not exist",
			args: args{[]string{"disk1"}, "disk2"},
			want: false,
		},
		{
			name: "test exist",
			args: args{[]string{"disk1"}, "disk1"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.arr, tt.args.num); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIOPoolStatusDeepCopy(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "test IOPoolStatus's DeepCopy function",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &IOPoolStatus{
				In:    1,
				Out:   2,
				Total: 3,
			}
			out := in.DeepCopy()
			if in == out || !reflect.DeepEqual(in, out) {
				t.Errorf("DeepCopy failed")
			}
		})
	}
}

func Test_GetDiskId(t *testing.T) {
	type args struct {
		model  string
		serial string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Test1",
			args{"", ""},
			"",
		},
		{
			"Test2",
			args{"", "123"},
			"123",
		},
		{
			"Test3",
			args{"abc", ""},
			"abc",
		},
		{
			"Test4",
			args{"abcd", "123"},
			"abc_123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDiskId(tt.args.model, tt.args.serial); got != tt.want {
				t.Errorf("GetDiskId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsIOIProvisioner(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		pvc    string
		ns     string
	}
	scName1 := "sc1"
	scName2 := "sc2"
	ctx := context.Background()
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()

	if err := MakeStorageClass(fakeKubeClient, ctx, scName1, "localstorage.csi.k8s.io"); err != nil {
		t.Error(err)
	}
	if err := MakePVC(fakeKubeClient, ctx, &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc1",
			Namespace: "default",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &scName1,
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					"storage": resource.MustParse("10"),
				},
			},
		},
	}); err != nil {
		t.Error(err)
	}

	if err := MakeStorageClass(fakeKubeClient, ctx, scName2, "test.csi.k8s.io"); err != nil {
		t.Error(err)
	}
	if err := MakePVC(fakeKubeClient, ctx, &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc2",
			Namespace: "default",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &scName2,
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					"storage": resource.MustParse("10"),
				},
			},
		},
	}); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second)
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				client: fakeKubeClient,
				pvc:    "pvc1",
				ns:     "default",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "test2",
			args: args{
				client: fakeKubeClient,
				pvc:    "pvc2",
				ns:     "default",
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsIOIProvisioner(tt.args.client, tt.args.pvc, tt.args.ns)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsIOIProvisioner() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsIOIProvisioner() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindMappingRatio(t *testing.T) {
	type args struct {
		bs     resource.Quantity
		ratios []MappingRatio
	}
	tests := []struct {
		name  string
		args  args
		want  resource.Quantity
		want1 float64
	}{
		{
			name: "test1",
			args: args{
				bs:     resource.MustParse("6k"),
				ratios: []MappingRatio{{resource.MustParse("4k"), 1}, {resource.MustParse("8k"), 2}},
			},
			want:  resource.MustParse("4k"),
			want1: 1,
		},
		{
			name: "test2",
			args: args{
				bs:     resource.MustParse("512"),
				ratios: []MappingRatio{{resource.MustParse("1k"), 1}, {resource.MustParse("4k"), 2}},
			},
			want:  resource.MustParse("1k"),
			want1: 1,
		},
		{
			name: "test3",
			args: args{
				bs:     resource.MustParse("32k"),
				ratios: []MappingRatio{{resource.MustParse("1k"), 1}, {resource.MustParse("4k"), 2}},
			},
			want:  resource.MustParse("4k"),
			want1: 2,
		},
		{
			name: "test4",
			args: args{
				bs:     resource.MustParse("32k"),
				ratios: []MappingRatio{{resource.MustParse("32k"), 1}, {resource.MustParse("32k"), 2}},
			},
			want:  resource.MustParse("32k"),
			want1: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := FindMappingRatio(tt.args.bs, tt.args.ratios)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindMappingRatio() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("FindMappingRatio() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestParseDiskRequestBandwidth(t *testing.T) {
	type args struct {
		config string
	}
	tests := []struct {
		name    string
		args    args
		want    resource.Quantity
		want1   resource.Quantity
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				config: "",
			},
			want:    resource.Quantity{},
			want1:   resource.Quantity{},
			wantErr: true,
		},
		{
			name: "test2",
			args: args{
				config: "10M",
			},
			want:    resource.MustParse("10M"),
			want1:   resource.MustParse("4k"),
			wantErr: false,
		},
		{
			name: "test3",
			args: args{
				config: "10M/32k",
			},
			want:    resource.MustParse("10M"),
			want1:   resource.MustParse("32k"),
			wantErr: false,
		},
		{
			name: "test4",
			args: args{
				config: "10M/512B",
			},
			want:    resource.MustParse("10M"),
			want1:   resource.MustParse("512"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := ParseDiskRequestBandwidth(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDiskRequestBandwidth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseDiskRequestBandwidth() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ParseDiskRequestBandwidth() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestParseMappingRatio(t *testing.T) {
	type args struct {
		ratio map[string]float64
	}
	tests := []struct {
		name string
		args args
		want []MappingRatio
	}{
		{
			name: "test1",
			args: args{
				ratio: map[string]float64{"4k": 1, "8k": 2, "16k": 3},
			},
			want: []MappingRatio{{resource.MustParse("4k"), 1}, {resource.MustParse("8k"), 2}, {resource.MustParse("16k"), 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseMappingRatio(tt.args.ratio); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMappingRatio() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunBash(t *testing.T) {
	type args struct {
		v1 string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "test1",
			args:    args{"echo 1"},
			want:    "1\n",
			wantErr: false,
		},
		{
			name:    "test2",
			args:    args{"echo"},
			want:    "\n",
			wantErr: false,
		},
		{
			name:    "test2",
			args:    args{"ll"},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RunBash(tt.args.v1)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunBash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("RunBash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsIntelPlatform(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{
			name: "test1",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsIntelPlatform(); got != tt.want {
				t.Errorf("IsIntelPlatform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransPVCInfoMapToDevice(t *testing.T) {
	type args struct {
		in map[string]*PVCInfo
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "test1",
			args: args{
				in: map[string]*PVCInfo{
					"pvc1": {
						DevID: "dev1",
					},
					"pvc2": {
						DevID: "dev2",
					},
				},
			},
			want: map[string]string{
				"pvc1": "dev1",
				"pvc2": "dev2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TransPVCInfoMapToDevice(tt.args.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TransPVCInfoMapToDevice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergePVCInfo(t *testing.T) {
	type args struct {
		current map[string]*PVCInfo
		added   map[string]*PVCInfo
	}
	tests := []struct {
		name string
		args args
		want map[string]*PVCInfo
	}{
		{
			name: "test1",
			args: args{
				current: map[string]*PVCInfo{
					"pvc1": {},
				},
				added: map[string]*PVCInfo{
					"pvc2": {},
					"pvc3": {},
				},
			},
			want: map[string]*PVCInfo{
				"pvc1": {},
				"pvc2": {},
				"pvc3": {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergePVCInfo(tt.args.current, tt.args.added); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergePVCInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "Unmarshal true",
			data:     []byte(`true`),
			expected: true,
		},
		{
			name:     "Unmarshal 1",
			data:     []byte(`1`),
			expected: true,
		},
		{
			name:     "Unmarshal \"true\"",
			data:     []byte(`"true"`),
			expected: true,
		},
		{
			name:     "Unmarshal \"1\"",
			data:     []byte(`"1"`),
			expected: true,
		},
		{
			name:     "Unmarshal false",
			data:     []byte(`false`),
			expected: false,
		},
		{
			name:     "Unmarshal 0",
			data:     []byte(`0`),
			expected: false,
		},
		{
			name:     "Unmarshal \"false\"",
			data:     []byte(`"false"`),
			expected: false,
		},
		{
			name:     "Unmarshal \"0\"",
			data:     []byte(`"0"`),
			expected: false,
		},
		{
			name:     "Unmarshal invalid",
			data:     []byte(`"invalid"`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b SpecialBool
			err := b.UnmarshalJSON(tt.data)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if b.Bool != tt.expected {
				t.Errorf("Expected bool to be %v, but got %v", tt.expected, b.Bool)
			}
		})
	}
}

func TestGetCPUInfoPath(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Get CPU info path",
			want: "/proc/cpuinfo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCPUInfoPath(); got != tt.want {
				t.Errorf("GetCPUInfoPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpecialBool_MarshalJSON(t *testing.T) {
	type fields struct {
		Bool bool
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name:    "SpecialBool test",
			fields:  fields{true},
			want:    []byte("true"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &SpecialBool{
				Bool: tt.fields.Bool,
			}
			got, err := b.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("SpecialBool.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SpecialBool.MarshalJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringsToJson(t *testing.T) {
	type args struct {
		s []string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "empty",
			args: args{
				s: []string{},
			},
			want:    "[]",
			wantErr: false,
		},
		{
			name: "not empty",
			args: args{
				s: []string{"dev1", "dev2"},
			},
			want:    "[\"dev1\",\"dev2\"]",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StringsToJson(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("StringsToJson() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("StringsToJson() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJsonToStrings(t *testing.T) {
	type args struct {
		jsonStr string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "empty",
			args: args{
				jsonStr: "",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "no elem",
			args: args{
				jsonStr: "[]",
			},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "has elem",
			args: args{
				jsonStr: "[\"dev1\"]",
			},
			want:    []string{"dev1"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := JsonToStrings(tt.args.jsonStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("JsonToStrings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("JsonToStrings() = %v, want %v", got, tt.want)
			}
		})
	}
}
