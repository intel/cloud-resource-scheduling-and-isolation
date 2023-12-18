/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/IOIsolation/pkg/agent/common"
	"sigs.k8s.io/IOIsolation/pkg/agent/metrics"
	aggr "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
)

const (
	conPrefix   string = "/cri-containerd-"
	podPrefix   string = "/sys/fs/cgroup"
	CrNamespace string = "ioi-system"
	CNI         string = "calico" //not used in Q2
	GATEWAY     string = ""       //not used in Q2
	emptyNetAnn string = "'{\"network\":[]}'"
)

var (
	MessageHeader = "Net IO"
	TypeNetworkIO = "NetIO"
)

// var nicInfo common.NetInfo
var NicInfos = make(map[string]utils.NicInfo) // key:net id
var podMutex sync.Mutex
var NetObservedGeneration int64
var NodeIps []common.NodeIP // from profile

type PodRegInfo struct {
	// NRI information
	NriOperation int
	Ns           string
	CGroupPath   string
	// CR information
	CrOperation int
	devIds      []string
	Group       string
}

// NetEngine
type NetEngine struct {
	agent.IOEngine
	Profile        utils.NicInfos
	NSWhitelist    map[string]struct{}
	Queues         int
	sysPool        int
	MetricCache    NetMetric
	EngineCache    NetMetric
	resourceConfig v1.ResourceConfigSpec
	podRegInfos    map[string]*PodRegInfo // key:podUid
}

func (e *NetEngine) Type() string {
	return TypeNetworkIO
}

func (e *NetEngine) Initialize(coreClient *kubernetes.Clientset, client *versioned.Clientset, mtls bool) error {
	klog.Info("3.3 initializing the net engine")
	a := agent.GetAgent()
	if (a.EnginesSwitch & agent.NetSwitch) == 0 {
		klog.V(utils.DBG).Info("Net engine is not enabled")
		return errors.New("net label is not set")
	}

	a.RegisterEventHandler(agent.AdminChan, agent.EVENT_ADMIN, e.ProcessAdmin, MessageHeader, e, false)
	a.RegisterEventHandler(agent.ProfileChan, agent.EVENT_PROF_NET, e.ProcessProfile, MessageHeader, e, true)
	a.RegisterEventHandler(agent.PolicyChan, agent.EVENT_POLICY, e.ProcessPolicy, MessageHeader, e, false)
	a.RegisterEventHandler(agent.PodInfoChan, agent.EVENT_NRI, e.ProcessNriData, MessageHeader, e, true)
	a.RegisterEventHandler(agent.PodInfoChan, agent.EVENT_NET, e.ProcessNicIOData, MessageHeader, e, true)
	a.RegisterEventHandler(agent.PodInfoChan, agent.EVENT_NET_CR, e.ProcessNetworkCrData, MessageHeader, e, true)

	e.CoreClient = coreClient
	e.Clientset = client
	e.NodeName = agent.GetAgent().IOInfo.Spec.NodeName
	e.GpPolicy = make([]*agent.Policy, utils.GroupIndex)

	// WorkloadGroup
	e.InitializeWorkloadGroup(common.NetWorkloadType)

	// Init profiler
	go StartNetProfiler()

	// Init Metric
	e.MetricCache.InitMetric()
	metrics.GetMetricEngine().AddMetric("Network", &e.MetricCache)
	e.EngineCache.PodCache[e.NodeName] = &PodBw{}
	e.EngineCache.Cache[e.NodeName] = make(map[string]*NetBw)

	return nil
}

func (e *NetEngine) Uninitialize() error {
	return nil
}

func StartNetProfiler() {
	// Assume for a nic
	for {
		nicReady, err := common.CheckGRPCStatus()
		if err == nil && nicReady {
			klog.V(utils.DBG).Info("CheckGRPCStatus success, now AnalysisNicProfile")
			allProfile := common.IOIGetServiceInfoNet()

			if allProfile != nil {
				agent.ProfileChan <- allProfile
			}
		}

		time.Sleep(10 * time.Minute)
	}
}

