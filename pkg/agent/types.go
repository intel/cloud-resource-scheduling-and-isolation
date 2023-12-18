/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package agent

import (
	"time"

	"github.com/containerd/nri/pkg/api"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

const (
	EVENT_NRI             = "NRI"
	EVENT_ADMIN           = "Admin"
	EVENT_POLICY          = "Policy"
	EVENT_DISK            = "Disk"
	EVENT_DISK_CR         = "Disk_CR"
	EVENT_NET             = "Network"
	EVENT_NET_CR          = "Network_CR"
	EVENT_RDT_CR          = "RDT_CR"
	EVENT_RDT_Quantity_CR = "RDT_Quantity_CR"
	EVENT_RDT             = "RDT"
	EVENT_RDT_Quantity    = "RDTQuantity"
	EVENT_PROF_DISK       = "PROF_DISK"
	EVENT_PROF_NET        = "PROF_NET"
	EVENT_HEARTBEAT       = "HEARTBEAT"
	EVENT_NONE            = "None"
	//GAIndex             = 0
	//BTIndex             = 1
	//BEIndex             = 2
	ProfileFlag         = 0b1
	PolicyFlag          = 0b10
	AdminFlag           = 0b100
	ExpectPolicy        = 0
	ExpectPodData       = ProfileFlag | PolicyFlag | AdminFlag
	ExpectAdmin         = 0
	ExpectProfile       = AdminFlag
	ExpectContainerData = AdminFlag
	ExpectClassData     = AdminFlag
)

type RegisterInfo struct {
	Id string
	PodEventInfo
	Op int
}

type PodData struct {
	T          int // 0:nri 1:disk service-client 2:disk cr 3.network client 4:network cr 5:rdt cr 6: rdt quantity cr
	Generation int64
	PodEvents  map[string]PodEventInfo // diskId:podEventInfo
}

type ContainerData struct {
	T              int // 0:nri 1:disk service-client 2:disk cr 3.network client 4:network cr 5. rdt cr 6: rdt quantity cr
	Generation     int64
	ContainerInfos map[string]ContainerInfo // containerid:ContainerInfo
}

type HeartBeatData struct {
	LastUpdateTime time.Time
}

func (p *PodData) Type() string {
	switch p.T {
	case 0:
		return EVENT_NRI
	case 1:
		return EVENT_DISK
	case 2:
		return EVENT_DISK_CR
	case 3:
		return EVENT_NET
	case 4:
		return EVENT_NET_CR
	case 5:
		return EVENT_RDT_CR
	case 6:
		return EVENT_RDT_Quantity_CR
	default:
		return EVENT_NONE
	}
}

func (p *PodData) Expect() int {
	return ExpectPodData
}

func (p *ContainerData) Type() string {
	switch p.T {
	case 0:
		return EVENT_NRI
	case 1:
		return EVENT_DISK
	case 2:
		return EVENT_DISK_CR
	case 3:
		return EVENT_NET
	case 4:
		return EVENT_NET_CR
	case 5:
		return EVENT_RDT_CR
	case 6:
		return EVENT_RDT_Quantity_CR
	default:
		return EVENT_NONE
	}
}

func (p *ContainerData) Expect() int {
	return ExpectContainerData
}

func (p *HeartBeatData) Type() string {
	return EVENT_HEARTBEAT
}

func (p *HeartBeatData) Expect() int {
	return ExpectPodData
}

type ClassData struct {
	T          int                  // 0:rdt service-client 1:rdt quantity service-client
	ClassInfos map[string]ClassInfo // class name: class info
}

type ClassInfo struct {
	Score float64
}

func (p *ClassData) Type() string {
	switch p.T {
	case 0:
		return EVENT_RDT
	case 1:
		return EVENT_RDT_Quantity
	default:
		return EVENT_NONE
	}
}

func (p *ClassData) Expect() int {
	return ExpectClassData
}

type WorkloadGroup struct {
	LimitPods     map[string]*utils.Workload // podid:workload
	ActualPods    map[string]*utils.Workload
	RequestPods   map[string]*utils.Workload
	PodsLimitSet  map[string]*utils.Workload
	RawRequest    map[string]*utils.Workload
	RawLimit      map[string]*utils.Workload
	PodName       map[string]string
	GroupLimit    *utils.Workload
	IoType        int // 	DiskWorkloadType = 0, NetWorkloadType  = 1
	GroupSettable bool
	WorkloadType  int32
}

type PodEventInfo map[string]PodRunInfo //podId

type PodRunInfo struct {
	Actual         *utils.Workload
	Request        *utils.Workload
	Limit          *utils.Workload
	RawRequest     *utils.Workload
	RawLimit       *utils.Workload
	PodName        string
	WorkloadType   int32 // 0: GA 1:BT 2:BE
	Operation      int   // 0:delete 1:add
	Namespace      string
	NetNamespace   []*api.LinuxNamespace
	DiskAnnotation string
	NetAnnotation  string
	CGroupPath     string
	RawActualBs    *map[string]*utils.Workload
}

type ContainerInfo struct {
	Operation             int // 0:delete 1:add
	PodUid                string
	PodName               string
	ContainerName         string
	ContainerId           string
	Namespace             string
	RDTAnnotation         string
	RDTQuantityAnnotation string
	ContainerCGroupPath   string
}

type GroupInfo struct {
	DevName string
	PodName string
}

type GroupKey struct {
	PodUid string // pod id
	DevUid string
}

type IOILogLevel struct {
	NodeAgentInfo   bool
	NodeAgentDebug  bool
	SchedulerInfo   bool
	SchedulerDebug  bool
	AggregatorInfo  bool
	AggregatorDebug bool
	ServiceInfo     bool
	ServiceDebug    bool
}

type ReportInterval struct {
	NetInterval  int
	DiskInterval int
	RdtInterval  int
}

type AdminResourceConfig struct {
	InitFlag            bool
	DiskBePool          int
	DiskSysPool         int
	NetBePool           int
	NetSysPool          int
	NamespaceWhitelist  map[string]struct{}
	SysDiskRatio        int
	ProfileTime         int
	Disk_min_pod_in_bw  int
	Disk_min_pod_out_bw int
	Net_min_pod_in_bw   int
	Net_min_pod_out_bw  int
	Loglevel            IOILogLevel
	Interval            ReportInterval
}

func (p *AdminResourceConfig) Type() string {
	return EVENT_ADMIN
}

func (p *AdminResourceConfig) Expect() int {
	return ExpectAdmin
}

type SYSInfo struct {
	AllProfInfo AllProfile
}

type Policy struct {
	Throttle          string
	Select_method     string
	Compress_method   string
	Decompress_method string
	Report_Flag       string
}

type PolicyConfig map[string]Policy //ga/bt/be

func (p *PolicyConfig) Type() string {
	return EVENT_POLICY
}

func (p *PolicyConfig) Expect() int {
	return ExpectPolicy
}

type AllDiskInfo map[string]utils.BlockDevice

type AllProfile struct {
	DProfile utils.DiskInfos
	NProfile utils.NicInfos
	DInfo    AllDiskInfo
	T        int
}

func (p *AllProfile) Type() string {
	switch p.T {
	case 0:
		return EVENT_PROF_DISK
	case 1:
		return EVENT_PROF_NET
	default:
		return EVENT_NONE
	}
}

func (p *AllProfile) Expect() int {
	return ExpectProfile
}
