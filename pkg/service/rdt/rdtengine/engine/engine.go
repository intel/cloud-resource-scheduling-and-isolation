/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package engine

import (
	"fmt"
	"reflect"
	"sync"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"
)

func init() {
	rdt.RegisterEngine(rdt.DefaultEngineType, DefaultEngineNewer)
}

var eventChanSize = 50

type EventHandle struct {
	f   rdt.EventFunc
	msg string
}

type PostEventHandle struct {
	f   rdt.PostEventFunc
	msg string
}

type EventHandler struct {
	t       string
	ch      chan rdt.EventData
	handles []EventHandle
}

type PostEventHandler struct {
	t       string
	handles []PostEventHandle
}

type EngineConfig struct {
	Policy    rdt.RawConfig
	Watchers  []rdt.RawConfig
	Monitors  []rdt.RawConfig
	Executors []rdt.RawConfig
}

type engine struct {
	sync.Mutex
	Type string
	//running status
	isRunning bool

	policy            rdt.Policy
	monitors          []rdt.Monitor
	watchers          []rdt.Watcher
	executors         []rdt.Executor
	monitor_receivers []rdt.MonitorReceiver

	// event handlers
	ehs []EventHandler

	// post event handlers
	pehs map[string]PostEventHandler

	// resources
	mu   sync.RWMutex
	apps map[string]rdt.Application  // key: application name
	mgs  map[string]rdt.MonitorGroup // key: monitor group id
	cgs  map[string]rdt.ControlGroup // key: control group id

	// rdtinfo in resctrl filesystem
	rdtInfo rdt.RdtInfo
}

func (eng *engine) GetMonitorGroup(id string) (rdt.MonitorGroup, error) {
	eng.mu.RLock()
	defer eng.mu.RUnlock()

	if v, ok := eng.mgs[id]; ok {
		return v, nil
	}
	return rdt.MonitorGroup{}, fmt.Errorf("Don't have group which id is %s", id)
}

func (eng *engine) GetAllApps() map[string]rdt.Application {
	return eng.apps
}

func DefaultEngineNewer(config string) (rdt.Engine, error) {
	var engine_config EngineConfig
	err := yaml.Unmarshal([]byte(config), &engine_config)
	if err != nil {
		klog.Fatal("error: %s", err)
		return &engine{}, err
	}

	// Policy is mandatory for engine
	if engine_config.Policy.Type == "" {
		return &engine{}, fmt.Errorf("Policy is missing")
	}

	eng := engine{
		Type:      rdt.DefaultEngineType,
		isRunning: false,
		apps:      make(map[string]rdt.Application),
		mgs:       make(map[string]rdt.MonitorGroup),
		cgs:       make(map[string]rdt.ControlGroup),
		pehs:      make(map[string]PostEventHandler),
	}

	eng.policy, err = rdt.NewPolicy(engine_config.Policy.Type, &eng, engine_config.Policy.Config)
	if err != nil {
		return &engine{}, err
	}

	// Watcher
	for _, watcher_config := range engine_config.Watchers {
		watcher, err := rdt.NewWatcher(watcher_config.Type, &eng, watcher_config.Config)
		if err != nil {
			klog.Infof("Fail to create watcher %s, err is ", watcher_config.Type, err)
			continue
		}

		eng.watchers = append(eng.watchers, watcher)
	}

	// Monitor
	for _, monitor_config := range engine_config.Monitors {
		monitor, err := rdt.NewMonitor(monitor_config.Type, &eng, monitor_config.Config)
		if err != nil {
			klog.Infof("Fail to create monitor %v, err is %v", monitor_config.Type, err)
			continue
		}

		eng.monitors = append(eng.monitors, monitor)
	}

	// add default executor
	executor, err := rdt.NewExecutor("default_executor", &eng, "")
	if err != nil {
		klog.Infof("Fail to create default executor, err is %v", err)
	}
	eng.executors = append(eng.executors, executor)
	// Executor
	for _, executor_config := range engine_config.Executors {
		executor, err := rdt.NewExecutor(executor_config.Type, &eng, executor_config.Config)
		if err != nil {
			klog.Infof("Fail to create executor %s, err is ", executor_config.Type, err)
			continue
		}

		eng.executors = append(eng.executors, executor)
	}

	eng.Initialize()

	return &eng, nil
}

