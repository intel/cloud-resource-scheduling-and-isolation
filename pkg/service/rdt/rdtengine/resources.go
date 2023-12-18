/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdtengine

// Application Spec
type ApplicationSpec struct {
	Name string
	// applition's process id
	PId string
	// cgroup path for a cloud native application, e.g. pod, container
	CgroupPath string
	// Customized defined parameter for policy
	Parameters string
}

// Application Status
type ApplicationStatus struct {
	// Monitor Group Id
	MonitorGroupId string
	// Control Group Id
	ControlGroupId string
	State          int // 1: running, 0: pending, -1: removed, -2: error

	// error information for last operation
	ErrorMessage string
}

// Application
type Application struct {
	Spec   ApplicationSpec   `json:"spec"`
	Status ApplicationStatus `json:"status"`
}

// Monitor Group
type MonitorGroup struct {
	Id             string
	ControlGroupId string
	state          int // 1: running, 0: pending, -1: removed
	// error information for last operation
	ErrorMessage string
	AppIds       []string
}

func (g *MonitorGroup) SetState(state int) {
	g.state = state
}

func (g *MonitorGroup) GetState() int {
	return g.state
}

// Control Group
type ControlSchema struct {
	version string
}

func (s *ControlSchema) Validate() bool {
	return true
}

func (s *ControlSchema) ToString(version string) string {
	return ""
}

type ControlGroup struct {
	Id     string
	Schema ControlSchema
	state  int // 1: running, 0: pending, -1: removed
	// error information for last operation
	ErrorMessage string
	AppIds       []string
}

func (g *ControlGroup) SetState(state int) {
	g.state = state
}

func (g *ControlGroup) GetState() int {
	return g.state
}

func (g *ControlGroup) Validate() bool {
	return g.Schema.Validate()
}

type Bitmask uint64

type RdtL3Info struct {
	CbmMask    Bitmask
	MinCbmBits uint64
	NumClosId  uint64
}

type RdtMbInfo struct {
	MinBW     uint64
	NumClosId uint64
}

type RdtInfo struct {
	L3Info        RdtL3Info
	MbInfo        RdtMbInfo
	Schemata      string
	CacheIds      []uint64
	ControlGroups map[string]*ControlGroup
	MonitorGroups map[string]*MonitorGroup
}

type PodMeta struct {
	Namespace string
	PodName   string
}
