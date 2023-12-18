/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"os"

	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

func (s *RdtEngine) getRDTClassCPUUtilization(className string) (float64, error) {
	cpuUtilization := 0.0
	for appId, appInfo := range s.Apps {
		if appInfo.RdtAppInfo.RDTClass != className {
			continue
		}
		cpu, err := getContainerCpuUtilization(appInfo.RdtAppInfo.CgroupPath)
		if err != nil {
			klog.Warningf("Can not get container cpu utilization: %v", err)
			_, err := os.Stat(appInfo.RdtAppInfo.CgroupPath)
			if os.IsNotExist(err) {
				klog.V(utils.DBG).Infof("The directory %v does not exist, so delete the container from apps.", appInfo.RdtAppInfo.CgroupPath)
				delete(s.Apps, appId)
			}
			continue
		}
		cpuUtilization += cpu

	}
	return cpuUtilization, nil
}

func (s *RdtEngine) getRDTClassL3CacheMiss(className string) (float64, error) {
	klog.V(utils.DBG).Infof("getRDTClassL3CacheMiss %s", className)
	return 100, nil
}