func (eng *engine) Initialize() {
	eng.RegisterEventHandler(rdt.EventTargetPolicy, eng.onPolicy, "engine.onPolicy")
	eng.RegisterEventHandler(rdt.EventTargetMonitorReceiver, eng.onMonitorReceiver, "engine.onMonitorReceiver")
	eng.RegisterEventHandler(rdt.EventTargetMonitor, eng.onMonitor, "engine.onMonitor")
	eng.RegisterEventHandler(rdt.EventTargetWatcher, eng.onWatcher, "engine.onWatcher")
	eng.RegisterEventHandler(rdt.EventTargetExecutor, eng.onExecutor, "engine.onExecutor")
	eng.RegisterEventHandler(rdt.EventTargetStop, nil, "")

	eng.RegisterPostEventHandler(rdt.EventTypeApp, eng.onApp, "engine.onApponApp")

	err := eng.policy.Initialize()
	if err != nil {
		klog.Errorf("policy init fail %v", err)
	}

	for _, watcher := range eng.watchers {
		err := watcher.Initialize()
		if err != nil {
			klog.Errorf("watcher %s init fail %v", watcher.GetType(), err)
		}
	}

	for _, monitor := range eng.monitors {
		err := monitor.Initialize()
		if err != nil {
			klog.Errorf("watcher %s init fail %v", monitor.GetType(), err)
		}
	}
	for _, executor := range eng.executors {
		err := executor.Initialize()
		if err != nil {
			klog.Errorf("watcher %s init fail %v", executor.GetType(), err)
		}
	}
}

// Policy event handler
func (eng *engine) onPolicy(data rdt.EventData) (rdt.EventResponse, error) {
	if eng.match(eng.policy.EventFilter(), data.Key()) {
		go eng.policy.OnEvent(data)
	}

	return nil, nil
}

// Policy event handler
func (eng *engine) onApp(data rdt.EventData, resp rdt.EventResponse) {
	if eng.match(eng.policy.EventFilter(), data.Key()) {
		klog.Infof("----------postPolicy----------------")
	}
}

// MonitorReceiver event handler
func (eng *engine) onMonitorReceiver(data rdt.EventData) (rdt.EventResponse, error) {
	for _, mr := range eng.monitor_receivers {
		if eng.match(mr.EventFilter(), data.Key()) {
			go mr.OnEvent(data)
		}
	}

	return nil, nil
}

// Monitor event handler
func (eng *engine) onMonitor(data rdt.EventData) (rdt.EventResponse, error) {
	for _, mon := range eng.monitors {
		if eng.match(mon.EventFilter(), data.Key()) {
			go mon.OnEvent(data)
		}
	}

	return nil, nil
}

// Watcher event handler
func (eng *engine) onWatcher(data rdt.EventData) (rdt.EventResponse, error) {
	for _, wat := range eng.watchers {
		if eng.match(wat.EventFilter(), data.Key()) {
			go wat.OnEvent(data)
		}
	}

	return nil, nil
}

// Executor event handler
func (eng *engine) onExecutor(data rdt.EventData) (rdt.EventResponse, error) {
	for _, exe := range eng.executors {
		if eng.match(exe.EventFilter(), data.Key()) {
			go exe.OnEvent(data)
		}
	}

	return nil, nil
}

func (eng *engine) match(filter []string, eventKey string) bool {
	if eventKey == "" || len(filter) == 0 {
		return true
	}

	for _, f := range filter {
		if f == eventKey {
			return true
		}
	}

	return false
}

// Engine interface
func (eng *engine) GetType() string {
	return eng.Type
}

