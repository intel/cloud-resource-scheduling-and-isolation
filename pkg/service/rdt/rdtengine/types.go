/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdtengine

const (
	EventTargetEngine          = "Engine"
	EventTargetPolicy          = "Policy"
	EventTargetMonitor         = "Monitor"
	EventTargetWatcher         = "Watcher"
	EventTargetMonitorReceiver = "MonitorReceiver"
	EventTargetExecutor        = "Executor"
	EventTargetStop            = "Stop"
)

type Engine interface {
	GetType() string

	// Register an application to be managed by the engine
	RegisterApp(name string, app ApplicationSpec) error
	// Unregister an application
	UnRegisterApp(name string) error
	// Get the application
	GetApp(name string) (Application, error)
	// Get the control schema of an application
	GetAppControlSchema(name string, version string) (string, error)

	// Register an receiver to receive data monitored by engine
	AddMonitorReceiver(receiver MonitorReceiver)

	// Start the engine
	Start() error
	// Stop the engine
	Stop() error
}

type EventBase interface {
	OnEvent(data EventData) (EventResponse, error)
	EventFilter() []string
}

type Policy interface {
	EventBase
	GetType() string
	Initialize() error
	Start() error
	Stop() error
}

type Monitor interface {
	EventBase
	GetType() string
	Initialize() error
	Start() error
	Stop() error
}

type MonitorData interface {
	ToString() string
	Key() string
}

type Watcher interface {
	EventBase
	GetType() string
	Initialize() error
	Start() error
	Stop() error
}

type Executor interface {
	EventBase
	GetType() string
	Initialize() error
	Start() error
	Stop() error
}

type MonitorReceiver interface {
	EventBase
	GetType() string
}

type ResourceHandler interface {
	// App operations
	GetAllApps() map[string]Application
	GetApp(id string) (Application, error)

	// Event operations
	EmitEvent(data EventData)
	RegisterEventHandler(t string, f EventFunc, msg string)

	// Monitor Group operations
	CreateMonitorGroup(id string, grp MonitorGroup) error
	RemoveMonitorGroup(id string) error
	GetMonitorGroup(id string) (MonitorGroup, error)

	// Control Group operations
	CreateControlGroup(id string, grp ControlGroup) error
	RemoveControlGroup(id string) error
	SetControlGroupSchema(id string, schema ControlSchema) error

	// Data
	ReportData(id string, data MonitorData)

	// Refresh
	Refresh()
	RefreshMonitorGroups(grps []MonitorGroup)
	RefreshControlGroups(grps []ControlGroup)
}

// Base object for policy/monitor/watcher/executor
type BaseObject struct {
	RH ResourceHandler
}

func (o *BaseObject) emitEvent(data EventData) {
	o.RH.EmitEvent(data)
}

func (o *BaseObject) EventFilter() []string {
	return []string{}
}
