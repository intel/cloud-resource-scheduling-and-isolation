/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/service"
)

func Test_slotToSize(t *testing.T) {
	tests := []struct {
		name string
		slot uint32
		want uint64
	}{
		{
			name: "Slot 0",
			slot: 0,
			want: 512,
		},
		{
			name: "Slot 1",
			slot: 1,
			want: 1024,
		},
		{
			name: "Slot 2",
			slot: 2,
			want: 2048,
		},
		// Add more test cases as needed
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slotToSize(tt.slot); got != tt.want {
				t.Errorf("slotToSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_determineHostByteOrder(t *testing.T) {
	tests := []struct {
		name string
		want binary.ByteOrder
	}{
		{
			name: "LittleEndian",
			want: binary.LittleEndian,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := determineHostByteOrder(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("determineHostByteOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createCountingMap() (*ebpf.Map, error) {
	//// Increase the memory rlimit to allow for eBPF program loading
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, err
	}

	// Define the map specification
	mapSpec := &ebpf.MapSpec{
		Type:       ebpf.Hash,
		KeySize:    uint32(binary.Size(Info{})), // Adjust key size as necessary
		ValueSize:  8,                           // Adjust value size as necessary
		MaxEntries: 1024,
	}

	// Create the map
	countingMap, err := ebpf.NewMap(mapSpec)
	if err != nil {
		return nil, err
	}

	return countingMap, nil
}

func serializeInfo(info Info) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, info); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestDiskEngine_SynchronizeContainerList(t *testing.T) {
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
		fields fields
	}{
		{
			name: "Read cgroup fail",
			fields: fields{
				ContainerToPod: map[uint64]string{12345: "test"},
				EbpfBuckets:    make(map[uint64]uint64),
				Apps: map[string]*AppInfo{
					"test": {
						AppName:    "test",
						CgroupPath: "test",
						DeviceMM:   []string{"0:1", "251:0"},
					},
				},
				prevRawData: nil,
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
			s.SynchronizeContainerList()
		})
	}
}

func TestDiskEngine_UpdateCountingMap(t *testing.T) {
	type fields struct {
		ServiceEngine  service.ServiceEngine
		Apps           map[string]*AppInfo
		ContainerToPod map[uint64]string
		EbpfBuckets    map[uint64]uint64
		Interval       utils.IntervalConfigInfo
		EbpfEnabled    bool
		prevRawData    *RawData
		mode           uint32
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]RW
	}{
		{
			name: "UpdateCountingMap write key found",
			fields: fields{

				ContainerToPod: map[uint64]string{12345: "test"},
				EbpfBuckets:    make(map[uint64]uint64),
				Apps: map[string]*AppInfo{
					"test": {
						AppName:    "test",
						CgroupPath: "test",
						DeviceMM:   []string{"0:1", "251:0"},
					},
				},
				prevRawData: nil,
				mode:        1,
			},
			want: map[string]RW{
				"test": {
					R: map[string]bdata{
						"0:1":   {0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						"251:0": {0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
					},
					W: map[string]bdata{
						"0:1":   {0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						"251:0": {0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
					},
				},
			},
		},
		{
			name: "UpdateCountingMap read key found",
			fields: fields{
				ContainerToPod: map[uint64]string{12345: "test"},
				EbpfBuckets:    make(map[uint64]uint64),
				Apps: map[string]*AppInfo{
					"test": {
						AppName:    "test",
						CgroupPath: "test",
						DeviceMM:   []string{"0:1", "251:0"},
					},
				},
				prevRawData: nil,
				mode:        0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := createCountingMap()
			if err != nil {
				klog.Errorf("error is %v", err)
				return
			}
			// Add key-value pairs to the CountingMap
			infoKey := Info{
				Cgroup:    12345,
				Operation: tt.fields.mode, // Write operation
				Slot:      0,
				Dev:       1,
			}
			serializedKey, err := serializeInfo(infoKey)
			if err != nil {
				t.Errorf("Failed to serialize Info key: %v", err)
			}
			if err := m.Update(serializedKey, binary.LittleEndian.Uint64([]byte{0, 0, 0, 0, 0, 0, 0, 100}), ebpf.UpdateAny); err != nil {
				t.Errorf("Failed to update CountingMap: %v", err)
			}

			objs = bpfObjects{
				bpfPrograms: bpfPrograms{},
				bpfMaps: bpfMaps{
					CountingMap: m,
					FilterMap:   nil,
				},
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
			s.UpdateCountingMap()
		})
	}
}