func (eng *engine) Start() error {
	klog.Infof("Engine start.")

	// TODO: all components need to start ?
	go eng.policy.Start()
	for _, executor := range eng.executors {
		go executor.Start()
	}

	for _, watcher := range eng.watchers {
		go watcher.Start()
	}

	for _, monitor := range eng.monitors {
		go monitor.Start()
	}

	if eng.isRunning {
		klog.Infof("Engine is already running.")
		return nil
	}

	eng.isRunning = true

	// Start event loop
	cases := make([]reflect.SelectCase, len(eng.ehs))
	for i, eh := range eng.ehs {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(eh.ch),
		}
	}

	for {
		chosen, value, ok := reflect.Select(cases)
		if !ok {
			klog.Warning("select cases failed")
			break
		}

		data, ok := value.Interface().(rdt.EventData)
		if ok {
			if eng.ehs[chosen].t == rdt.EventTargetStop {
				break
			}

			for _, handle := range eng.ehs[chosen].handles {
				if handle.f != nil {
					go func() {
						resp, err := handle.f(data)
						if err != nil {
							klog.Warning(handle.msg + ":" + err.Error())
						}

						if peh, ok := eng.pehs[data.GetEventType()]; ok {
							for _, pet := range data.PostEventTarget() {
								if eng.ehs[chosen].t == pet {
									for _, handle := range peh.handles {
										handle.f(data, resp)
									}
								}
							}
						}
					}()
				}
			}
		}
	}

	eng.isRunning = false

	klog.Infof("Engine stop.")

	return nil
}

func (eng *engine) Stop() error {
	eng.EmitEvent(&rdt.StopEvent{})
	return nil
}

func (eng *engine) RegisterApp(name string, app rdt.ApplicationSpec) error {
	eng.Lock()
	defer eng.Unlock()

	if _, ok := eng.apps[name]; ok {
		return fmt.Errorf("Duplicate application: " + name)
	}

	eng.apps[name] = rdt.Application{
		Spec: app,
		Status: rdt.ApplicationStatus{
			State: 0,
		},
	}

	eng.EmitEvent(&rdt.AppEvent{
		Name:      name,
		Operation: rdt.OperationAdd,
	})

	return nil
}

func (eng *engine) UnRegisterApp(name string) error {
	eng.Lock()
	defer eng.Unlock()

	if _, ok := eng.apps[name]; !ok {
		return fmt.Errorf("Application: " + name + " is not found")
	}

	// Todo: remove from eng.apps when all resources aligned
	app := eng.apps[name]
	app.Status.State = -1
	eng.apps[name] = app

	eng.EmitEvent(&rdt.AppEvent{
		Name:      name,
		Operation: rdt.OperationRemove,
	})

	return nil
}

func (eng *engine) GetApp(name string) (rdt.Application, error) {
	eng.Lock()
	defer eng.Unlock()

	if _, ok := eng.apps[name]; !ok {
		return rdt.Application{}, fmt.Errorf("Application: " + name + " is not found")
	}

	return eng.apps[name], nil
}

func (eng *engine) GetAppControlSchema(name string, version string) (string, error) {
	eng.Lock()
	defer eng.Unlock()

	app, err := eng.GetApp(name)
	if err != nil {
		return "", err
	}

	if app.Status.State != 1 {
		return "", fmt.Errorf("Application: " + name + " is not in running state")
	}

	if app.Status.ControlGroupId == "" {
		return "", fmt.Errorf("Application: " + name + " control group is not active")
	}

	grp, ok := eng.cgs[app.Status.ControlGroupId]
	if !ok {
		return "", fmt.Errorf("Control Group: " + app.Status.ControlGroupId + " is not found")
	}

	if grp.GetState() != 1 {
		return "", fmt.Errorf("Control Group: " + app.Status.ControlGroupId + " is not in running state")
	}

	return grp.Schema.ToString(version), nil
}

func (eng *engine) AddMonitorReceiver(receiver rdt.MonitorReceiver) {
	eng.monitor_receivers = append(eng.monitor_receivers, receiver)
}

// ResourceHandler interface
// Event
func (eng *engine) RegisterEventHandler(t string, f rdt.EventFunc, msg string) {
	found := false
	handle := EventHandle{
		f:   f,
		msg: msg,
	}

	for _, eh := range eng.ehs {
		if eh.t == t {
			eh.handles = append(eh.handles, handle)
			found = true
			break
		}
	}

	if !found {
		eng.ehs = append(eng.ehs, EventHandler{
			t:       t,
			ch:      make(chan rdt.EventData, eventChanSize),
			handles: []EventHandle{handle},
		})
	}
}

