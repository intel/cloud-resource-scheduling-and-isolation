/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package blockio

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

var ga1 = &utils.IOPoolStatus{
	Total: 100,
	In:    50,
	Out:   50,
}
var ga2 = &utils.IOPoolStatus{
	Total: 110,
	In:    55,
	Out:   55,
}
var be = &utils.IOPoolStatus{
	Total: 20,
	In:    10,
	Out:   10,
}
var DisksStatus = map[string]*DiskInfo{
	"dev1": {
		ReadRatio:          ReadRatio,
		WriteRatio:         WriteRatio,
		BEDefaultRead:      5,
		BEDefaultWrite:     5,
		Capacity:           100, // 90
		GA:                 ga2,
		BE:                 be,
		RequestedDiskSpace: 10,
	},
	"dev2": {
		ReadRatio:          ReadRatio,
		WriteRatio:         WriteRatio,
		BEDefaultRead:      5,
		BEDefaultWrite:     5,
		Capacity:           100, // 80
		GA:                 ga1,
		BE:                 be,
		RequestedDiskSpace: 20,
	},
}

func TestDeviceSlice(t *testing.T) {
	tests := []struct {
		name string
		args *DeviceSlice
	}{
		{
			name: "add device",
			args: NewDeviceSlice(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := tt.args
			for name, info := range DisksStatus {
				ds.AddDevice(name, info)
			}

			for n, d := range ds.devices {
				if !reflect.DeepEqual(DisksStatus[n].GA, d.GA) ||
					!reflect.DeepEqual(DisksStatus[n].BE, d.BE) ||
					(DisksStatus[n].Capacity-DisksStatus[n].RequestedDiskSpace) != d.Size {
					t.Errorf("DeviceSlice.AddDevice failed: want = %v, got %v", DisksStatus[n], d.String())
				}
			}

		})
	}
}

func TestFirstFitPacker(t *testing.T) {
	vol1 := &Volume{
		Name:           "vol1",
		PvcName:        "pvc1",
		QoS:            NotBE,
		RawRead:        20,
		RawWrite:       30,
		Size:           30,
		BlockSizeRead:  resource.MustParse("32k"),
		BlockSizeWrite: resource.MustParse("32k"),
	}
	vol2 := &Volume{
		Name:           "vol2",
		PvcName:        "pvc2",
		QoS:            NotBE,
		RawRead:        30,
		RawWrite:       20,
		Size:           40,
		BlockSizeRead:  resource.MustParse("32k"),
		BlockSizeWrite: resource.MustParse("32k"),
	}
	vol3 := &Volume{
		Name:           "vol3",
		PvcName:        "pvc3",
		QoS:            NotBE,
		RawRead:        40,
		RawWrite:       40,
		Size:           50,
		BlockSizeRead:  resource.MustParse("32k"),
		BlockSizeWrite: resource.MustParse("32k"),
	}
	vol4 := &Volume{
		Name:           "vol4",
		PvcName:        "pvc4",
		QoS:            NotBE,
		RawRead:        40,
		RawWrite:       40,
		Size:           60,
		BlockSizeRead:  resource.MustParse("32k"),
		BlockSizeWrite: resource.MustParse("32k"),
	}
	vol5 := &Volume{
		Name:    "vol5",
		PvcName: "pvc5",
		QoS:     BE,
		Size:    90,
	}
	vol6 := &Volume{
		Name:    "vol6",
		PvcName: "pvc6",
		QoS:     BE,
		Size:    90,
	}
	vol7 := &Volume{
		Name:    "vol7",
		PvcName: "pvc7",
		QoS:     BE,
		Size:    4,
	}
	vol8 := &Volume{
		Name:    "vol8",
		PvcName: "pvc8",
		QoS:     BE,
		Size:    6,
	}
	type fields struct {
		diskStatus map[string]*DiskInfo
		vols       VolumeSlice
	}
	tests := []struct {
		name string
		args *fields
		want bool
	}{
		{
			name: "able to pack 1",
			args: &fields{
				diskStatus: DisksStatus,
				vols: []*Volume{
					vol1,
					vol2,
				},
			},
			want: true,
		},
		{
			name: "able to pack 2",
			args: &fields{
				diskStatus: DisksStatus,
				vols: []*Volume{
					vol1,
					vol3,
				},
			},
			want: true,
		},
		{
			name: "unable to pack",
			args: &fields{
				diskStatus: DisksStatus,
				vols: []*Volume{
					vol1,
					vol2,
					vol3,
					vol4,
				},
			},
			want: false,
		},
		{
			name: "able to pack BE",
			args: &fields{
				diskStatus: DisksStatus,
				vols: []*Volume{
					vol7,
					vol8,
				},
			},
			want: true,
		},
		{
			name: "unable to pack BE",
			args: &fields{
				diskStatus: DisksStatus,
				vols: []*Volume{
					vol5,
					vol6,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ff := NewFirstFitPacker()

			_, allco, ret := ff.Pack(tt.args.diskStatus, tt.args.vols)
			if ret != tt.want {
				t.Errorf("FirstFitPack failed want = %v, got %v", tt.want, ret)
			}
			if ret {
				for v, d := range allco {
					t.Logf("%v:%v ", v, d.DevID)
				}
			}
		})
	}
}
