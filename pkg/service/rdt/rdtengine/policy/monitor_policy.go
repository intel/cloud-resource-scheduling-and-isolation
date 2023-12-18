/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package policy

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"
)

func init() {
	rdt.RegisterPolicy("monitor", MonitorPolicyNewer)
}

type SampleConfig struct {
	ConfigPath string
}

type PureMonitorPolicy struct {
	rdt.BaseObject
	Type string
	path string
}

func (p *PureMonitorPolicy) Initialize() error {
	return nil
}

func (p *PureMonitorPolicy) Start() error {
	return nil
}

func (p *PureMonitorPolicy) Stop() error {
	return nil
}

func MonitorPolicyNewer(eng rdt.ResourceHandler, config string) (rdt.Policy, error) {
	var sample_config SampleConfig
	err := yaml.Unmarshal([]byte(config), &sample_config)
	if err != nil {
		klog.Fatal("error: %s", err)
		return &PureMonitorPolicy{}, err
	}

	po := PureMonitorPolicy{
		rdt.BaseObject{eng},
		"monitor",
		sample_config.ConfigPath,
	}

	return &po, nil
}

func (p *PureMonitorPolicy) GetType() string {
	return p.Type
}

func (p *PureMonitorPolicy) String() string {
	return fmt.Sprintf("Policy %s: %s", p.Type, p.path)
}

func (e *PureMonitorPolicy) OnEvent(data rdt.EventData) (rdt.EventResponse, error) {
	t := data.GetEventType()

	switch t {
	case rdt.EventTypeApp:
		evt := data.(*rdt.AppEvent)
		klog.Infof("Policy: Receive AppEvent, App is: %s", evt.Name)

		switch evt.Operation {
		case rdt.OperationAdd:
			// Create Monitor Group
			err := e.RH.CreateMonitorGroup(evt.Name, rdt.MonitorGroup{
				Id:             evt.Name,
				ControlGroupId: "",
			})
			if err != nil {
				return rdt.ErrorResponse{Err: err}, err
			}
		case rdt.OperationRemove:
			err := e.RH.RemoveMonitorGroup(evt.Name)
			if err != nil {
				return rdt.ErrorResponse{Err: err}, err
			}
		}

	case rdt.EventTypeData:
		evt := data.(*rdt.DataEvent)
		klog.V(5).Infof("Policy: Receive DataEvent: Monitor Group is %s, Data is %s", evt.Id, evt.Data.ToString())
	}
	return nil, nil
}