func (e *NetEngine) updateLocalNodeNICStatusIOInfo() error {
	var netIO aggr.IOBandwidth
	gen := NetObservedGeneration

	for nicId, nicInfo := range NicInfos {
		send, recv, pressure := e.calNicAllocatable(nicId)
		e.UpdateNetMetrics()

		netIO.IoType = aggr.IOType_NetworkIO
		ioStatus := make(map[string]*aggr.IOStatus)

		var gaStatus aggr.IOStatus
		var beStatus aggr.IOStatus
		gaStatus.In = recv[utils.GaIndex]
		gaStatus.Out = send[utils.GaIndex]
		gaStatus.Total = gaStatus.In + gaStatus.Out
		if pressure[utils.GaIndex] {
			gaStatus.Pressure = 1
		} else {
			gaStatus.Pressure = 0
		}

		beStatus.In = recv[utils.BeIndex]
		beStatus.Out = send[utils.BeIndex]
		beStatus.Total = beStatus.In + beStatus.Out
		if pressure[utils.BeIndex] {
			beStatus.Pressure = 1
		} else {
			beStatus.Pressure = 0
		}

		ioStatus["GA"] = &gaStatus // BT status is same as GA for now
		ioStatus["BE"] = &beStatus
		deviceInfo := &aggr.DeviceBandwidthInfo{
			Name:   nicInfo.Name,
			Status: ioStatus,
		}
		netIO.DeviceBwInfo = make(map[string]*aggr.DeviceBandwidthInfo)
		netIO.DeviceBwInfo[nicId] = deviceInfo
	}
	err := agent.UpdateNodeIOStatus(e.NodeName, &netIO, gen)

	return err
}

