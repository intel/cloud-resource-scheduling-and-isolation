/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
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

var (
	MessageHeader = "Disk IO"
	TypeDiskIO    = "DiskIO"
)

const (
	conSuffix    string = ".scope"
	podPrefix    string = "/sys/fs/cgroup"
	emptyDiskAnn string = "'{\"disks\":[]}'"
	CrNamespace  string = "ioi-system"
	conPrefix    string = "/cri-containerd-"
)

var DiskObservedGeneration int64
var DiskInfos = make(map[string]utils.DiskInfo) // key:id
var podMutex sync.Mutex
var IsProfiled bool = false
var ProfileMutex sync.Mutex

type PodRegInfo struct {
	CgroupPath   string
	NriOperation int
	CrOperation  int
	devIds       []string // devId array
}

// DiskEngine
type DiskEngine struct {
	agent.IOEngine
	Profile        utils.DiskInfos
	NSWhitelist    map[string]struct{}
	MetricCache    DiskMetric
	EngineCache    DiskMetric
	resourceConfig v1.ResourceConfigSpec
	sysDiskRatio   int
	profileTimes   int
	sysPool        int
	podRegInfos    map[string]*PodRegInfo // key:podUid
}

func GetProfiled() bool {
	ProfileMutex.Lock()
	profile := IsProfiled
	ProfileMutex.Unlock()
	return profile
}

func SetProfiled(profile bool) {
	ProfileMutex.Lock()
	IsProfiled = profile
	ProfileMutex.Unlock()
}

func (e *DiskEngine) Type() string {
	return TypeDiskIO
}

func (e *DiskEngine) Initialize(coreClient *kubernetes.Clientset, client *versioned.Clientset, mtls bool) error {
	klog.Info("3.2 initializing the disk engine")

	a := agent.GetAgent()
	if (a.EnginesSwitch & agent.DiskSwitch) == 0 {
		klog.Warning("disk engine is not enabled")
		return errors.New("disk label is not set")
	}

	a.RegisterEventHandler(agent.AdminChan, agent.EVENT_ADMIN, e.ProcessAdmin, MessageHeader, e, false)
	a.RegisterEventHandler(agent.ProfileChan, agent.EVENT_PROF_DISK, e.ProcessProfile, MessageHeader, e, true)
	a.RegisterEventHandler(agent.PolicyChan, agent.EVENT_POLICY, e.ProcessPolicy, MessageHeader, e, false)
	a.RegisterEventHandler(agent.PodInfoChan, agent.EVENT_NRI, e.ProcessNriData, MessageHeader, e, true)
	a.RegisterEventHandler(agent.PodInfoChan, agent.EVENT_DISK, e.ProcessDiskIOData, MessageHeader, e, true)
	a.RegisterEventHandler(agent.PodInfoChan, agent.EVENT_DISK_CR, e.ProcessDiskCrData, MessageHeader, e, true)

	var err error
	e.CoreClient = coreClient
	e.Clientset = client
	e.NodeName = agent.GetAgent().IOInfo.Spec.NodeName
	e.GpPolicy = make([]*agent.Policy, utils.GroupIndex)

	e.InitializeWorkloadGroup(common.DiskWorkloadType)

	go common.DiskConsumer(agent.DisksProfiled)
	go e.StartAnalyzeDiskProfiler()
	go common.IOISubscribeDisk(DiskInfos)

	// Watch device status
	err = WatchDeviceStatus(e.Clientset)
	if err != nil {
		klog.Warning("WatchDeviceStatus failed.")
	}

	e.MetricCache.InitMetric()
	metrics.GetMetricEngine().AddMetric("Disk", &e.MetricCache)
	e.EngineCache.PodCache[e.NodeName] = &PodBw{}
	e.EngineCache.Cache[e.NodeName] = make(map[string]*DiskBw)

	return err
}

func (e *DiskEngine) Uninitialize() error {
	return nil
}

func (e *DiskEngine) StartAnalyzeDiskProfiler() {
	for {
		if !(e.IsAdmin()) {
			time.Sleep(1 * time.Second)
			continue
		}
		DeviceStatus, err := common.GetDiskStatus(common.Filename)
		if err != nil {
			klog.Error("can not get disk status ")
			continue
		}

		// filter with valid disks in deviceStatus
		// valid disk are ready or not
		GetValidDiskStatus(&DeviceStatus)

		diskReady, err := AnalysisDiskStatus(DeviceStatus)
		if err != nil {
			klog.Error("3. wait for the disk profile result to create")
		} else {
			if diskReady {
				common.AnalysisDiskProfile(DeviceStatus, common.Filename)
			} else {
				klog.V(utils.DBG).Info("4.2 Profiling the disk")
			}
		}
		time.Sleep(1 * time.Minute)
	}
}

