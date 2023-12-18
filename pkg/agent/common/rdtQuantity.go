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
	rdtQuantity "sigs.k8s.io/IOIsolation/pkg/scheduler/rdt-quantity"
)

func IOIRegisterRDTQuantity(containerInfo agent.ContainerInfo) {
	var rdtinfo utils.RDTAppInfo
	rdtinfo.CgroupPath = containerInfo.ContainerCGroupPath
	rdtinfo.ContainerName = containerInfo.ContainerName
	rdtinfo.PodUid = containerInfo.PodUid
	rdtinfo.PodName = containerInfo.PodName

	anno := rdtQuantity.RDTAnnotationResource{}
	if containerInfo.RDTQuantityAnnotation != "" {
		if err := json.Unmarshal([]byte(containerInfo.RDTQuantityAnnotation), &anno); err != nil {
			klog.Errorf("cannot parse %v to rdt quantity requests/limits: %v", containerInfo.RDTQuantityAnnotation, err)
			return
		}
		if anno.Class == "" {
			anno.Class = containerInfo.PodName
		}
		if anno.Requests.L3 == "" && anno.Requests.Mb == "" && anno.Limits.L3 == "" && anno.Limits.Mb == "" {
			anno.Class = "BE"
		} else if anno.Requests.L3 == "" || anno.Requests.Mb == "" {
			klog.Errorf("pod %v does not have complete rdt quantity annotation", containerInfo.PodName)
		} else {
			if anno.Limits.L3 == "" {
				anno.Limits.L3 = anno.Requests.L3
			}
			if anno.Limits.Mb == "" {
				anno.Limits.Mb = anno.Requests.Mb
			}
		}
		var err error
		rdtinfo.RDTClass = anno.Class
		rdtinfo.RDTClassInfo.L3CacheLim, err = anno.Requests.L3.Parse()
		if err != nil {
			klog.Errorf("cannot parse %v to rdt quantity request/limit: %v", anno.Requests.L3, err)
		}
		rdtinfo.RDTClassInfo.L3CacheLim, err = anno.Limits.L3.Parse()
		if err != nil {
			klog.Errorf("cannot parse %v to rdt quantity request/limit: %v", anno.Limits.L3, err)
		}
		rdtinfo.RDTClassInfo.MbaReq, err = anno.Requests.Mb.Parse()
		if err != nil {
			klog.Errorf("cannot parse %v to rdt quantity request/limit: %v", anno.Requests.Mb, err)
		}
		rdtinfo.RDTClassInfo.MbaLim, err = anno.Limits.Mb.Parse()
		if err != nil {
			klog.Errorf("cannot parse %v to rdt quantity request/limit: %v", anno.Limits.Mb, err)
		}
	}

	data, json_err := json.Marshal(rdtinfo)
	if json_err != nil {
		klog.Warningf("Marshal is failed")
	}
	klog.V(utils.DBG).Infof("containerInfo: %+v", rdtinfo)
	agent.ClientHandlerChan <- &pb.RegisterAppRequest{AppName: containerInfo.ContainerId, AppInfo: string(data), IoiType: utils.RDTQuantityIOIType}
}

func IOIUnRegisterRDTQuantity(containerInfo agent.ContainerInfo) {
	agent.ClientHandlerChan <- &pb.UnRegisterAppRequest{AppId: containerInfo.ContainerId, IoiType: utils.RDTQuantityIOIType}
}

func IOISubscribeRDTQuantity() {
	if agent.IoiClient == nil {
		klog.Errorf("ioi client is nil")
		return
	}
	for {
		stream, err := agent.IoiClient.Subscribe(agent.IoiCtx, &pb.SubscribeContext{
			IoiType: utils.RDTQuantityIOIType,
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
			result.T = 1
			result.ClassInfos = classInfoes
			agent.ClassInfoChan <- &result
		}
	}
}
