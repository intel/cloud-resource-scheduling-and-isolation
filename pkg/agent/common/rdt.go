/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"encoding/json"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

func IOIRegisterRDT(containerInfo agent.ContainerInfo) {
	var rdtinfo utils.RDTAppInfo
	rdtinfo.CgroupPath = containerInfo.ContainerCGroupPath
	rdtinfo.ContainerName = containerInfo.ContainerName
	rdtinfo.RDTClass = containerInfo.RDTAnnotation
	data, json_err := json.Marshal(rdtinfo)
	if json_err != nil {
		klog.Warningf("Marshal is failed")
	}
	agent.ClientHandlerChan <- &pb.RegisterAppRequest{AppName: containerInfo.ContainerId, AppInfo: string(data), IoiType: utils.RDTIOIType}
}

func IOIUnRegisterRDT(containerInfo agent.ContainerInfo) {
	agent.ClientHandlerChan <- &pb.UnRegisterAppRequest{AppId: containerInfo.ContainerId, IoiType: utils.RDTIOIType}
}

func IOISubscribeRDT() {
	if agent.IoiClient == nil {
		klog.Errorf("ioi client is nil")
		return
	}
	for {
		stream, err := agent.IoiClient.Subscribe(agent.IoiCtx, &pb.SubscribeContext{
			IoiType: utils.RDTIOIType,
		}, grpc.WaitForReady(true))
		if err != nil {
			klog.Warning("Subscribe error: ", err)
		} else {
			klog.Info("Subscribe succeed")
		}
		for {
			resp, err := stream.Recv()
			if err != nil {
				klog.Warning("Waiting for subscribe restart, Stream recv failed error: ", err)
				break
			}
			rdtClassesInfo := resp.GetRdtInfo().ClassInfo
			var result agent.ClassData
			classInfoes := make(map[string]agent.ClassInfo)
			for class, info := range rdtClassesInfo {
				classInfoes[class] = agent.ClassInfo{
					Score: info,
				}
			}
			klog.V(utils.DBG).Info("classInfos: ", classInfoes)
			result.T = 0
			result.ClassInfos = classInfoes
			agent.ClassInfoChan <- &result
		}
	}
}
