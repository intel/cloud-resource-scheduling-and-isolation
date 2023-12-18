/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

var mountInfoPath string = "/proc/mounts"
var cgroupControllerPath string = "/sys/fs/cgroup/cgroup.controllers"

var (
	resctrlL3Info *rdtL3Info
)

type rdtL3Info struct {
	resctrlPath string
	info        catInfos
}

type catInfos struct {
	cacheIds []uint64
	unified  catInfo
	code     catInfo
	data     catInfo
}

type catInfo struct {
	cbmMask    bitmask
	minCbmBits uint64
}

type bitmask uint64

func getRdtL3Info() (*rdtL3Info, error) {
	var err error
	info := &rdtL3Info{}
	L3 := "L3"
	info.resctrlPath, err = getResctrlMountInfo()
	if err != nil {
		return info, fmt.Errorf("failed to detect resctrl mount info: %v", err)
	}
	klog.V(utils.DBG).Infof("resctrl filesystem %q has been detected", info.resctrlPath)

	infoPath := filepath.Join(info.resctrlPath, "info")
	if _, err := os.Stat(infoPath); err != nil {
		return info, fmt.Errorf("failed to get RDT info: %v", err)
	}

	cat := catInfos{}
	cats := map[string]*catInfo{
		"":     &cat.unified,
		"CODE": &cat.code,
		"DATA": &cat.data,
	}
	for k, v := range cats {
		dir := L3 + k
		path := filepath.Join(infoPath, dir)
		if _, err := os.Stat(path); err == nil {
			*v, err = getCatInfo(path)
			if err != nil {
				return info, fmt.Errorf("failed to get %s info from %q: %v", dir, path, err)
			}
		}
	}
	if cat.getInfo().Supported() {
		cat.cacheIds, err = getIds(L3, info.resctrlPath)
		if err != nil {
			return info, fmt.Errorf("failed to get %s cat cache IDs from %q: %v", L3, info.resctrlPath, err)
		}
	}
	info.info = cat
	return info, nil
}

func getIds(prefix string, resctrlPath string) ([]uint64, error) {
	str, err := readFileToString(filepath.Join(resctrlPath, "schemata"))
	if err != nil {
		return []uint64{}, fmt.Errorf("failed to get root schemata: %v", err)
	}

	lines := strings.Split(str, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineSplit := strings.SplitN(trimmed, ":", 2)

		if len(lineSplit) == 2 && strings.HasPrefix(lineSplit[0], prefix) {
			schema := strings.Split(lineSplit[1], ";")
			ids := make([]uint64, len(schema))

			for k, v := range schema {
				split := strings.Split(v, "=")
				if len(split) != 2 {
					return ids, fmt.Errorf("looks like an invalid schema %q", trimmed)
				}
				ids[k], err = strconv.ParseUint(split[0], 10, 64)
				if err != nil {
					return ids, fmt.Errorf("failed to parse cache id in %q: %v", trimmed, err)
				}
			}
			return ids, nil
		}
	}
	return []uint64{}, fmt.Errorf("no %s resources in root schemata", prefix)
}

func getResctrlMountInfo() (string, error) {
	f, err := os.Open(mountInfoPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	s := bufio.NewReader(f)
	for {
		txt, _, err := s.ReadLine()
		if err == io.EOF {
			break
		}
		line := string(txt)
		split := strings.Split(line, " ")
		if len(split) > 3 && split[2] == "resctrl" {
			return split[1], nil
		}
	}
	return "", fmt.Errorf("resctrl not found in " + mountInfoPath)
}

func getCatInfo(basepath string) (catInfo, error) {
	var err error
	info := catInfo{}

	info.cbmMask, err = readFileToBitmask(filepath.Join(basepath, "cbm_mask"))
	if err != nil {
		return info, err
	}
	info.minCbmBits, err = readFileToUint64(filepath.Join(basepath, "min_cbm_bits"))
	if err != nil {
		return info, err
	}
	return info, nil
}

func getContainerCpuUtilization(cgroupPath string) (float64, error) {
	var cpuTimeBefore, cpuTimeAfter int32 = 0, 0
	var err error
	croupVersion := getCgroupVersion()
	timeBefore := time.Now()
	if croupVersion == "v1" {
		cpuTimeBefore, err = getCgroupV1CpuTime(cgroupPath)
		if err != nil {
			return 0, err
		}
	} else {
		cpuTimeBefore, err = getCgroupV2CpuTime(cgroupPath)
		if err != nil {
			return 0, err
		}
	}
	time.Sleep(1 * time.Second)
	timeAfter := time.Now()
	if croupVersion == "v1" {
		cpuTimeAfter, err = getCgroupV1CpuTime(cgroupPath)
		if err != nil {
			return 0, err
		}
	} else {
		cpuTimeAfter, err = getCgroupV2CpuTime(cgroupPath)
		if err != nil {
			return 0, err
		}
	}
	res := float64(cpuTimeAfter-cpuTimeBefore) / float64((timeAfter.UnixNano()-timeBefore.UnixNano())/1000) // cpu nums, not %
	return math.Round(res*100) / 100, nil
}

func getCgroupVersion() string {
	_, err := os.Stat(cgroupControllerPath)
	if err == nil {
		return "v2"
	} else {
		return "v1"
	}
}

func getCgroupV1CpuTime(cgroupPath string) (int32, error) {
	filePath := filepath.Join(cgroupPath, "cpuacct.usage")
	_, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}
	cpuUsage, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 32)
	return int32(cpuUsage), err
}

func getCgroupV2CpuTime(cgroupPath string) (int32, error) {
	filePath := filepath.Join(cgroupPath, "cpu.stat")
	_, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		columns := strings.Split(line, " ")
		if columns[0] == "usage_usec" {
			cpuUsage, err := strconv.ParseInt(strings.TrimSpace(columns[1]), 10, 32)
			return int32(cpuUsage), err
		}
	}
	return 0, err
}

func readFileToBitmask(path string) (bitmask, error) {
	data, err := readFileToString(path)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseUint(data, 16, 64)
	return bitmask(value), err
}

func readFileToUint64(path string) (uint64, error) {
	data, err := readFileToString(path)
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(data, 10, 64)
}

func readFileToString(path string) (string, error) {
	data, err := os.ReadFile(path)
	return strings.TrimSpace(string(data)), err
}

func (i catInfos) getInfo() catInfo {
	switch {
	case i.code.Supported():
		return i.code
	case i.data.Supported():
		return i.data
	}
	return i.unified
}

func (i catInfo) Supported() bool {
	return i.cbmMask != 0
}

func (i catInfos) cbmMask() bitmask {
	mask := i.getInfo().cbmMask
	if mask != 0 {
		return mask
	}
	return bitmask(^uint64(0))
}

func (i catInfos) minCbmBits() uint64 {
	return i.getInfo().minCbmBits
}

func (b bitmask) msbOne() int {
	// Returns -1 for b == 0
	return 63 - bits.LeadingZeros64(uint64(b))
}

func (b bitmask) lsbOne() int {
	if b == 0 {
		return -1
	}
	return bits.TrailingZeros64(uint64(b))
}
