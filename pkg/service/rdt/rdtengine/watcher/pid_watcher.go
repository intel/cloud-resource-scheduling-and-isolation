/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package watcher

import (
	//	"fmt"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"
	"sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/utils"
	"time"
)

func init() {
	rdt.RegisterWatcher("pid_watcher", PidWatcherNewer)
}

type SampleConfig struct {
	Interval int
}

type PidWatcher struct {
	rdt.BaseObject
	Type     string
	Interval int
}

func (e *PidWatcher) Initialize() error {
	return nil
}

func (e *PidWatcher) Start() error {
	for {
		apps := e.RH.GetAllApps()
		for _, app := range apps {
			if app.Status.State == -1 {
				time.Sleep(time.Duration(e.Interval) * time.Second)
				continue
			}
			containers, err := utils.GetSubCgroupPath(app.Spec.CgroupPath)
			if err != nil {
				klog.Errorf("get sub cgroup path err is %v", err)
				continue
			}

			var ids []int32
			for _, path := range containers {
				containerIds, err := utils.ReadCPUTasks(path + "/cgroup.procs")
				if err != nil {
					klog.Errorf("can't read this app's taskids, reason: %v", err)
					continue
				}
				klog.Infof("Pod cgourp is %s, container is %s, containerIds is %v", app.Spec.CgroupPath, path, containerIds)
				ids = append(ids, containerIds...)
			}

			klog.Infof("Pod cgourp is %s, all ids is %v", app.Spec.CgroupPath, ids)

			if len(ids) > 0 {
				e.RH.EmitEvent(&rdt.TaskIdsEvent{
					Tasks:            ids,
					MonitorGroupPath: app.Spec.Name,
				})
			}
		}
		time.Sleep(time.Duration(e.Interval) * time.Second)
	}
	//for {
	//	apps := e.RH.ListApps()
	//	for id, app := range apps {
	//		ids := utils.ReadCPUTasks(app.CgroupPath)
	//		newIds := utils.NewTasks(app.Tasks, ids)
	//		e.RH.EmitEvent(&rdt.RefreshEvent{})
	//	}
	//	time.Sleep(time.Duration(e.Interval) * time.Second)
	//}
	return nil
}

func (e *PidWatcher) Stop() error {
	//TODO implement me
	panic("implement me")
}

func PidWatcherNewer(eng rdt.ResourceHandler, config string) (rdt.Watcher, error) {
	var sample_config SampleConfig
	err := yaml.Unmarshal([]byte(config), &sample_config)

	if err != nil {
		klog.Fatal("error: %s", err)
		return &PidWatcher{}, err
	}

	e := PidWatcher{
		rdt.BaseObject{eng},
		"pid_watcher",
		sample_config.Interval,
	}

	return &e, nil
}

func (e *PidWatcher) GetType() string {
	return e.Type
}

func (e *PidWatcher) OnEvent(data rdt.EventData) (rdt.EventResponse, error) {
	t := data.GetEventType()

	switch t {
	case rdt.EventTypeApp:
		evt := data.(*rdt.AppEvent)
		klog.Infof("Watcher: Receive AppEvent, App is: %s", evt.Name)
	}
	return nil, nil
}