func StartDiskProfiler(disksInput map[string][]string) {
	// get disk info from svc client - server
	klog.V(utils.INF).Infof("0.StartDiskProfiler disksInput %+v", disksInput)
	NodeName := os.Getenv("Node_Name")
	disks := []common.DiskInput{}
	for _, dk := range disksInput[NodeName] {
		disks = append(disks, common.DiskInput{DiskName: dk})
	}
	data, json_err := json.Marshal(disks)
	if json_err != nil {
		klog.Warningf("Marshal is failed")
	}
	klog.V(utils.INF).Infof("StartDiskProfiler 1.Prepare to get service diskInput: %+v", string(data))
	allProfile := common.IOIGetServiceInfoDisk(string(data))
	if allProfile == nil {
		klog.Error("take disks info from service failed")
		return
	}
	klog.V(utils.INF).Infof("StartDiskProfiler 2.get service returned disks: %+v", allProfile.DInfo)
	newDisks := FilterDisk(allProfile.DInfo, disks)
	klog.V(utils.INF).Infof("StartDiskProfiler 3.filter disk returned disks: %+v", newDisks)
	// compare with file and determine which disks be profiled (wrt to chan)
	if len(newDisks) > 0 {
		common.GetNewDisks(&newDisks, common.Filename)
		klog.V(utils.INF).Infof("StartDiskProfiler 4.compared new disks in profiler file: %+v", newDisks)
		if len(newDisks) == 0 {
			SetProfiled(true)
			return
		}
		// parallel profile (wrt to another chan for process to consume)
		common.DiskProducer(newDisks, agent.DisksProfiled)
	}
}

func AnalysisDiskStatus(DeviceStatus map[string]int) (bool, error) {
	SetProfilingDisk(DeviceStatus)

	for disk := range DeviceStatus {
		if DeviceStatus[disk] != 2 {
			return false, nil
		}
	}
	return true, nil
}

func GetMountPath(deviceID string) (string, error) {
	var mountPath string
	if diskInfo, ok := DiskInfos[deviceID]; ok {
		mountPath = diskInfo.MountPoint
	} else {
		err := errors.New("GetMountPath error with no such diskInfo")
		klog.Error(err)
		return "", err
	}
	return mountPath, nil
}

func (e *DiskEngine) updateLocalNodeDiskStatusIOInfo() error {
	var diskIO aggr.IOBandwidth
	diskIO.DeviceBwInfo = make(map[string]*aggr.DeviceBandwidthInfo)
	diskIO.IoType = aggr.IOType_DiskIO
	gen := DiskObservedGeneration
	for diskId, diskInfo := range DiskInfos {
		read, write, total, pressure := e.calDiskAllocatable(diskId)
		e.UpdateDiskMetrics()
		ioStatus := make(map[string]*aggr.IOStatus)

		var gaStatus aggr.IOStatus
		var beStatus aggr.IOStatus

		gaStatus.In = read[utils.GaIndex]
		gaStatus.Out = write[utils.GaIndex]
		gaStatus.Total = total[utils.GaIndex]
		if pressure[utils.GaIndex] {
			gaStatus.Pressure = 1
		} else {
			gaStatus.Pressure = 0
		}

		beStatus.In = read[utils.BeIndex]
		beStatus.Out = write[utils.BeIndex]
		beStatus.Total = total[utils.BeIndex]
		if pressure[utils.BeIndex] {
			beStatus.Pressure = 1
		} else {
			beStatus.Pressure = 0
		}

		ioStatus["GA"] = &gaStatus // BT status is same as GA for now
		ioStatus["BE"] = &beStatus

		deviceInfo := &aggr.DeviceBandwidthInfo{
			Name:   diskInfo.Name,
			Status: ioStatus,
		}
		diskIO.DeviceBwInfo[diskId] = deviceInfo
	}
	klog.V(utils.DBG).Infof("updateLocalNodeDiskStatusIOInfo diskIO: %s", diskIO.String())
	err := agent.UpdateNodeIOStatus(e.NodeName, &diskIO, gen)

	return err
}

