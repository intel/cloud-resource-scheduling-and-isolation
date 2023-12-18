/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdtengine

const (
	EventTypeStop = "Stop"
	EventTypeApp  = "App"

	EventTypeMonitorGroup = "MonitorGroup"
	EventTypeControlGroup = "ControlGroup"
	EventTypeRefresh      = "Refresh"
	EventTaskIds          = "TaskIds"
	EventTypeData         = "Data"

	OperationAdd    = "Add"
	OperationRemove = "Remove"
	OperationUpdate = "Update"
)

type EventFunc func(data EventData) (EventResponse, error)
type PostEventFunc func(data EventData, response EventResponse)

type EventData interface {
	Target() []string
	PostEventTarget() []string
	GetEventType() string
	Key() string
}

type EventResponse interface {
	GetType() string // “Error”, “App”, “MonitorGroup”
}

type ErrorResponse struct {
	Err error
}

func (r ErrorResponse) GetType() string {
	return "Err"
}

type AppResponse struct {
	AppId          string
	MonitorGroupId string
	ControlGroupId string
	Status         string // created, removed
}

// Stop Event
type StopEvent struct {
}

func (e *StopEvent) Target() []string {
	return []string{EventTargetStop}
}

func (e *StopEvent) PostEventTarget() []string {
	return []string{EventTargetStop}
}

func (e *StopEvent) GetEventType() string {
	return EventTypeStop
}

func (e *StopEvent) Key() string {
	return ""
}

// Application Event
type AppEvent struct {
	Name string
	// add, remove
	Operation string
	EventKey  string
}

func (e *AppEvent) Target() []string {
	// AppEvent will be handled by Policy and Watcher
	return []string{EventTargetPolicy, EventTargetWatcher}
}

func (e *AppEvent) PostEventTarget() []string {
	return []string{EventTargetPolicy, EventTargetWatcher}
}

func (e *AppEvent) GetEventType() string {
	return EventTypeApp
}

func (e *AppEvent) Key() string {
	return e.EventKey
}

// Monitor Group Event
type MonitorGroupEvent struct {
	Id string
	// add, remove
	Operation string
	EventKey  string
}

func (e *MonitorGroupEvent) Target() []string {
	// MonitorGroupEvent will be handled by Monitor and Executor
	return []string{EventTargetMonitor, EventTargetExecutor}
}

func (e *MonitorGroupEvent) PostEventTarget() []string {
	return []string{EventTargetMonitor, EventTargetExecutor}
}

func (e *MonitorGroupEvent) GetEventType() string {
	return EventTypeMonitorGroup
}

func (e *MonitorGroupEvent) Key() string {
	return e.EventKey
}

// Control Group Event
type ControlGroupEvent struct {
	Id string
	// add, remove, update
	Operation string
	EventKey  string
	Schema    ControlSchema
}

func (e *ControlGroupEvent) Target() []string {
	// ControlGroupEvent will be handled by Executor
	return []string{EventTargetExecutor}
}

func (e *ControlGroupEvent) PostEventTarget() []string {
	return []string{EventTargetExecutor}
}

func (e *ControlGroupEvent) GetEventType() string {
	return EventTypeControlGroup
}

func (e *ControlGroupEvent) Key() string {
	return e.EventKey
}

// TaskIds Event
type TaskIdsEvent struct {
	MonitorGroupPath string // monitor group path
	Tasks            []int32
}

func (e *TaskIdsEvent) Target() []string {
	// RefreshEvent will be handled by Executor
	return []string{EventTargetExecutor}
}

func (e *TaskIdsEvent) PostEventTarget() []string {
	return []string{EventTargetExecutor}
}

func (e *TaskIdsEvent) GetEventType() string {
	return EventTaskIds
}

func (e *TaskIdsEvent) Key() string {
	return ""
}

// Refresh Event
type RefreshEvent struct {
}

func (e *RefreshEvent) Target() []string {
	// RefreshEvent will be handled by Executor
	return []string{EventTargetExecutor}
}

func (e *RefreshEvent) PostEventTarget() []string {
	return []string{EventTargetExecutor}
}

func (e *RefreshEvent) GetEventType() string {
	return EventTypeRefresh
}

func (e *RefreshEvent) Key() string {
	return ""
}

// Data Event
type DataEvent struct {
	Id   string // monitor group id
	Data MonitorData
}

func (e *DataEvent) Target() []string {
	// DataEvent will be handled by Policy and MonitorReceiver
	return []string{EventTargetPolicy, EventTargetMonitorReceiver}
}

func (e *DataEvent) PostEventTarget() []string {
	return []string{EventTargetPolicy, EventTargetMonitorReceiver}
}

func (e *DataEvent) GetEventType() string {
	return EventTypeData
}

func (e *DataEvent) Key() string {
	return e.Data.Key()
}