func (e *NetEngine) ProcessProfile(data agent.EventData) error {
	profileData := data.(*agent.AllProfile)
	klog.V(utils.INF).Info("now in net ProcessProfile: ", *profileData)
	var masterNic utils.NicInfo

	c := &agent.GetAgent().IOInfo
	resourceConfig := &e.resourceConfig
	if !reflect.DeepEqual(e.Profile, profileData.NProfile) {
		e.Profile = profileData.NProfile
		resourceConfig.Devices = make(map[string]v1.Device)
		availRatio := (100.0 - float64(e.sysPool)) / 100.0
		for nicId, nicInfo := range e.Profile {
			var newDevice v1.Device
			if nicInfo.Type == "masterNic" {
				newDevice.Type = v1.DefaultDevice
				// Only consider emptydir now.

				nicInfo.SndGroupBPS = nicInfo.TotalBPS * availRatio // reserve 20% for system
				nicInfo.RcvGroupBPS = nicInfo.TotalBPS * availRatio // reserve 20% for system

				NicInfos[nicId] = nicInfo
				var nodeIp common.NodeIP
				nodeIp.InterfaceName = nicInfo.Name
				nodeIp.IpAddr = nicInfo.IpAddr
				nodeIp.InterfaceType = nicInfo.Type
				nodeIp.MacAddr = nicId
				NodeIps = append(NodeIps, nodeIp)
				masterNic = nicInfo
			} else {
				newDevice.Type = v1.OtherDevice
			}

			newDevice.Name = nicInfo.Name
			newDevice.DefaultIn = common.MinPodNetInBw
			newDevice.DefaultOut = common.MinPodNetOutBw
			newDevice.CapacityIn = nicInfo.TotalBPS * availRatio  // reserve 20% for system
			newDevice.CapacityOut = nicInfo.TotalBPS * availRatio // reserve 20% for system
			newDevice.CapacityTotal = newDevice.CapacityIn + newDevice.CapacityOut
			resourceConfig.Devices[nicId] = newDevice

			if _, ok := e.WgDevices[nicId]; !ok {
				for i := utils.GaIndex; i < utils.GroupIndex; i++ {
					wg := agent.WorkloadGroup{
						IoType:        common.NetWorkloadType,
						GroupSettable: false,
						WorkloadType:  i,
						LimitPods:     make(map[string]*utils.Workload),
						ActualPods:    make(map[string]*utils.Workload),
						RequestPods:   make(map[string]*utils.Workload),
						PodsLimitSet:  make(map[string]*utils.Workload),
						GroupLimit:    &utils.Workload{},
						PodName:       make(map[string]string),
					}
					e.AddWorkloadGroup(wg, nicId)
				}
			}
		}

		e.SetFlag(agent.ProfileFlag)
		if e.IsExecution() {
			if c.Spec.ResourceConfig == nil {
				klog.Errorf("c.Spec.ResourceConfig is nil")
				return nil
			}
			c.Spec.ResourceConfig[string(v1.NetworkIO)] = e.resourceConfig
			err := common.UpdateNodeStaticIOInfoSpec(c, e.Clientset)
			if err != nil {
				return err
			}
		}
		// Configure base value
		klog.V(utils.DBG).Infof("QueueNum:", masterNic.QueueNum)
		groupQueue := 0
		if masterNic.QueueNum > 0 {
			groupQueue = (masterNic.QueueNum-2)/2 - ((masterNic.QueueNum-2)/2)%2
		}

		NetGroups := []common.Group{
			{Id: "0", Types: 0, Pool: int(masterNic.SndGroupBPS), Queues: groupQueue},
			{Id: "1", Types: 1, Pool: int(masterNic.SndGroupBPS), Queues: groupQueue},
			{Id: "2", Types: 2, Pool: resourceConfig.BEPool * int(masterNic.SndGroupBPS) / 100, Queues: groupQueue},
		}
		klog.V(utils.DBG).Info("now start to IOIConfigureNet")
		GroupSettableInfo := common.IOIConfigureNet(CNI, GATEWAY, NodeIps, NetGroups)
		for nicId, nicInfo := range NicInfos {
			for index, set := range GroupSettableInfo.SettableInfo {
				e.WgDevices[nicId][index].GroupSettable = set.GroupSettable
			}
			klog.V(utils.DBG).Infof("nicInfo_:", nicInfo)
		}
		klog.V(utils.DBG).Infof("GroupSettableInfo:", GroupSettableInfo)
	} else {
		klog.V(utils.DBG).Info("nic profile is the same as last time, no need to change")
	}
	return nil
}

