/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

// Package iomonitor This program demonstrates attaching an eBPF program to a kernel tracepoint.
// The eBPF program will be attached to the page allocation tracepoint and
// prints out the number of times it has been reached. The tracepoint fields
// are printed into /sys/kernel/tracing/trace_pipe.
package disk

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"

	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go bpf tracepoint.c -- -I../headers

type Info struct {
	Cgroup    uint64 // cgroup id
	Operation uint32 // type of I/O (W=write, R=read, A=readahead, S=sync, ...)
	Slot      uint32 // size of I/O (in sectors)
	Dev       uint32 // storage device major and minor number
	// _         uint32
}
type CGroupID struct {
	Cgroup uint64 // cgroup id
}

// for ebpf
var objs bpfObjects
var tp link.Link

func (d *DiskEngine) UpdateCountingMap() map[string]RW {
	// initialize bandwidth map
	bandwidth := map[string]RW{}
	for pod, info := range d.Apps {
		bandwidth[pod] = RW{
			R: map[string]bdata{},
			W: map[string]bdata{},
		}
		for _, dev := range info.DeviceMM {
			bandwidth[pod].R[dev] = make([]float64, 10)
			bandwidth[pod].W[dev] = make([]float64, 10)
		}
	}
	// iterate counting map in ebpf program
	entries := objs.CountingMap.Iterate()
	var key Info
	var value uint64
	for entries.Next(&key, &value) {
		klog.V(utils.DBG).Infof(">>> %v <<< major:minor: %v:%v, size: %v, operation: %v, cgroup: %v", value,
			key.Dev>>20, key.Dev&((1<<20)-1), key.Slot, key.Operation, key.Cgroup)
		pod, ok := d.ContainerToPod[key.Cgroup]
		if !ok {
			// klog.Infof("No pod found for cgroup %v", key.Cgroup)
			continue
		}
		hash := utils.HashObject(key)
		d.Lock()
		if _, ok := d.EbpfBuckets[hash]; !ok {
			d.EbpfBuckets[hash] = 0
		}
		prev := d.EbpfBuckets[hash]
		d.EbpfBuckets[hash] = value
		d.Unlock()
		dev := fmt.Sprintf("%v:%v", key.Dev>>20, key.Dev&((1<<20)-1))
		if _, ok := bandwidth[pod]; !ok {
			continue
		}
		if _, ok := bandwidth[pod].R[dev]; !ok {
			if v, ok := d.LoopDevMap[dev]; ok {
				dev = v
			} else {
				continue
			}
		}
		if _, ok := bandwidth[pod].W[dev]; !ok {
			if v, ok := d.LoopDevMap[dev]; ok {
				dev = v
			} else {
				continue
			}
		}
		// calculate bandwidth in MiB/s
		delta := value - prev
		if value < prev {
			delta = 0
		}
		if key.Operation == 0 {
			bandwidth[pod].R[dev][key.Slot] += float64(delta*slotToSize(key.Slot)) / float64(d.Interval.Interval) // 2 seconds
		} else {
			bandwidth[pod].W[dev][key.Slot] += float64(delta*slotToSize(key.Slot)) / float64(d.Interval.Interval) // 2 seconds
		}
	}
	return bandwidth
}

func slotToSize(slot uint32) uint64 {
	return (1 << slot) * 512
}

func (d *DiskEngine) SynchronizeContainerList() {
	byteOrder := determineHostByteOrder()
	for pod, info := range d.Apps {
		// get all container list
		podCgroupPath := info.CgroupPath
		var dirs []string
		files, err := os.ReadDir(podCgroupPath)
		if err != nil {
			klog.Errorf("ReadDir %v failed: %v", podCgroupPath, err)
			continue
		}
		for _, f := range files {
			if f.IsDir() && strings.Contains(f.Name(), "cri-containerd") {
				dirs = append(dirs, fmt.Sprintf("%s/%s", podCgroupPath, f.Name()))
			}
		}
		// update container list in service cache and ebpf filter
		for _, path := range dirs {
			handle, _, err := unix.NameToHandleAt(unix.AT_FDCWD, path, 0)
			if err != nil {
				klog.Infof("Error resolving handle of %s: %s", path, err)
				continue
			}
			cgroupid := byteOrder.Uint64(handle.Bytes())
			if _, ok := d.ContainerToPod[cgroupid]; !ok {
				d.ContainerToPod[cgroupid] = pod
				err := objs.FilterMap.Put(CGroupID{Cgroup: cgroupid}, uint8(1))
				if err != nil {
					klog.Errorf("Error adding cgroup %v to ebpf filter: %v", cgroupid, err)
				}
				klog.Info("Add ContainerToPod:", cgroupid, " of pod ", pod)
			}
		}
	}
}
func determineHostByteOrder() binary.ByteOrder {
	var i uint16 = 1
	if byte(i) == 1 {
		return binary.LittleEndian
	} else {
		return binary.BigEndian
	}
}