func (eng *engine) RegisterPostEventHandler(t string, f rdt.PostEventFunc, msg string) {
	handle := PostEventHandle{
		f:   f,
		msg: msg,
	}

	if peh, ok := eng.pehs[t]; ok {
		peh.handles = append(peh.handles, handle)
	} else {
		eng.pehs[t] = PostEventHandler{
			t:       t,
			handles: []PostEventHandle{handle},
		}
	}
	klog.Infof("--------pehs______%v", eng.pehs)
}

func (eng *engine) EmitEvent(data rdt.EventData) {
	for _, target := range data.Target() {
		for _, eh := range eng.ehs {
			if eh.t == target {
				eh.ch <- data
				break
			}
		}
	}
}

// Monitor Group Operations
func (eng *engine) CreateMonitorGroup(id string, grp rdt.MonitorGroup) error {
	eng.Lock()
	defer eng.Unlock()

	if _, ok := eng.mgs[id]; ok {
		return fmt.Errorf("Duplicate monitor group: " + id)
	}

	// Todo: validate ParentId and ControlGroupId
	grp.SetState(0) // Set Pending sstate
	eng.mgs[id] = grp

	eng.EmitEvent(&rdt.MonitorGroupEvent{
		Id:        id,
		Operation: rdt.OperationAdd,
	})

	return nil
}

func (eng *engine) RemoveMonitorGroup(id string) error {
	eng.Lock()
	defer eng.Unlock()

	if _, ok := eng.mgs[id]; !ok {
		return fmt.Errorf("Monitor Group: " + id + " is not found")
	}

	// Todo: remove from eng.apps when all resources aligned
	grp := eng.mgs[id]
	grp.SetState(-1) // Set Remove state

	eng.EmitEvent(&rdt.MonitorGroupEvent{
		Id:        id,
		Operation: rdt.OperationRemove,
		//GroupPath: utils.GetMonitorGroupPath(grp),
	})

	return nil
}

// Control Group Operations
func (eng *engine) CreateControlGroup(id string, grp rdt.ControlGroup) error {
	eng.Lock()
	defer eng.Unlock()

	if _, ok := eng.cgs[id]; ok {
		return fmt.Errorf("Duplicate control group: " + id)
	}

	if !grp.Validate() {
		return fmt.Errorf("Invalidate control group: " + id)
	}

	grp.SetState(0)
	eng.cgs[id] = grp

	eng.EmitEvent(&rdt.ControlGroupEvent{
		Id:        id,
		Operation: rdt.OperationAdd,
	})

	return nil
}

func (eng *engine) RemoveControlGroup(id string) error {
	eng.Lock()
	defer eng.Unlock()

	if _, ok := eng.cgs[id]; !ok {
		return fmt.Errorf("Control Group: " + id + " is not found")
	}

	// Todo: remove from eng.apps when all resources aligned
	grp := eng.cgs[id]
	grp.SetState(-1) // Set Remove state

	eng.EmitEvent(&rdt.ControlGroupEvent{
		Id:        id,
		Operation: rdt.OperationRemove,
	})

	return nil
}

func (eng *engine) SetControlGroupSchema(id string, schema rdt.ControlSchema) error {
	eng.Lock()
	defer eng.Unlock()

	if _, ok := eng.cgs[id]; !ok {
		return fmt.Errorf("Control Group: " + id + " is not found")
	}

	if !schema.Validate() {
		return fmt.Errorf("Invalidate control group schema")
	}

	grp := eng.cgs[id]
	if grp.GetState() == -1 {
		return fmt.Errorf("Control Group: " + id + " is removed")
	}

	eng.EmitEvent(&rdt.ControlGroupEvent{
		Id:        id,
		Operation: rdt.OperationUpdate,
		Schema:    schema,
	})

	return nil
}

// Data
func (eng *engine) ReportData(id string, data rdt.MonitorData) {
	eng.EmitEvent(&rdt.DataEvent{
		Id:   id,
		Data: data,
	})
}

// Refresh
func (eng *engine) Refresh() {
	eng.EmitEvent(&rdt.RefreshEvent{})
}

func (eng *engine) RefreshMonitorGroups(grps []rdt.MonitorGroup) {
	// Todo: update eng.mgs
	eng.Lock()
	defer eng.Unlock()
}

func (eng *engine) RefreshControlGroups(grps []rdt.ControlGroup) {
	// Todo: update eng.cgs
	eng.Lock()
	defer eng.Unlock()
}
