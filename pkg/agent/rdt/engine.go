/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdt

import (
	"errors"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	aggr "sigs.k8s.io/IOIsolation/pkg/api/aggregator"

	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/IOIsolation/pkg/agent/common"
)

var (
	MessageHeader = "RDT"
	TypeRDTIO     = "RDT"
)

const (
	CrNamespace string = "ioi-system"
)

var RDTObservedGeneration int64

// RDTEngine
type RDTEngine struct {
	agent.IOEngine
	NSWhitelist    map[string]struct{}
	resourceConfig v1.ResourceConfigSpec
	classInfos     map[string]agent.ClassInfo
}

func (e *RDTEngine) Type() string {
	return TypeRDTIO
}

func (e *RDTEngine) Initialize(coreClient *kubernetes.Clientset, client *versioned.Clientset, mtls bool) error {
	klog.Info("3.3 initializing the rdt engine.")
	a := agent.GetAgent()
	if (a.EnginesSwitch & agent.RDTSwitch) == 0 {
		klog.Infof("rdt engine is not enabled.")
		return errors.New("rdt label is not set")
	}

	a.RegisterEventHandler(agent.AdminChan, agent.EVENT_ADMIN, e.ProcessAdmin, MessageHeader, e, false)
	a.RegisterEventHandler(agent.ClassInfoChan, agent.EVENT_RDT, e.ProcessRDTData, MessageHeader, e, true)
	a.RegisterEventHandler(agent.ContainerInfoChan, agent.EVENT_NRI, e.ProcessNriData, MessageHeader, e, true)
	a.RegisterEventHandler(agent.PodInfoChan, agent.EVENT_RDT_CR, e.ProcessRDTCrData, MessageHeader, e, true)

	e.CoreClient = coreClient
	e.Clientset = client
	e.NodeName = os.Getenv("Node_Name")

	e.InitializeWorkloadGroup(common.RDTWorkloadType)

	go common.IOISubscribeRDT()

	return nil
}

func (e *RDTEngine) Uninitialize() error {
	return nil
}

func (e *RDTEngine) ProcessNriData(data agent.EventData) error {
	nriData := data.(*agent.ContainerData)
	klog.V(utils.INF).Info("Now in Process Nri Data.")
	for _, containerInfo := range nriData.ContainerInfos {
		klog.V(utils.DBG).Infof("pod name: %v, container name: %v, container id: %v.",
			containerInfo.PodName, containerInfo.ContainerName, containerInfo.ContainerId)
		_, ok := e.NSWhitelist[containerInfo.Namespace]
		if ok {
			klog.V(utils.DBG).Info("This is system pod, do not process nri data")
			continue
		}
		if containerInfo.Operation == utils.StartContainer {
			common.IOIRegisterRDT(containerInfo)
		} else if containerInfo.Operation == utils.StopContainer {
			common.IOIUnRegisterRDT(containerInfo)
		}
	}
	time.Sleep(time.Second / 5)
	return nil

}

func (e *RDTEngine) ProcessAdmin(data agent.EventData) error {
	adminData := data.(*agent.AdminResourceConfig)
	klog.Info("Now in rdt ProcessAdmin: ", *adminData)
	err := common.IOIConfigureInterval(utils.RDTIOIType, adminData.Interval.RdtInterval)
	if err != nil {
		klog.Error(err)
	}

	e.NSWhitelist = adminData.NamespaceWhitelist
	e.SetFlag(agent.AdminFlag)
	e.SetFlag(agent.PolicyFlag)
	e.SetFlag(agent.ProfileFlag)

	c := &agent.GetAgent().IOInfo

	if e.IsExecution() {
		if c.Spec.ResourceConfig == nil {
			klog.Errorf("c.Spec.ResourceConfig is nil")
			return nil
		}
	}

	return nil
}

func (e *RDTEngine) ProcessRDTData(data agent.EventData) error {
	klog.V(utils.INF).Info("Now in Process RDT service Data")

	classData := data.(*agent.ClassData)

	devices := make(map[string]v1.Device)
	for class := range classData.ClassInfos {
		devices[class] = v1.Device{}
	}
	e.resourceConfig.Devices = devices
	klog.V(utils.DBG).Info("rdt engine resourceConfig: ", e.resourceConfig)

	e.classInfos = classData.ClassInfos
	klog.V(utils.DBG).Info("rdt engine class info: ", e.classInfos)
	var rdtIO aggr.IOBandwidth
	rdtIO.DeviceBwInfo = make(map[string]*aggr.DeviceBandwidthInfo)
	rdtIO.IoType = aggr.IOType_RDT
	gen := RDTObservedGeneration
	for class, info := range classData.ClassInfos {
		ioStatus := make(map[string]*aggr.IOStatus)
		ioStatus[string(v1.WorkloadGA)] = &aggr.IOStatus{
			Pressure: info.Score,
		}
		rdtIO.DeviceBwInfo[class] = &aggr.DeviceBandwidthInfo{
			Status: ioStatus,
		}
	}
	klog.V(utils.DBG).Info("rdtIO: ", rdtIO.IoType, ", ", rdtIO.DeviceBwInfo)

	if agent.GetAgent().IOInfo.Spec.ResourceConfig == nil {
		return errors.New("can not find agent ioinfo spec resourceConfig")
	}
	agent.GetAgent().IOInfo.Spec.ResourceConfig[string(v1.RDT)] = e.resourceConfig
	err := common.UpdateNodeStaticIOInfoSpec(&agent.GetAgent().IOInfo, e.Clientset)
	if err != nil {
		klog.Errorf("updateLocalNodeRDTStatusInfo fail: %v", err)
		return err
	}
	err = agent.UpdateNodeIOStatus(e.NodeName, &rdtIO, gen)
	if err != nil {
		klog.V(utils.INF).Info("updateLocalNodeRDTStatusIOInfo fail" + err.Error())
	} else {
		klog.V(utils.INF).Info("updateLocalNodeRDTStatusIOInfo successfully!")
	}
	return nil
}

func (e *RDTEngine) ProcessRDTCrData(data agent.EventData) error {
	rdtData := data.(*agent.PodData)
	klog.V(utils.INF).Info("now in ProcessRDTCrData: ", *rdtData)
	RDTObservedGeneration = rdtData.Generation
	return nil
}

func init() {
	a := agent.GetAgent()

	engine := &RDTEngine{
		agent.IOEngine{
			Flag:          0,
			ExecutionFlag: agent.ProfileFlag | agent.PolicyFlag | agent.AdminFlag,
		},
		make(map[string]struct{}),
		v1.ResourceConfigSpec{},
		make(map[string]agent.ClassInfo, 0),
	}

	a.RegisterEngine(engine)
}