func (e *NetEngine) ProcessPolicy(data agent.EventData) error {
	policyData := data.(*agent.PolicyConfig)
	klog.V(utils.INF).Info("now in net ProcessPolicy: ", *policyData)
	gaSet := false
	beSet := false
	btSet := false
	for policyType, policy := range *policyData {
		switch policyType {
		case "NetGArules":
			e.GpPolicy[utils.GaIndex] = &policy
			gaSet = true
		case "NetBTrules":
			e.GpPolicy[utils.BtIndex] = &policy
			btSet = true
		case "NetBErules":
			e.GpPolicy[utils.BeIndex] = &policy
			beSet = true
		default:
			klog.V(utils.DBG).Infof("the policyType %s, policy +%v", policyType, policy)
		}
	}

	if gaSet && beSet && btSet {
		e.SetFlag(agent.PolicyFlag)
		if e.IsExecution() {
			c := &agent.GetAgent().IOInfo
			c.Spec.ResourceConfig[string(v1.NetworkIO)] = e.resourceConfig
			err := common.UpdateNodeStaticIOInfoSpec(c, e.Clientset)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *NetEngine) ProcessAdmin(data agent.EventData) error {
	adminData := data.(*agent.AdminResourceConfig)
	klog.V(utils.INF).Info("now in net ProcessAdmin: ", *adminData)
	err := common.IOIConfigureInterval(utils.NetIOIType, adminData.Interval.NetInterval)
	if err != nil {
		klog.Error(err)
	}

	c := &agent.GetAgent().IOInfo
	resourceConfig := &e.resourceConfig
	resourceConfig.BEPool = adminData.NetBePool
	e.sysPool = adminData.NetSysPool
	common.MinPodNetInBw = float64(adminData.Net_min_pod_in_bw)
	common.MinPodNetOutBw = float64(adminData.Net_min_pod_out_bw)

	e.NSWhitelist = adminData.NamespaceWhitelist

	e.SetFlag(agent.AdminFlag)
	if e.IsExecution() {
		c.Spec.ResourceConfig[string(v1.NetworkIO)] = e.resourceConfig
		err := common.UpdateNodeStaticIOInfoSpec(c, e.Clientset)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *NetEngine) GetPodGroup(podId string) string {
	var group string
	for _, wgs := range e.WgDevices {
		for gi := utils.GaIndex; gi < utils.GroupIndex; gi++ {
			wg := wgs[gi]
			if wg.WorkloadType != -1 {
				// check wg.RequestPods
				_, ok := wg.RequestPods[podId]
				if ok {
					group = fmt.Sprint(gi)
				}
			}
		}
	}
	return group
}

func (e *NetEngine) RegisterPod() {
	podMutex.Lock()
	for podId, pRegInfo := range e.podRegInfos {
		group := pRegInfo.Group
		if group == "" {
			klog.Errorf("group is nil, CR pod is not ready %s", podId)
			continue
		}
		netNs := pRegInfo.Ns
		if netNs == "" {
			klog.Errorf("Ns is nil, Nri pod is not ready %s", podId)
			continue
		}
		cgroupPath := pRegInfo.CGroupPath
		if cgroupPath == "" {
			klog.Errorf("cgroupPath is nil, Nri pod is not ready %s", podId)
			continue
		}

		devIds := pRegInfo.devIds
		if len(devIds) == 0 { // not set group
			klog.V(utils.INF).Infof("devIds is empty, podId: %s", podId)
			continue
		}

		Operation := pRegInfo.NriOperation & pRegInfo.CrOperation
		if Operation == 0 {
			klog.V(utils.INF).Infof("network RegisterPod NriOperation:%d , CrOperation", pRegInfo.NriOperation, pRegInfo.CrOperation)
			continue
		}

		// clear operation in cr and nri
		pRegInfo.CrOperation = Operation ^ pRegInfo.CrOperation
		pRegInfo.NriOperation = Operation ^ pRegInfo.NriOperation
		// delete pod and clear register information
		if Operation&utils.StopPod == utils.StopPod {
			delete(e.podRegInfos, podId)
		}
		podMutex.Unlock()

		// Execute register and unregister operation
		if Operation&utils.RunPod == utils.RunPod {
			klog.Infof("IOIRegisterNet pod operation: %d, podId: %d, netNs: %d", Operation, podId, netNs)
			common.IOIRegisterNet(podId, group, netNs, NodeIps, cgroupPath)
		}
		if Operation&utils.StopPod == utils.StopPod {
			klog.Infof("IOIRegisterNet pod operation: %d, podId: %d, netNs: %d", Operation, podId, netNs)
			common.IOIUnRegisterNet(podId)
		}
		if Operation&utils.StopContainer == utils.StopContainer {
			klog.V(utils.INF).Info("StopContainer event from nri")
		}
		// fixme for multiple nics
		err := e.updateLocalNodeNICStatusIOInfo()
		if err != nil {
			klog.Errorf("updateLocalNodeNICStatusIOInfo failed, err: %v", err)
		}
		podMutex.Lock()
	}
	podMutex.Unlock()
}

func (e *NetEngine) ProcessNriData(data agent.EventData) error {
	nriData := data.(*agent.PodData)
	klog.V(utils.INF).Info("now in ProcessNriData")

	// get nic id from pod id
	podEvent := nriData.PodEvents["dev"]
	for podId, podInfo := range podEvent {
		klog.V(utils.INF).Infof("podId: %s, Operation:%d", podId, podInfo.Operation)
		// Ignore system pod
		_, NetOk := e.NSWhitelist[podInfo.Namespace]
		if NetOk {
			klog.V(utils.DBG).Info("This is system pod, do not process nri data")
			continue
		}

		netNs := ""

		klog.V(utils.INF).Infof("podId: %s, NetNamespace:%d", podId, podInfo.NetNamespace)
		klog.V(utils.INF).Infof("podId: %s, CGroupPath:%s", podId, podInfo.CGroupPath)

		for _, ns := range podInfo.NetNamespace {
			if ns != nil && ns.Type == "network" {
				netNs = ns.Path
			}
		}

		cgroupPath := podPrefix + podInfo.CGroupPath
		podMutex.Lock()
		if pInfo, ok := e.podRegInfos[podId]; !ok || pInfo == nil {
			pRegInfo := &PodRegInfo{
				NriOperation: podInfo.Operation,
				Ns:           netNs,
				CGroupPath:   cgroupPath,
			}
			e.podRegInfos[podId] = pRegInfo
			klog.V(utils.INF).Infof("podId: %s, CGroupPath:%s", podId, cgroupPath)
		} else {
			pRegInfo := e.podRegInfos[podId]
			pRegInfo.NriOperation = pRegInfo.NriOperation | podInfo.Operation
			if netNs != "" {
				pRegInfo.Ns = netNs
			}
			pRegInfo.CGroupPath = cgroupPath
			klog.V(utils.INF).Infof("podId: %s, netNs:%s", podId, netNs)
		}
		podMutex.Unlock()

		e.RegisterPod()
	}

	return nil
}

func (e *NetEngine) ProcessNicIOData(data agent.EventData) error {
	netIOData := data.(*agent.PodData)
	klog.V(utils.INF).Info("now in ProcessNicIOData")
	// Update the limit data after runtime update
	for nicId, nicInfo := range netIOData.PodEvents {
		klog.V(utils.DBG).Info("Start to process nic data, nicId: ", nicId)
		for podId, podInfo := range nicInfo {
			klog.V(utils.DBG).Infof("Start podId: %s in: %f, out: %f", podId, podInfo.Actual.InBps, podInfo.Actual.OutBps)
			//workloadType := podInfo.WorkloadType
			for _, wg := range e.WgDevices[nicId] {
				if wg.RequestPods[podId] != nil { // pod is in the group
					wg.ActualPods[podId] = podInfo.Actual
				}
			}
			e.calNicAllocatable(nicId)
		}
	}

	podsRealBw := make(map[string]PodInfos)
	for nicId, podEvent := range netIOData.PodEvents {
		podRealBw := make(map[string]PodInfo)
		for podId, podInfo := range podEvent {
			var rif PodInfo
			for _, wg := range e.WgDevices[nicId] {
				if wg.RequestPods[podId] != nil { // pod is in the group
					rif.PodName = wg.PodName[podId]
					rif.Type = utils.IndexToType[wg.WorkloadType]
					rif.Workload = podInfo.Actual
				}
			}
			podRealBw[rif.PodName] = rif
		}
		if nicInfo, ok := NicInfos[nicId]; ok {
			podsRealBw[nicInfo.Name] = podRealBw
		}
	}

	e.EngineCache.PodCache[e.NodeName].PodTranBw = podsRealBw
	klog.V(utils.INF).Info("Send node agent network data to metrics,PodTranBw:  ", e.EngineCache.PodCache[e.NodeName].PodTranBw)
	e.UpdateNetMetrics()

	return nil
}

func (e *NetEngine) ProcessNetworkCrData(data agent.EventData) error {
	if data == nil {
		return errors.New("event data is nil")
	}
	netIOData := data.(*agent.PodData)
	visited := make(map[string]struct{})
	NetObservedGeneration = netIOData.Generation
	klog.V(utils.INF).Infof("now in ProcessNetworkCrData NetObservedGeneration: %d", NetObservedGeneration)
	for nicId, podEvent := range netIOData.PodEvents {
		klog.V(utils.INF).Infof("nicId: %d", nicId)
		if _, ok := e.WgDevices[nicId]; !ok {
			klog.Warningf("diskId: %d is not profiled, so the pod can not be registed", nicId)
			continue
		}
		for podId, podInfo := range podEvent {
			klog.V(utils.INF).Infof("podId: %f, Request: %v, Limit: %v", podId, podInfo.Request, podInfo.Limit)
			if podInfo.PodName == "" {
				klog.V(utils.INF).Infof("podName is empty, podId: %s", podId)
				continue
			}
			workloadType := podInfo.WorkloadType
			e.WgDevices[nicId][workloadType].RequestPods[podId] = podInfo.Request
			e.WgDevices[nicId][workloadType].LimitPods[podId] = podInfo.Limit
			e.WgDevices[nicId][workloadType].PodName[podId] = podInfo.PodName
			e.WgDevices[nicId][workloadType].WorkloadType = workloadType

			visited[podId] = struct{}{}

			podMutex.Lock()
			if pInfo, ok := e.podRegInfos[podId]; !ok || pInfo == nil {
				pRegInfo := &PodRegInfo{
					CrOperation: podInfo.Operation,
					devIds:      []string{nicId},
					Group:       fmt.Sprint(workloadType),
				}
				e.podRegInfos[podId] = pRegInfo
			} else {
				pRegInfo := e.podRegInfos[podId]
				pRegInfo.CrOperation = pRegInfo.CrOperation | podInfo.Operation
				pRegInfo.devIds = append(pRegInfo.devIds, nicId)
				pRegInfo.Group = fmt.Sprint(workloadType)
			}
			podMutex.Unlock()
		}
	}
	for _, wgs := range e.WgDevices {
		for gi := utils.GaIndex; gi < utils.GroupIndex; gi++ {
			common.DeleteOutdatePod(&(wgs[gi]), visited)
		}
	}

	podMutex.Lock()
	for podId, pRegInfo := range e.podRegInfos {
		if _, ok := visited[podId]; !ok {
			pRegInfo.CrOperation = pRegInfo.CrOperation | utils.StopPod
		} else {
			pRegInfo.CrOperation = pRegInfo.CrOperation | utils.RunPod
		}
	}
	podMutex.Unlock()
	//Register pod
	e.RegisterPod()

	return nil
}

func (e *NetEngine) calNicAllocatable(nicId string) ([]float64, []float64, []bool) {
	klog.V(utils.DBG).Info("now start to calNicAllocatable")

	if _, ok := NicInfos[nicId]; !ok {
		klog.Warningf("nicId: %s is not profiled, so the pod can not be registed", nicId)
		return nil, nil, nil
	}

	if _, ok := e.WgDevices[nicId]; !ok {
		klog.Warningf("nicId: %s is not profiled, so the pod can not be registed", nicId)
		return nil, nil, nil
	}

	nicInfo := NicInfos[nicId]

	totalSizeSnd := nicInfo.SndGroupBPS
	totalSizeRcv := nicInfo.RcvGroupBPS

	bePoolRatio := float64(e.resourceConfig.BEPool) / 100
	bePoolSizeSnd := bePoolRatio * nicInfo.SndGroupBPS
	bePoolSizeRcv := bePoolRatio * nicInfo.RcvGroupBPS

	allocRead := make([]float64, len(e.WgDevices[nicId]))
	allocWrite := make([]float64, len(e.WgDevices[nicId]))
	pressure := make([]bool, len(e.WgDevices[nicId]))
	expect := make([]utils.Workload, len(e.WgDevices[nicId]))

	gaRequest := common.CalculateGroupBw(e.WgDevices[nicId][utils.GaIndex].RequestPods, nicId)
	btRequest := common.CalculateGroupBw(e.WgDevices[nicId][utils.BtIndex].RequestPods, nicId)

	allocRequest := utils.Workload{
		InBps:  totalSizeRcv - bePoolSizeRcv - gaRequest.InBps - btRequest.InBps,
		OutBps: totalSizeSnd - bePoolSizeSnd - gaRequest.OutBps - btRequest.OutBps,
	}

	allocWrite[utils.GaIndex] = allocRequest.OutBps
	allocRead[utils.GaIndex] = allocRequest.InBps

	allocWrite[utils.BtIndex] = allocRequest.OutBps
	allocRead[utils.BtIndex] = allocRequest.InBps

	allocWrite[utils.BeIndex] = bePoolSizeSnd
	allocRead[utils.BeIndex] = bePoolSizeRcv

	if gaRequest.InBps == 0 {
		expect[utils.GaIndex].InBps = allocRead[utils.GaIndex]
	} else {
		expect[utils.GaIndex].InBps = gaRequest.InBps
	}

	if gaRequest.OutBps == 0 {
		expect[utils.GaIndex].OutBps = allocWrite[utils.GaIndex]
	} else {
		expect[utils.GaIndex].OutBps = gaRequest.OutBps
	}

	if btRequest.OutBps > 0 {
		expect[utils.BtIndex].OutBps = allocWrite[utils.BtIndex]/btRequest.OutBps + 1
	} else {
		expect[utils.BtIndex].OutBps = 1
	}

	if btRequest.InBps > 0 {
		expect[utils.BtIndex].InBps = allocRead[utils.BtIndex]/btRequest.InBps + 1
	} else {
		expect[utils.BtIndex].InBps = 1
	}

	btActual := common.CalculateGroupBw(e.WgDevices[nicId][utils.BtIndex].ActualPods, nicId)
	var btRuntimeOut float64
	var btRuntimeIn float64
	if btRequest.OutBps > btActual.OutBps {
		btRuntimeOut = btRequest.OutBps
	} else {
		btRuntimeOut = btActual.OutBps
	}
	if btRequest.InBps > btActual.InBps {
		btRuntimeIn = btRequest.InBps
	} else {
		btRuntimeIn = btActual.InBps
	}
	expect[utils.BeIndex].OutBps = totalSizeSnd - gaRequest.OutBps - btRuntimeOut
	expect[utils.BeIndex].InBps = totalSizeRcv - gaRequest.InBps - btRuntimeIn

	for idx := range e.WgDevices[nicId] {
		wlg := &e.WgDevices[nicId][idx]
		request := common.CalculateGroupBw(wlg.RequestPods, nicId)
		dev := common.DevInfo{
			Id:            nicId,
			Name:          nicInfo.Name,
			DevType:       "NIC",
			DevMajorMinor: nicInfo.Name,
			DInfo:         nil,
			NInfo:         &nicInfo,
		}
		policy := e.GpPolicy[wlg.WorkloadType]
		common.SetLimit(wlg, policy, expect[idx], request, dev, int32(idx))
	}

	return allocWrite, allocRead, pressure
}

func (e *NetEngine) UpdateNetMetrics() {
	// Prepare data for metrics
	var nbw NetBw
	var pb PodBw
	pb.PodRequestBw = make(map[string]PodInfos)
	pb.PodLimitBw = make(map[string]PodInfos)

	for nid, nicInfo := range NicInfos {
		totalSizeSnd := nicInfo.SndGroupBPS
		totalSizeRcv := nicInfo.RcvGroupBPS
		c := &agent.GetAgent().IOInfo
		nic := nicInfo
		bePoolRatio := float64(c.Spec.ResourceConfig[string(v1.NetworkIO)].BEPool) / 100
		bePoolSizeSnd := bePoolRatio * nic.SndGroupBPS
		bePoolSizeRcv := bePoolRatio * nic.RcvGroupBPS
		pb.PodRequestBw[nicInfo.Name] = make(map[string]PodInfo)
		pb.PodLimitBw[nicInfo.Name] = make(map[string]PodInfo)

		for idx, wlg := range e.WgDevices[nid] {
			// Update NetBw
			netReq := common.CalculateGroupBw(wlg.RequestPods, nid)
			nbw.NetRequestBw = append(nbw.NetRequestBw, &netReq)

			var mAlcNtf utils.Workload
			if int32(idx) == utils.GaIndex {
				mAlcNtf = netReq
				// To 2. GA allocated: all GA MRQ
				//3. BT allocated: Total – All GA MRQ
				//4. BE allocated: Min((Total – All GA MRQ  – All BT MRQ), BE Pool)
			} else if int32(idx) == utils.BtIndex {
				mAlcNtf = utils.Workload{
					OutBps: totalSizeSnd - nbw.NetRequestBw[utils.GaIndex].OutBps,
					InBps:  totalSizeRcv - nbw.NetRequestBw[utils.GaIndex].InBps,
				}
			} else if int32(idx) == utils.BeIndex {
				mAlcNtf = utils.Workload{
					OutBps: math.Min(totalSizeSnd-nbw.NetRequestBw[utils.GaIndex].OutBps-nbw.NetRequestBw[utils.BtIndex].OutBps, bePoolSizeSnd),
					InBps:  math.Min(totalSizeRcv-nbw.NetRequestBw[utils.GaIndex].InBps-nbw.NetRequestBw[utils.BtIndex].InBps, bePoolSizeRcv),
				}
			}
			nbw.NetAllocBw = append(nbw.NetAllocBw, &mAlcNtf)

			mTotalNtf := utils.Workload{
				OutBps: totalSizeSnd - bePoolSizeSnd,
				InBps:  totalSizeRcv - bePoolSizeRcv,
			}
			if int32(idx) == utils.BeIndex {
				mTotalNtf.OutBps = bePoolSizeSnd
				mTotalNtf.InBps = bePoolSizeRcv
			}

			nbw.NetTotalBw = append(nbw.NetTotalBw, &mTotalNtf)

			// Update PowBW
			for podId, po := range wlg.RequestPods {
				var sif PodInfo
				sif.Workload = po
				sif.Type = utils.IndexToType[int32(idx)]
				sif.PodName = wlg.PodName[podId]
				pb.PodRequestBw[nicInfo.Name][podId] = sif
			}

			for podId, po := range wlg.LimitPods {
				var sif PodInfo
				sif.Workload = po
				sif.Type = utils.IndexToType[int32(idx)]
				sif.PodName = wlg.PodName[podId]
				var ne utils.Workload
				ne.InBps = po.InBps
				ne.OutBps = po.OutBps

				sif.Workload = &ne
				pb.PodLimitBw[nicInfo.Name][podId] = sif
			}
		}

		// Update to metrics
		e.EngineCache.Cache[e.NodeName][nicInfo.Name] = &nbw
	}
	e.EngineCache.PodCache[e.NodeName].PodRequestBw = pb.PodRequestBw
	e.EngineCache.PodCache[e.NodeName].PodLimitBw = pb.PodLimitBw

	klog.V(utils.DBG).Infof("UpdateNetMetrics: NicCache:%+v\nPodCache:%+v\n", nbw, e.EngineCache.PodCache[e.NodeName])
	e.MetricCache.UpdateCache(&e.EngineCache)
}

func init() {
	a := agent.GetAgent()

	engine := &NetEngine{
		agent.IOEngine{
			Flag:          0,
			ExecutionFlag: agent.ProfileFlag | agent.PolicyFlag | agent.AdminFlag,
		},
		utils.NicInfos{},
		make(map[string]struct{}),
		0,
		20,
		NetMetric{
			Cache:    make(map[string]NodeNetBw),
			PodCache: make(map[string]*PodBw),
		},
		NetMetric{
			Cache:    make(map[string]NodeNetBw),
			PodCache: make(map[string]*PodBw),
		},
		v1.ResourceConfigSpec{},
		make(map[string]*PodRegInfo),
	}

	a.RegisterEngine(engine)
}
