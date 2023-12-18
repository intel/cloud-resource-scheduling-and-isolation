/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"
	"strconv"
	"strings"
)

var (
	ResctrlRoot   = "/sys/fs/resctrl"
	CgroupRoot    = "/sys/fs/cgroup"
	conPrefix     = "cri-containerd-"
	conSuffix     = ".scope"
	MonitorGroups = "mon_groups"

	RuntimeTypeDocker     = "docker"
	RuntimeTypeContainerd = "containerd"
	RuntimeTypePouch      = "pouch"
	RuntimeTypeCrio       = "cri-o"
	RuntimeTypeUnknown    = "unknown"
)

func Unmarshal(jsonOrYaml string, obj interface{}) error {
	if err := json.Unmarshal([]byte(jsonOrYaml), obj); err != nil {
		if err := yaml.Unmarshal([]byte(jsonOrYaml), obj); err != nil {
			return err
		}
	}
	return nil
}

func ReadCPUTasks(path string) ([]int32, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	tasksStr := strings.Trim(string(data), "\n")
	var values []int32
	lines := strings.Split(tasksStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) <= 0 {
			continue
		}
		v, err := strconv.ParseInt(line, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("cannot parse cgroup value of line %s, err: %v", line, err)
		}
		values = append(values, int32(v))
	}
	return values, nil
}

func GetMonitorGroupPath(group rdt.MonitorGroup) string {
	path := ResctrlRoot
	if group.ControlGroupId != "" {
		path += "/" + group.ControlGroupId
	}
	if group.ControlGroupId == group.Id {
		return path
	} else {
		path += "/" + MonitorGroups
		if group.Id != "" {
			path += "/" + group.Id
		}
	}

	return path
}

func GetSubCgroupPath(cgroupPath string) (map[string]string, error) {
	appCgroupPath := CgroupRoot + cgroupPath
	files, err := os.ReadDir(appCgroupPath)
	if err != nil {
		return nil, err
	}
	containers := make(map[string]string)
	for _, file := range files {
		// app is pod
		if file.IsDir() && strings.HasPrefix(file.Name(), conPrefix) {
			cid := strings.TrimSuffix(strings.TrimPrefix(file.Name(), conPrefix), conSuffix)
			containers[cid] = filepath.Join(appCgroupPath, file.Name())
		}
	}
	if len(containers) == 0 {
		// TODO: parse ID from cgroupPath
		cid := strings.TrimSuffix(strings.TrimPrefix(cgroupPath, conPrefix), conSuffix)
		containers[cid] = appCgroupPath
	}
	return containers, nil
}

func WriteTaskIDsToFile(taskids []int32, filename string) error {

	klog.Infof("executor try to write ids %v to %s", taskids, filename)
	// 创建或打开文件
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read all tasks
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	s := strings.Trim(string(data), "\n")
	// content: "%d\n%d\n%d\n..."
	curIds := make(map[int32]int)
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) <= 0 {
			continue
		}
		v, err := strconv.ParseInt(line, 10, 32)
		if err != nil {
			return fmt.Errorf("cannot parse cgroup value of line %s, err: %v", line, err)
		}
		curIds[int32((v))] = 1
	}

	// only append the non-mapped ids
	var newIds []int32
	for _, id := range taskids {
		if _, ok := curIds[id]; !ok {
			newIds = append(newIds, id)
		}
	}

	klog.Infof("executor try to write new ids %v, curIds is %v, all Ids is %v to %s", newIds, curIds, taskids, filename)
	// 将每个 taskid 写入文件
	for _, id := range newIds {
		_, err := file.WriteString(strconv.FormatInt(int64(id), 10) + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

func GetContainerPath(id string) (string, string, error) {
	return RuntimeTypeContainerd, fmt.Sprintf("cri-containerd-%s.scope", id), nil
}

func GetContainerCgroupPathById(cgroupParent, containerId string) (string, error) {
	_, containerDir, err := GetContainerPath(containerId)
	if err != nil {
		return "", err
	}
	return filepath.Join(
		cgroupParent,
		containerDir,
	), nil
}
