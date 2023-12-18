/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package executor

import (
	//	"fmt"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	"os"
	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"
	"sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/utils"
)

func init() {
	rdt.RegisterExecutor("rdt_executor", RdtExecutorNewer)
}

type SampleConfig struct {
	test string
}

type RdtExecutor struct {
	rdt.BaseObject
	Type string
	test string
}

func (e *RdtExecutor) Initialize() error {
	//TODO implement me
	return nil
}

func (e *RdtExecutor) Start() error {
	return nil
}

func (e *RdtExecutor) Stop() error {
	return nil
}

func RdtExecutorNewer(eng rdt.ResourceHandler, config string) (rdt.Executor, error) {
	var sample_config SampleConfig
	err := yaml.Unmarshal([]byte(config), &sample_config)
	if err != nil {
		klog.Fatal("error: %s", err)
		return &RdtExecutor{}, err
	}

	e := RdtExecutor{
		rdt.BaseObject{eng},
		"rdt_executor",
		sample_config.test,
	}

	return &e, nil
}

func (e *RdtExecutor) GetType() string {
	return e.Type
}

func (e *RdtExecutor) OnEvent(data rdt.EventData) (rdt.EventResponse, error) {
	t := data.GetEventType()

	switch t {
	case rdt.EventTypeMonitorGroup:
		evt := data.(*rdt.MonitorGroupEvent)
		klog.Infof("Executor Receive MonitorGroupEvent: %s", evt.Id)
		switch evt.Operation {
		case rdt.OperationAdd:
			grp, err := e.RH.GetMonitorGroup(evt.Id)
			if err != nil {
				klog.Errorf("GetMonitorGroup fail, error is %v", err)
			}
			err = e.createMonitorGroup(utils.GetMonitorGroupPath(grp))
			if err != nil {
				klog.Errorf("Executor Receive MonitorGroupEvent with operationAdd error %v", err)
			}
		case rdt.OperationRemove:
			klog.Infof("Executor Receive MonitorGroupEvent with operationRemove")
			grp, err := e.RH.GetMonitorGroup(evt.Id)
			if err != nil {
				klog.Errorf("GetMonitorGroup fail, error is %v", err)
			}
			err = e.removeMonitorGroup(utils.GetMonitorGroupPath(grp))
			if err != nil {
				klog.Errorf("Executor Receive MonitorGroupEvent with operationAdd error %v", err)
			}
		case rdt.OperationUpdate:
			klog.Infof("Executor Receive MonitorGroupEvent with operationUpdate")
		default:
			klog.Errorf("Wrong Operation with %s", evt.Operation)
		}
	case rdt.EventTypeControlGroup:
		evt := data.(*rdt.ControlGroupEvent)
		klog.Infof("Executor Receive ControlGroupEvent:%s", evt.Id)
	case rdt.EventTaskIds:
		evt := data.(*rdt.TaskIdsEvent)
		//klog.Infof("Executor Receive new taskdIds: %v for %s", evt.Tasks, evt.MonitorGroupPath)
		grp, err := e.RH.GetMonitorGroup(evt.MonitorGroupPath)
		if err != nil {
			klog.Errorf("GetMonitorGroup fail, error is %v", err)
		}
		path := utils.GetMonitorGroupPath(grp)
		err = utils.WriteTaskIDsToFile(evt.Tasks, path+"/tasks")
		if err != nil {
			klog.Errorf("Write task ids fail %v", err)
		}
	}
	return nil, nil
}

func (o *RdtExecutor) EventFilter() []string {
	return []string{"filter1", "filter2"}
}

func (e *RdtExecutor) createMonitorGroup(path string) error {
	// os.Mkdir
	// 创建文件夹
	err := os.Mkdir(path, 0755)
	if err != nil {
		return err
	}
	return nil
}

func (e *RdtExecutor) removeMonitorGroup(path string) error {
	// os.Mkdir
	// 创建文件夹
	err := os.RemoveAll(path)
	if err != nil {
		return err
	}
	return nil
}