func (e *DiskEngine) ProcessProfile(data agent.EventData) error {
	profileData := data.(*agent.AllProfile)
	klog.V(utils.INF).Info("now in disk ProcessProfile: ", *profileData)

	c := &agent.GetAgent().IOInfo
	profile := GetProfiled()
	if !reflect.DeepEqual(e.Profile, profileData.DProfile) || profile {
		if profile {
			SetProfiled(false)
		}
		e.Profile = profileData.DProfile
		resourceConfig := &e.resourceConfig
		resourceConfig.Devices = make(map[string]v1.Device)
		for diskId, diskInfo := range e.Profile {
			var newDiskInfo utils.DiskInfo
			newDiskInfo.Name = diskInfo.Name
			newDiskInfo.MountPoint = diskInfo.MountPoint
			newDiskInfo.MajorMinor = diskInfo.MajorMinor

			var newDevice v1.Device
			klog.Infof("diskInfo.Type:%s, diskInfo.Capacity:%s", diskInfo.Type, diskInfo.Capacity)
			newDevice.Name = diskInfo.Name
			if diskInfo.Type == utils.EmptyDir {
				newDevice.Type = v1.DefaultDevice
				availRatio := (100.0 - float64(e.sysPool)) / 100.0
				newDiskInfo.TotalBPS = diskInfo.TotalBPS * availRatio * float64(e.profileTimes)
				newDiskInfo.TotalRBPS = diskInfo.TotalRBPS * availRatio * float64(e.profileTimes)
				newDiskInfo.TotalWBPS = diskInfo.TotalWBPS * availRatio * float64(e.profileTimes)
				dv := resource.MustParse(diskInfo.Capacity)
				dvs := dv.ToDec().Value() * int64(e.sysDiskRatio) / 100
				newDevice.DiskSize = strconv.FormatInt(dvs, 10)
			} else {
				newDevice.Type = v1.OtherDevice
				newDiskInfo.TotalBPS = diskInfo.TotalBPS * float64(e.profileTimes)
				newDiskInfo.TotalRBPS = diskInfo.TotalRBPS * float64(e.profileTimes)
				newDiskInfo.TotalWBPS = diskInfo.TotalWBPS * float64(e.profileTimes)
				newDevice.DiskSize = diskInfo.Capacity
			}
			newDevice.ReadRatio = make(map[string]float64)
			newDevice.WriteRatio = make(map[string]float64)
			newDiskInfo.ReadRatio = make(map[string]float64)
			newDiskInfo.WriteRatio = make(map[string]float64)
			for k, v := range diskInfo.ReadRatio {
				newDiskInfo.ReadRatio[k] = v
				newDevice.ReadRatio[k] = v
			}
			for k, v := range diskInfo.WriteRatio {
				newDiskInfo.WriteRatio[k] = v
				newDevice.WriteRatio[k] = v
			}
			readMpRatio := utils.ParseMappingRatio(diskInfo.ReadRatio)
			newDiskInfo.ReadMpRatio = readMpRatio
			writeMpRatio := utils.ParseMappingRatio(diskInfo.WriteRatio)
			newDiskInfo.WriteMpRatio = writeMpRatio

			DiskInfos[diskId] = newDiskInfo

			newDevice.DefaultIn = common.MinPodDiskInBw
			newDevice.DefaultOut = common.MinPodDiskOutBw
			newDevice.CapacityIn = newDiskInfo.TotalRBPS
			newDevice.CapacityOut = newDiskInfo.TotalWBPS
			newDevice.CapacityTotal = newDiskInfo.TotalBPS
			newDevice.MountPath = newDiskInfo.MountPoint
			resourceConfig.Devices[diskId] = newDevice

			if _, ok := e.WgDevices[diskId]; !ok {
				for i := utils.GaIndex; i < utils.GroupIndex; i++ {
					wg := agent.WorkloadGroup{
						IoType:        common.DiskWorkloadType,
						GroupSettable: false,
						WorkloadType:  i,
						LimitPods:     make(map[string]*utils.Workload),
						ActualPods:    make(map[string]*utils.Workload),
						RequestPods:   make(map[string]*utils.Workload),
						PodsLimitSet:  make(map[string]*utils.Workload),
						RawRequest:    make(map[string]*utils.Workload),
						RawLimit:      make(map[string]*utils.Workload),
						PodName:       make(map[string]string),
					}
					e.AddWorkloadGroup(wg, diskId)
				}
			}
		}

		// Update cache status
		UpdateProfileDiskState(c)

		e.SetFlag(agent.ProfileFlag)
		if e.IsExecution() {
			c.Spec.ResourceConfig[string(v1.BlockIO)] = e.resourceConfig
			klog.V(utils.INF).Infof("update static cr in ProcessProfile c.Spec.ResourceConfig: %v", c.Spec.ResourceConfig)
			err := common.UpdateNodeStaticIOInfo(c, e.Clientset)
			if err != nil {
				return err
			}
		}
	} else {
		klog.V(utils.DBG).Info("disk profile is the same as last time, no need to change")
	}

	return nil
}

