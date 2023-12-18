/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package blockio

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

const (
	NotBE string = "NotBE"
	BE    string = "BE"
)

type Device struct {
	Name string
	GA   *utils.IOPoolStatus
	BE   *utils.IOPoolStatus
	BEM  *utils.IOPoolStatus
	Size int64
}

func (ds *Device) String() string {
	return fmt.Sprintf("%s(GA: %v/%v/%v, BE: %v/%v/%v/%v, BEM: %v/%v/%v, Size: %v)", ds.Name, ds.GA.In, ds.GA.Out, ds.GA.Total, ds.BE.In, ds.BE.Out, ds.BE.Total, ds.BE.Num, ds.BEM.In, ds.BEM.Out, ds.BEM.Total, ds.Size)
}

func (ds *Device) canFit(vol *Volume, diskinfo *DiskInfo) bool {
	if vol.QoS == NotBE {
		_, rratio := utils.FindMappingRatio(vol.BlockSizeRead, diskinfo.ReadRatio)
		_, wratio := utils.FindMappingRatio(vol.BlockSizeWrite, diskinfo.WriteRatio)
		if ds.GA.In >= vol.RawRead*rratio && ds.GA.Out >= vol.RawWrite*wratio && ds.GA.Total >= vol.RawRead*rratio+vol.RawWrite*wratio &&
			ds.Size >= vol.Size && !ds.GA.UnderPressure {
			vol.TransRead = vol.RawRead * rratio
			vol.TransWrite = vol.RawWrite * wratio
			return true
		} else {
			return false
		}

	} else if vol.QoS == BE {
		return ds.BE.In/float64(ds.BE.Num+1) > ds.BEM.In && ds.BE.Out/float64(ds.BE.Num+1) > ds.BEM.Out && ds.BE.Total/float64(ds.BE.Num+1) > ds.BEM.Total && ds.Size >= vol.Size && !ds.BE.UnderPressure
	}
	klog.Errorf("volume %v QoS %v cannot parse", vol.Name, vol.QoS)
	return false
}

func (ds *Device) pack(vol *Volume) {
	if vol.QoS == NotBE {
		ds.GA.In -= vol.TransRead
		ds.GA.Out -= vol.TransWrite
		ds.GA.Total -= vol.TransRead + vol.TransWrite
	}
	ds.Size -= vol.Size
}

// todo: add QoS type and mapping ratio
func (ds *Device) AddVol(vol *Volume, ratio *DiskInfo) bool {
	if !ds.canFit(vol, ratio) {
		return false
	}
	ds.pack(vol)
	return true
}

// Keep the disk's remaining capacity
type DeviceSlice struct {
	devices map[string]*Device
}

func NewDeviceSlice() *DeviceSlice {
	return &DeviceSlice{
		devices: make(map[string]*Device),
	}
}

func (ds *DeviceSlice) AddDevice(name string, dev *DiskInfo) {
	ds.devices[name] = &Device{
		Name: name,
		GA:   dev.GA.DeepCopy(),
		BE:   dev.BE.DeepCopy(),
		BEM: &utils.IOPoolStatus{
			In:    dev.BEDefaultRead,
			Out:   dev.BEDefaultWrite,
			Total: dev.BEDefaultRead + dev.BEDefaultWrite,
		},
		Size: dev.Capacity - dev.RequestedDiskSpace,
	}
}

type Volume struct {
	Name           string
	PvcName        string
	QoS            string
	RawRead        float64
	RawWrite       float64
	TransRead      float64
	TransWrite     float64
	BlockSizeRead  resource.Quantity
	BlockSizeWrite resource.Quantity
	Size           int64
}

func (i *Volume) String() string {
	return fmt.Sprintf("%s(read: %v, write: x%v, size: %v)", i.Name, i.RawRead, i.RawWrite, i.Size)
}

func (v Volume) GetVolume() float64 {
	return (v.RawRead + v.RawWrite) * float64(v.Size)
}

type VolumeSlice []*Volume

func (is VolumeSlice) Len() int { return len(is) }
func (is VolumeSlice) Less(i, j int) bool {
	return is[i].GetVolume() > is[j].GetVolume()
}
func (is VolumeSlice) Swap(i, j int) {
	is[i], is[j] = is[j], is[i]
}

type BinPacker interface {
	Pack(map[string]*DiskInfo, []*Volume) (map[string]string, map[string]*utils.PVCInfo, bool)
}

type FirstFitPacker struct {
	Name string
	ds   *DeviceSlice
}

func NewFirstFitPacker() *FirstFitPacker {
	return &FirstFitPacker{
		Name: "FirstFit",
		ds:   NewDeviceSlice(),
	}
}

// return value: first: volume to device, second: pvc.podNamespace to PVCInfo, third: success or not
func (p *FirstFitPacker) Pack(devices map[string]*DiskInfo, vols []*Volume) (map[string]string, map[string]*utils.PVCInfo, bool) {
	allocation := make(map[string]*utils.PVCInfo)
	volToDev := make(map[string]string)
	// sort volume in descending order
	sort.Sort(VolumeSlice(vols))

	// cache disks remaining resources
	for n, devInfo := range devices {
		p.ds.AddDevice(n, devInfo)
	}
	// cache devices' keys
	devs := []string{}
	for dev := range devices {
		devs = append(devs, dev)
	}
	// sort keys based on devInfo.GA.Total in descending order
	sort.Slice(devs, func(i, j int) bool {
		return devices[devs[i]].GA.Total > devices[devs[j]].GA.Total
	})
	for _, v := range vols {
		for _, name := range devs {
			info := devices[name]
			dev := p.ds.devices[name]
			// this device does not fit
			if dev.AddVol(v, info) {
				// pack the volume into device
				allocation[v.PvcName] = &utils.PVCInfo{
					DevID:      name,
					ReqStorage: v.Size,
					Reuse:      false,
				}
				volToDev[v.Name] = name
				break
			}
		}
		// return false if the volume cannot be allocated
		if _, ok := allocation[v.PvcName]; !ok {
			return nil, nil, false
		}
	}
	return volToDev, allocation, true
}
