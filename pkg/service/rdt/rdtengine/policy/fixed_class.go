/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package policy

import (
	"fmt"
	"k8s.io/klog/v2"
	"os"
	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"
	"sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/monitor"
	"sigs.k8s.io/yaml"
	"time"
)

func init() {
	rdt.RegisterPolicy("FixedClass", FixedClassPolicyNewer)
}

type RDTConfig struct {
	Classes        []string `json:"classes,omitempty"`
	UpdateInterval int64    `json:"updateInterval,omitempty"`
	Adjustments    map[string]struct {
		Threshold Threshold         `json:"threshold,omitempty"`
		AR        AjustmentResource `json:"adjustmentResource,omitempty"`
	} `json:"adjustments,omitempty"`
}

type Threshold struct {
	L3CacheMiss    map[string]float64 `json:"l3CacheMiss,omitempty"`
	CpuUtilization map[string]float64 `json:"cpuUtilization,omitempty"`
	L3Usage        map[string]float64 `json:"l3usage,omitempty"`
	Mb             map[string]float64 `json:"Mb,omitempty""`
}

type AjustmentResource struct {
	L3 map[string]string `json:"l3Allocation,omitempty"`
}

type FixedClassPolicyConfig struct {
	ConfigPath string
}

type FixedClassPolicy struct {
	rdt.BaseObject
	Type      string
	path      string
	rdtConfig RDTConfig
	L3Usage   map[string]monitor.L3Data
	MB        map[string]monitor.MbData
}

func (p *FixedClassPolicy) Initialize() error {
	// Init all fixed classes

	return nil
}

func (p *FixedClassPolicy) getRDTClassMB(className string) (float64, error) {
	if mb, ok := p.MB[className]; ok {
		return mb.TotalMb, nil
	}
	return 0.0, nil
}

func (p *FixedClassPolicy) getRDTClassL3Usage(className string) (float64, error) {
	if mb, ok := p.L3Usage[className]; ok {
		return mb.L3Cache, nil
	}
	return 0.0, nil
}

func (p *FixedClassPolicy) findAppropriateAdjustment() (bool, string, error) {
	// collect metrics and decide whether and which adjustment to make.
	l3s := make(map[string]float64, 0)
	mbs := make(map[string]float64, 0)
	for _, class := range p.rdtConfig.Classes {
		ans, err := p.getRDTClassL3Usage(class)
		if err != nil {
			return false, "", err
		}
		l3s[class] = ans
		ans, err = p.getRDTClassMB(class)
		if err != nil {
			return false, "", err
		}
		mbs[class] = ans
		klog.Infof("rdt class %v l3 cache: %v, mb: %v", class, l3s[class], mbs[class])
	}
	for k, v := range p.rdtConfig.Adjustments {
		flag := true
		if v.Threshold.L3Usage != nil {
			for class, l3 := range v.Threshold.L3Usage {
				if l3s[class] > l3 {
					flag = false
					break
				}
			}
		}
		if flag && v.Threshold.Mb != nil {
			for class, mb := range v.Threshold.Mb {
				if mbs[class] > mb {
					flag = false
					break
				}
			}
		}
		if flag {
			return true, k, nil
		}
	}
	return false, "", nil
}

func (p *FixedClassPolicy) Start() error {
	var err error
	if resctrlL3Info, err = getRdtL3Info(); err != nil {
		klog.Fatalf("Failed to get rdt l3 info: %v", err)
	}

	for _, class := range p.rdtConfig.Classes {
		err := p.RH.CreateMonitorGroup(class, rdt.MonitorGroup{
			Id:             class,
			ControlGroupId: class,
		})
		if err != nil {
			klog.Errorf("CreateMonitorGroup fail %v", err)
		}
	}
	oldAdjustment := ""
	for {
		klog.Infof("---------------Load Metrics and Adjust Resources------------------")
		flag, adjustment, err := p.findAppropriateAdjustment()
		if err != nil {
			klog.Fatalf("Can not find appropriate adjustment: %v", err)
		}
		if !flag && oldAdjustment == "" {
			klog.Infof("Donot need to adjust resource. Resources have not been adjusted yet.")
		} else if !flag && oldAdjustment != "" || flag && adjustment == oldAdjustment {
			klog.Infof("Donot need to adjust resource. The current adjustment is %v", oldAdjustment)
		} else {
			klog.Infof("Need to adjust resource to %v.", adjustment)
			oldAdjustment = adjustment
			if err := p.adjustResource(adjustment); err != nil {
				klog.Fatalf("Can not adjust resource: %v", err)
			}
		}
		klog.Infof("------------------------------------------------------------------")
		time.Sleep(time.Duration(p.rdtConfig.UpdateInterval) * time.Second)
	}
	return nil
}

func (p *FixedClassPolicy) adjustL3Cache(adjustment string) error {
	for class, l3 := range p.rdtConfig.Adjustments[adjustment].AR.L3 {
		if err := modifyRDTClassL3schema(class, l3); err != nil {
			return err
		}
	}
	return nil
}

func (p *FixedClassPolicy) adjustResource(adjustment string) error {
	if err := p.adjustL3Cache(adjustment); err != nil {
		return fmt.Errorf("can not adjust L3 cache: %v", err)
	}
	return nil
}

func (p *FixedClassPolicy) Stop() error {
	return nil
}

func FixedClassPolicyNewer(eng rdt.ResourceHandler, config string) (rdt.Policy, error) {
	var fixedClass_config FixedClassPolicyConfig
	err := yaml.Unmarshal([]byte(config), &fixedClass_config)
	if err != nil {
		klog.Fatal("error: %s", err)
		return &FixedClassPolicy{}, err
	}

	c, err := os.ReadFile(fixedClass_config.ConfigPath)
	if err != nil {
		klog.Fatal(err)
	}
	var conf RDTConfig
	err = yaml.Unmarshal(c, &conf)
	po := FixedClassPolicy{
		rdt.BaseObject{eng},
		"FixedClass",
		fixedClass_config.ConfigPath,
		conf,
		make(map[string]monitor.L3Data),
		make(map[string]monitor.MbData),
	}

	return &po, nil
}

func (p *FixedClassPolicy) GetType() string {
	return p.Type
}

func (p *FixedClassPolicy) String() string {
	return fmt.Sprintf("Policy %s: %s", p.Type, p.path)
}

func (p *FixedClassPolicy) OnEvent(data rdt.EventData) (rdt.EventResponse, error) {
	t := data.GetEventType()

	switch t {
	case rdt.EventTypeApp:
		evt := data.(*rdt.AppEvent)
		klog.Infof("Policy: Receive AppEvent, App is: %s", evt.Name)
		switch evt.Operation {
		case rdt.OperationAdd:
			//TODO: Create Class Control Group if not exist
			app, err := p.RH.GetApp(evt.Name)
			if err != nil {
				klog.Errorf("err is %v, state is %v", err, app)
			}
		}
	case rdt.EventTypeData:
		evt := data.(*rdt.DataEvent)
		klog.Infof("Policy: Receive DataEvent: Monitor Group is %s, Data is %s", evt.Id, evt.Data.ToString())
		switch evt.Key() {
		case monitor.L3_Monitor:
			l3 := evt.Data.(*monitor.L3Data)
			p.L3Usage[evt.Id] = *l3

		case monitor.Mb_Monitor:
			mb := evt.Data.(*monitor.MbData)
			p.MB[evt.Id] = *mb
		}
	}
	return nil, nil
}