func (e *DiskEngine) ProcessPolicy(data agent.EventData) error {
	policyData := data.(*agent.PolicyConfig)
	klog.V(utils.INF).Info("now in disk ProcessPolicy: ", *policyData)
	gaSet := false
	beSet := false
	btSet := false

	for policyType, policy := range *policyData {
		switch policyType {
		case "DiskGArules":
			e.GpPolicy[utils.GaIndex] = &policy
			gaSet = true
		case "DiskBTrules":
			e.GpPolicy[utils.BtIndex] = &policy
			btSet = true
		case "DiskBErules":
			e.GpPolicy[utils.BeIndex] = &policy
			beSet = true
		default:
			klog.V(utils.DBG).Infof("the policyType %s, policy +%v", policyType, policy)
		}
	}

	if gaSet && beSet && btSet {
		e.SetFlag(agent.PolicyFlag)
		if e.IsExecution() {
			agent.GetAgent().IOInfo.Spec.ResourceConfig[string(v1.BlockIO)] = e.resourceConfig
			klog.V(utils.INF).Infof("update static cr in ProcessPolicy c.Spec.ResourceConfig: %v", e.resourceConfig)
			err := common.UpdateNodeStaticIOInfo(&(agent.GetAgent().IOInfo), e.Clientset)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func GetDiskStaticCR(client *versioned.Clientset) (*v1.NodeStaticIOInfo, error) {
	cr, err := common.GetNodeStaticIOInfos(client)
	if err != nil {
		klog.Warningf("get cr fail:%s", err)
		return nil, errors.New("get cr fail")
	}

	if _, ok := cr.Spec.ResourceConfig[string(v1.BlockIO)]; !ok {
		klog.Warning("the cr of disk part has not been created")
		return nil, errors.New("the cr of disk part has not been created")
	}

	return cr, nil
}

func (e *DiskEngine) ProcessAdmin(data agent.EventData) error {
	adminData := data.(*agent.AdminResourceConfig)
	klog.V(utils.INF).Info("now in disk ProcessAdmin: ", *adminData)
	err := common.IOIConfigureInterval(utils.DiskIOIType, adminData.Interval.DiskInterval)
	if err != nil {
		klog.Error(err)
	}

	c := &agent.GetAgent().IOInfo
	resourceConfig := &e.resourceConfig
	resourceConfig.BEPool = adminData.DiskBePool

	e.NSWhitelist = adminData.NamespaceWhitelist
	e.sysDiskRatio = adminData.SysDiskRatio
	e.sysPool = adminData.DiskSysPool
	common.MinPodDiskInBw = float64(adminData.Disk_min_pod_in_bw)
	common.MinPodDiskOutBw = float64(adminData.Disk_min_pod_out_bw)

	e.profileTimes = adminData.ProfileTime
	// admin config just for init disks.
	firstAdmin := (e.Flag & agent.AdminFlag) == agent.AdminFlag
	if adminData.InitFlag && !firstAdmin {
		disks := make(map[string][]string)
		cr, err := GetDiskStaticCR(e.Clientset)
		if err != nil {
			klog.Errorf("GetDiskStaticCR failed: %v", err)
			go StartDiskProfiler(disks)
		} else {
			// update the cache
			LoadDiskInfo(cr)
			// update the cache cr
			e.resourceConfig = cr.Spec.ResourceConfig[string(v1.BlockIO)]
			//CopyStaticCR(&(cr.Spec.ResourceConfig[string(v1.BlockIO)]), &(e.resourceConfig))
			// deep copy to resourceConfig
			c.Status.DeviceStates = cr.Status.DeviceStates
			for did, devices := range cr.Status.DeviceStates {
				c.Status.DeviceStates[did] = devices
			}
		}
	}
	e.SetFlag(agent.AdminFlag)
	if e.IsExecution() {
		if c.Spec.ResourceConfig == nil {
			klog.Errorf("c.Spec.ResourceConfig is nil")
			return nil
		}
		c.Spec.ResourceConfig[string(v1.BlockIO)] = e.resourceConfig
		klog.V(utils.INF).Infof("update static cr in ProcessAdmin c.Spec.ResourceConfig: %v", c.Spec.ResourceConfig)
		err := common.UpdateNodeStaticIOInfo(c, e.Clientset)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *DiskEngine) RegisterPod() {
	podMutex.Lock()
	for podId, pRegInfo := range e.podRegInfos {
		cgroupPath := pRegInfo.CgroupPath
		if cgroupPath == "" {
			klog.Errorf("Nri pod is not ready %s", podId)
			continue
		}
		devIds := pRegInfo.devIds
		klog.V(utils.INF).Infof("CGroupPath:%s, devIds:%v", pRegInfo.CgroupPath, devIds)
		if len(devIds) == 0 { // not set group
			klog.V(utils.INF).Infof("devIds is empty, podId: %s", podId)
			continue
		}

		Operation := pRegInfo.NriOperation & pRegInfo.CrOperation
		if Operation == 0 {
			klog.V(utils.INF).Infof("ProcessDiskCrData NriOperation:%d, CrOperation:%d", pRegInfo.NriOperation, pRegInfo.CrOperation)
			continue
		}

		klog.V(utils.INF).Infof("ProcessDiskCrData Operation:%d ", Operation)
		if Operation == 0 {
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
			DevMajorMinors := []string{}
			for _, disk := range devIds {
				DevMajorMinors = append(DevMajorMinors, DiskInfos[disk].MajorMinor)
			}
			klog.Infof("pod operation: %d, podId: %s, DevMajorMinors: %s", Operation, podId, DevMajorMinors)
			common.IOIRegisterDisk(podId, cgroupPath, DevMajorMinors)
		}
		if Operation&utils.StopPod == utils.StopPod {
			common.IOIUnRegisterDisk(podId)
			// clear workload group
			for _, wgs := range e.WgDevices {
				for _, wg := range wgs {
					delete(wg.RequestPods, podId)
					delete(wg.LimitPods, podId)
					delete(wg.RawRequest, podId)
					delete(wg.RawLimit, podId)
					delete(wg.PodsLimitSet, podId)
				}
			}

			// clear cache
			delete(e.EngineCache.PodCache[e.NodeName].PodTranBw, podId)
			delete(e.EngineCache.PodCache[e.NodeName].PodRawBw, podId)
			e.UpdateDiskMetrics()
		}
		if Operation&utils.StopContainer == utils.StopContainer {
			klog.V(utils.INF).Info("StopContainer event from nri")
		}

		err := e.updateLocalNodeDiskStatusIOInfo()
		if err != nil {
			klog.Errorf("updateLocalNodeDiskStatusIOInfo failed: %v", err)
		}
		podMutex.Lock()
	}
	podMutex.Unlock()
}

func (e *DiskEngine) ProcessNriData(data agent.EventData) error {
	nriData := data.(*agent.PodData)
	klog.V(utils.INF).Info("now in ProcessNriData")

	// get disk id from pod id
	podEvent := nriData.PodEvents["dev"]

	for podId, podInfo := range podEvent {
		klog.V(utils.INF).Infof("podId: %s, operation:%d", podId, podInfo.Operation)
		_, ok := e.NSWhitelist[podInfo.Namespace]
		if ok {
			klog.V(utils.DBG).Info("This is system pod, do not process nri data")
			continue
		}

		cgroupPath := podPrefix + podInfo.CGroupPath
		podMutex.Lock()
		if pInfo, ok := e.podRegInfos[podId]; !ok || pInfo == nil {
			pRegInfo := &PodRegInfo{
				CgroupPath:   cgroupPath,
				NriOperation: podInfo.Operation,
			}
			e.podRegInfos[podId] = pRegInfo
		} else {
			pRegInfo := e.podRegInfos[podId]
			pRegInfo.CgroupPath = cgroupPath
			pRegInfo.NriOperation = pRegInfo.NriOperation | podInfo.Operation
		}
		podMutex.Unlock()
	}

	e.RegisterPod()

	return nil
}

func (e *DiskEngine) ProcessDiskIOData(data agent.EventData) error {
	diskIOData := data.(*agent.PodData)
	klog.V(utils.INF).Info("now in ProcessDiskIOData get diskIOData: ")

	for diskId, podEvent := range diskIOData.PodEvents {
		klog.V(utils.INF).Infof("Start to process disk data, diskId: %s, pod count: %d", diskId, len(podEvent))
		for podId, podInfo := range podEvent {
			klog.V(utils.DBG).Infof("Start podId: %s in: %f, out: %f", podId, podInfo.Actual.InBps, podInfo.Actual.OutBps)
			//workloadType := podInfo.WorkloadType
			for _, wg := range e.WgDevices[diskId] {
				if _, ok := wg.RequestPods[podId]; !ok {
					klog.V(utils.DBG).Infof("ProcessDiskIOData podId: %s is not in the group", podId)
					continue
				}
				if wg.RequestPods[podId] != nil { // pod is in the group
					klog.V(utils.DBG).Infof("ProcessDiskIOData, podInfo.Actual:%+v", podInfo.Actual)
					klog.V(utils.DBG).Infof("ProcessDiskIOData, podInfo.RawActualBs:%+v", podInfo.RawActualBs)
					wg.ActualPods[podId] = podInfo.Actual
				}
			}
		}
		e.calDiskAllocatable(diskId)
	}

	podRawBw := make(map[string]PodInfos)
	PodTranBw := make(map[string]PodInfos)
	for diskId, podEvent := range diskIOData.PodEvents {
		var newRawPodInfo PodInfos
		var newTranPodInfo PodInfos

		newRawPodInfo = make(map[string]PodInfo)
		newTranPodInfo = make(map[string]PodInfo)

		for podId, podInfo := range podEvent {
			var rif PodInfo
			var tif PodInfo
			for _, wg := range e.WgDevices[diskId] {
				if _, ok := wg.RequestPods[podId]; !ok {
					klog.V(utils.DBG).Infof("ProcessDiskIOData podId: %s is not in the group", podId)
					continue
				}
				if wg.RequestPods[podId] != nil { // pod is in the group
					rif.PodName = wg.PodName[podId]
					tif.PodName = wg.PodName[podId]

					rif.WlBs = podInfo.RawActualBs
					tif.Workload = podInfo.Actual

					rif.Type = utils.IndexToType[wg.WorkloadType]
					tif.Type = utils.IndexToType[wg.WorkloadType]
				}
			}

			newRawPodInfo[rif.PodName] = rif
			newTranPodInfo[tif.PodName] = tif
		}

		if diskInfo, ok := DiskInfos[diskId]; ok {
			podRawBw[diskInfo.Name] = newRawPodInfo
			PodTranBw[diskInfo.Name] = newTranPodInfo
		}
	}

	e.EngineCache.PodCache[e.NodeName].PodRawBw = podRawBw
	e.EngineCache.PodCache[e.NodeName].PodTranBw = PodTranBw
	klog.V(utils.DBG).Info("Send node agent disk data to metrics,PodRawBw:  ", e.EngineCache.PodCache[e.NodeName].PodRawBw)
	klog.V(utils.DBG).Info("Send node agent disk data to metrics,PodTranBw:  ", e.EngineCache.PodCache[e.NodeName].PodTranBw)

	e.UpdateDiskMetrics()

	return nil
}

func (e *DiskEngine) ProcessDiskCrData(data agent.EventData) error {
	if data == nil {
		return errors.New("event data is nil")
	}
	diskIOData := data.(*agent.PodData)

	DiskObservedGeneration = diskIOData.Generation
	klog.V(utils.INF).Infof("now in ProcessDiskCrData generation:%d ", DiskObservedGeneration)
	visited := make(map[string]struct{})
	for diskId, podEvent := range diskIOData.PodEvents {
		klog.V(utils.INF).Infof("diskId: %s", diskId)
		if _, ok := e.WgDevices[diskId]; !ok {
			klog.Warningf("diskId: %s is not profiled, so the pod can not be registed", diskId)
			continue
		}
		for podId, podInfo := range podEvent {
			klog.V(utils.INF).Infof("podId: %s, Request: %v, Limit: %v", podId, podInfo.Request, podInfo.Limit)
			klog.V(utils.INF).Infof("podId: %s, RawRequest: %v, RawLimit: %v", podId, podInfo.RawRequest, podInfo.RawLimit)

			if podInfo.PodName == "" {
				klog.V(utils.INF).Infof("podName is empty, podId: %s", podId)
				continue
			}
			workloadType := podInfo.WorkloadType
			e.WgDevices[diskId][workloadType].RequestPods[podId] = podInfo.Request
			e.WgDevices[diskId][workloadType].LimitPods[podId] = podInfo.Limit
			e.WgDevices[diskId][workloadType].RawRequest[podId] = podInfo.RawRequest
			e.WgDevices[diskId][workloadType].RawLimit[podId] = podInfo.RawLimit
			e.WgDevices[diskId][workloadType].PodName[podId] = podInfo.PodName
			e.WgDevices[diskId][workloadType].WorkloadType = workloadType

			visited[podId] = struct{}{}

			podMutex.Lock()
			if pInfo, ok := e.podRegInfos[podId]; !ok || pInfo == nil {
				pRegInfo := &PodRegInfo{
					CrOperation: 0,
					devIds:      []string{diskId},
				}
				e.podRegInfos[podId] = pRegInfo
			} else {
				pRegInfo := e.podRegInfos[podId]
				pRegInfo.devIds = append(pRegInfo.devIds, diskId)
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
	// Register pod
	e.RegisterPod()

	return nil
}

func (e *DiskEngine) calDiskAllocatable(diskId string) ([]float64, []float64, []float64, []bool) {
	klog.V(utils.DBG).Info("now start to calDiskAllocatable")

	if _, ok := DiskInfos[diskId]; !ok {
		klog.Warningf("diskId: %s is not profiled, so the pod can not be registed", diskId)
		return nil, nil, nil, nil
	}

	if _, ok := e.WgDevices[diskId]; !ok {
		klog.Warningf("diskId: %s is not profiled, so the pod can not be registed", diskId)
		return nil, nil, nil, nil
	}

	diskInfo := DiskInfos[diskId]
	SizeRead := diskInfo.TotalRBPS
	SizeWrite := diskInfo.TotalWBPS
	SizeTotal := diskInfo.TotalBPS

	bePoolRatio := float64(e.resourceConfig.BEPool) / 100
	klog.V(utils.INF).Infof("SizeRead: %f,bePoolRatio: %f", SizeRead, bePoolRatio)
	bePoolSizeRead := bePoolRatio * SizeRead
	bePoolSizeWrite := bePoolRatio * SizeWrite
	bePoolSizeTotal := bePoolRatio * SizeTotal

	allocRead := make([]float64, len(e.WgDevices[diskId]))
	allocWrite := make([]float64, len(e.WgDevices[diskId]))
	allocTotal := make([]float64, len(e.WgDevices[diskId]))
	pressure := make([]bool, len(e.WgDevices[diskId]))
	expect := make([]utils.Workload, len(e.WgDevices[diskId]))

	gaReqWl := common.CalculateGroupBw(e.WgDevices[diskId][utils.GaIndex].RequestPods, diskId)
	btReqWl := common.CalculateGroupBw(e.WgDevices[diskId][utils.BtIndex].RequestPods, diskId)

	allocRequest := utils.Workload{
		InBps:    SizeRead - bePoolSizeRead - gaReqWl.InBps - btReqWl.InBps,
		OutBps:   SizeWrite - bePoolSizeWrite - gaReqWl.OutBps - btReqWl.OutBps,
		TotalBps: SizeTotal - bePoolSizeTotal - gaReqWl.InBps - btReqWl.InBps - gaReqWl.OutBps - btReqWl.OutBps,
	}

	allocWrite[utils.GaIndex] = allocRequest.OutBps
	allocRead[utils.GaIndex] = allocRequest.InBps
	allocTotal[utils.GaIndex] = allocRequest.TotalBps

	allocWrite[utils.BtIndex] = allocRequest.OutBps
	allocRead[utils.BtIndex] = allocRequest.InBps
	allocTotal[utils.BtIndex] = allocRequest.TotalBps

	allocWrite[utils.BeIndex] = bePoolSizeRead
	allocRead[utils.BeIndex] = bePoolSizeWrite
	allocTotal[utils.BeIndex] = bePoolSizeTotal

	if gaReqWl.InBps == 0 {
		expect[utils.GaIndex].InBps = allocRead[utils.GaIndex]
	} else {
		expect[utils.GaIndex].InBps = gaReqWl.InBps
	}

	if gaReqWl.OutBps == 0 {
		expect[utils.GaIndex].OutBps = allocWrite[utils.GaIndex]
	} else {
		expect[utils.GaIndex].OutBps = gaReqWl.OutBps
	}

	if btReqWl.OutBps > 0 {
		expect[utils.BtIndex].OutBps = allocWrite[utils.BtIndex]/btReqWl.OutBps + 1
	} else {
		expect[utils.BtIndex].OutBps = 1
	}

	if btReqWl.InBps > 0 {
		expect[utils.BtIndex].InBps = allocRead[utils.BtIndex]/btReqWl.InBps + 1
	} else {
		expect[utils.BtIndex].InBps = 1
	}

	btActual := common.CalculateGroupBw(e.WgDevices[diskId][utils.BtIndex].ActualPods, diskId)
	var btRuntimeOut float64
	var btRuntimeIn float64
	if btReqWl.OutBps > btActual.OutBps {
		btRuntimeOut = btReqWl.OutBps
	} else {
		btRuntimeOut = btActual.OutBps
	}
	if btReqWl.InBps > btActual.InBps {
		btRuntimeIn = btReqWl.InBps
	} else {
		btRuntimeIn = btActual.InBps
	}
	expect[utils.BeIndex].OutBps = SizeWrite - gaReqWl.OutBps - btRuntimeOut
	expect[utils.BeIndex].InBps = SizeRead - gaReqWl.InBps - btRuntimeIn

	for idx := range e.WgDevices[diskId] {
		wlg := &e.WgDevices[diskId][idx]
		klog.V(utils.DBG).Info("Start Cal group, Wlgroup index: ", idx)
		request := common.CalculateGroupBw(wlg.RequestPods, diskId)
		dev := common.DevInfo{
			Id:            diskId,
			Name:          diskInfo.Name,
			DevType:       "DISK",
			DevMajorMinor: diskInfo.MajorMinor,
			DInfo:         &diskInfo,
			NInfo:         nil,
		}
		policy := e.GpPolicy[wlg.WorkloadType]

		common.SetLimit(wlg, policy, expect[idx], request, dev, int32(idx))
	}

	return allocRead, allocWrite, allocTotal, pressure
}

func (e *DiskEngine) UpdateDiskMetrics() {
	// Prepare data for metrics
	var pb PodBw
	pb.PodRequestBw = make(map[string]PodInfos)
	pb.PodLimitBw = make(map[string]PodInfos)

	for diskId, diskInfo := range DiskInfos {
		var nbw DiskBw
		SizeRead := diskInfo.TotalRBPS
		SizeWrite := diskInfo.TotalWBPS
		SizeTotal := diskInfo.TotalBPS

		c := &agent.GetAgent().IOInfo
		bePoolRatio := float64(c.Spec.ResourceConfig[string(v1.BlockIO)].BEPool) / 100
		bePoolSizeRead := bePoolRatio * SizeRead
		bePoolSizeWrite := bePoolRatio * SizeWrite
		bePoolSizeTotal := bePoolRatio * SizeTotal

		for idx, wlg := range e.WgDevices[diskId] {
			diskReq := common.CalculateGroupBw(wlg.RequestPods, diskId)
			nbw.DiskRequestBw = append(nbw.DiskRequestBw, &diskReq)

			var mAlcNtf utils.Workload
			if int32(idx) == utils.GaIndex {
				mAlcNtf = diskReq
			} else if int32(idx) == utils.BtIndex {
				mAlcNtf = utils.Workload{
					InBps:    SizeRead - nbw.DiskRequestBw[utils.GaIndex].InBps,
					OutBps:   SizeWrite - nbw.DiskRequestBw[utils.GaIndex].OutBps,
					TotalBps: SizeTotal - nbw.DiskRequestBw[utils.GaIndex].TotalBps,
				}
			} else if int32(idx) == utils.BeIndex {
				mAlcNtf = utils.Workload{
					InBps:    math.Min(SizeRead-nbw.DiskRequestBw[utils.GaIndex].InBps-nbw.DiskRequestBw[utils.BtIndex].InBps, bePoolSizeRead),
					OutBps:   math.Min(SizeWrite-nbw.DiskRequestBw[utils.GaIndex].OutBps-nbw.DiskRequestBw[utils.BtIndex].OutBps, bePoolSizeWrite),
					TotalBps: math.Min(SizeTotal-nbw.DiskRequestBw[utils.GaIndex].TotalBps-nbw.DiskRequestBw[utils.BtIndex].TotalBps, bePoolSizeTotal),
				}
			}
			nbw.DiskAllocBw = append(nbw.DiskAllocBw, &mAlcNtf)

			mTotalNtf := utils.Workload{
				InBps:    SizeRead - bePoolSizeRead,
				OutBps:   SizeWrite - bePoolSizeWrite,
				TotalBps: SizeTotal - bePoolSizeTotal,
			}
			if int32(idx) == utils.BeIndex {
				mTotalNtf.InBps = bePoolSizeRead
				mTotalNtf.OutBps = bePoolSizeWrite
				mTotalNtf.TotalBps = bePoolSizeTotal
			}
			nbw.DiskTotalBw = append(nbw.DiskTotalBw, &mTotalNtf)
		}
		e.EngineCache.Cache[e.NodeName][diskInfo.Name] = &nbw

		pb.PodRequestBw[diskInfo.Name] = make(map[string]PodInfo)
		pb.PodLimitBw[diskInfo.Name] = make(map[string]PodInfo)
		// Update PowBW
		for idx, wlg := range e.WgDevices[diskId] {
			for podId, po := range wlg.RequestPods {
				var rif PodInfo
				rif.Workload = po
				rif.Type = utils.IndexToType[int32(idx)]
				rif.PodName = wlg.PodName[podId]
				pb.PodRequestBw[diskInfo.Name][podId] = rif
			}

			for podId, po := range wlg.LimitPods {
				var lif PodInfo
				lif.Workload = po
				lif.Type = utils.IndexToType[int32(idx)]
				lif.PodName = wlg.PodName[podId]
				pb.PodLimitBw[diskInfo.Name][podId] = lif
			}
		}
	}
	// Update to metrics
	if e.EngineCache.PodCache == nil {
		return
	}

	key := e.NodeName
	if _, ok := e.EngineCache.PodCache[key]; ok {
		e.EngineCache.PodCache[e.NodeName].PodRequestBw = pb.PodRequestBw
		e.EngineCache.PodCache[e.NodeName].PodLimitBw = pb.PodLimitBw
	} else {
		klog.Errorf(key, "does not exist in the map")
		return
	}

	for _, diskInfo := range DiskInfos {
		klog.V(utils.DBG).Infof("UpdateDiskMetrics: DiskCache[%s]=%v", diskInfo.Name, e.EngineCache.Cache[e.NodeName][diskInfo.Name])
	}

	klog.V(utils.DBG).Infof("UpdateDiskMetrics: PodCache: %+v", e.EngineCache.PodCache[e.NodeName])

	e.MetricCache.UpdateCache(&e.EngineCache)
}

func init() {
	a := agent.GetAgent()

	engine := &DiskEngine{
		agent.IOEngine{
			Flag:          0,
			ExecutionFlag: agent.ProfileFlag | agent.PolicyFlag | agent.AdminFlag,
		},
		utils.DiskInfos{},
		make(map[string]struct{}),
		DiskMetric{
			Cache:    make(map[string]NodeDiskBw),
			PodCache: make(map[string]*PodBw),
		},
		DiskMetric{
			Cache:    make(map[string]NodeDiskBw),
			PodCache: make(map[string]*PodBw),
		},
		v1.ResourceConfigSpec{},
		0,
		1,
		20,
		make(map[string]*PodRegInfo),
	}

	a.RegisterEngine(engine)
}
