/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package executor

import (
	//	"fmt"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"
	"time"
)

func init() {
	rdt.RegisterExecutor("default_executor", DefaultExecutorNewer)
}

var (
	SleepDuration = 10 * time.Second
)

type DefaultExecutorConfig struct {
	test string
}

type DefaultExecutor struct {
	rdt.BaseObject
	Type string
	test string
}

func (e *DefaultExecutor) getRdtL3Info() error {
	return nil
}

func (e *DefaultExecutor) getRdtMbInfo() error {
	return nil
}

func (e *DefaultExecutor) getRdtSchemata() error {
	return nil
}

func (e *DefaultExecutor) getRdtCacheIds() error {
	return nil
}

func (e *DefaultExecutor) getControlGroups() error {
	return nil
}

func (e *DefaultExecutor) getMonitorGroups() error {
	return nil
}

func (e *DefaultExecutor) getRdtInfo() error {
	if err := e.getRdtL3Info(); err != nil {
		return err
	}
	if err := e.getRdtMbInfo(); err != nil {
		return err
	}
	if err := e.getRdtSchemata(); err != nil {
		return err
	}
	if err := e.getRdtCacheIds(); err != nil {
		return err
	}
	if err := e.getControlGroups(); err != nil {
		return err
	}
	if err := e.getMonitorGroups(); err != nil {
		return err
	}
	return nil
}

func (e *DefaultExecutor) Initialize() error {
	return e.getRdtInfo()
}

func (e *DefaultExecutor) Start() error {
	for {
		e.getRdtInfo()
		time.Sleep(SleepDuration)
	}
	return nil
}

func (e *DefaultExecutor) Stop() error {
	return nil
}

func DefaultExecutorNewer(eng rdt.ResourceHandler, config string) (rdt.Executor, error) {
	var default_config DefaultExecutorConfig
	err := yaml.Unmarshal([]byte(config), &default_config)
	if err != nil {
		klog.Fatal("error: %s", err)
		return &DefaultExecutor{}, err
	}

	e := DefaultExecutor{
		rdt.BaseObject{eng},
		"default",
		default_config.test,
	}

	return &e, nil
}

func (e *DefaultExecutor) GetType() string {
	return e.Type
}

func (e *DefaultExecutor) OnEvent(data rdt.EventData) (rdt.EventResponse, error) {
	t := data.GetEventType()

	switch t {
	case rdt.EventTypeMonitorGroup:
		evt := data.(*rdt.MonitorGroupEvent)
		klog.Infof("Executor Receive MonitorGroupEvent: %s", evt.Id)
	case rdt.EventTypeControlGroup:
		evt := data.(*rdt.ControlGroupEvent)
		klog.Infof("Executor Receive ControlGroupEvent:%s", evt.Id)
	}
	return nil, nil
}

func (o *DefaultExecutor) EventFilter() []string {
	return []string{"default"}
}
