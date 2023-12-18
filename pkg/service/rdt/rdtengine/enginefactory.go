/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdtengine

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	"net/http"
	"os"
)

var Reg *prometheus.Registry
var Hostname string

func init() {
	klog.Infof("Engine initialized")
	tmp, err := os.Hostname()
	Hostname = tmp
	if err != nil {
		klog.Infof("Error getting hostname: %v\n", err)
	}
	go func() {
		Reg = prometheus.NewRegistry()
		http.Handle("/metrics", promhttp.HandlerFor(
			Reg,
			promhttp.HandlerOpts{
				// Opt into OpenMetrics to support exemplars.
				EnableOpenMetrics: true,
			},
		))
		http.ListenAndServe(":8080", nil)
	}()
}

const (
	DefaultEngineType = "Default"
)

type EngineNewerFunc func(config string) (Engine, error)
type PolicyNewerFunc func(rh ResourceHandler, config string) (Policy, error)
type WatcherNewerFunc func(rh ResourceHandler, config string) (Watcher, error)
type MonitorNewerFunc func(rh ResourceHandler, config string) (Monitor, error)
type ExecutorNewerFunc func(rh ResourceHandler, config string) (Executor, error)

var (
	engineNewers   = make(map[string]EngineNewerFunc)
	policyNewers   = make(map[string]PolicyNewerFunc)
	watcherNewers  = make(map[string]WatcherNewerFunc)
	monitorNewers  = make(map[string]MonitorNewerFunc)
	executorNewers = make(map[string]ExecutorNewerFunc)
)

// engine
func RegisterEngine(t string, f EngineNewerFunc) {
	engineNewers[t] = f
}
func NewEngine(t string, config string) (Engine, error) {
	if creator, ok := engineNewers[t]; ok {
		return creator(config)
	}
	return nil, fmt.Errorf("invalid engine type %q", t)
}

// policy
func RegisterPolicy(t string, f PolicyNewerFunc) {
	policyNewers[t] = f
}
func NewPolicy(t string, rh ResourceHandler, config string) (Policy, error) {
	if creator, ok := policyNewers[t]; ok {
		return creator(rh, config)
	}
	return nil, fmt.Errorf("invalid policy type %q", t)
}

// watcher
func RegisterWatcher(t string, f WatcherNewerFunc) {
	watcherNewers[t] = f
}
func NewWatcher(t string, rh ResourceHandler, config string) (Watcher, error) {
	if creator, ok := watcherNewers[t]; ok {
		return creator(rh, config)
	}
	return nil, fmt.Errorf("invalid watcher type %q", t)
}

// monitor
func RegisterMonitor(t string, f MonitorNewerFunc) {
	monitorNewers[t] = f
}
func NewMonitor(t string, rh ResourceHandler, config string) (Monitor, error) {
	if creator, ok := monitorNewers[t]; ok {
		return creator(rh, config)
	}
	return nil, fmt.Errorf("invalid monitor type %q", t)
}

// executor
func RegisterExecutor(t string, f ExecutorNewerFunc) {
	executorNewers[t] = f
}
func NewExecutor(t string, rh ResourceHandler, config string) (Executor, error) {
	if creator, ok := executorNewers[t]; ok {
		return creator(rh, config)
	}
	return nil, fmt.Errorf("invalid executor type %q", t)
}

type RawConfig struct {
	Name   string
	Type   string
	Config string
}

type EngineListConfig struct {
	Engines []RawConfig
}

// Engine factory to create rdt.Engine
type EngineFactory struct {
	engines map[string]Engine
}

// Create a EngineFactory instance from a configration file
func NewFactoryFromFile(conf_file string) (*EngineFactory, error) {
	conf_bytes, err := os.ReadFile(conf_file)
	if err != nil {
		klog.Fatal("%s", err)
		return &EngineFactory{}, err
	}

	return NewFactory(conf_bytes)
}

// Create a EngineFactory instance from configuration data
func NewFactory(conf_bytes []byte) (*EngineFactory, error) {
	var engine_list_config EngineListConfig
	err := yaml.Unmarshal(conf_bytes, &engine_list_config)
	if err != nil {
		klog.Fatal("error: %s", err)
		return &EngineFactory{}, err
	}

	// Create Engine Factory
	engine_factory := EngineFactory{
		engines: make(map[string]Engine),
	}

	for _, engine_config := range engine_list_config.Engines {
		if engine_config.Name == "" {
			klog.Errorf("Fail to create engine as Name is missing.")
			continue
		}

		if engine_config.Type == "" {
			engine_config.Type = DefaultEngineType
		}

		engine, err := NewEngine(engine_config.Type, engine_config.Config)
		if err != nil {
			klog.Errorf("Fail to create engine %s, error is %v", engine_config.Type, err)
			continue
		}

		engine_factory.engines[engine_config.Name] = &engineProxy{engine}
	}

	return &engine_factory, nil
}

func (f *EngineFactory) GetEngine(name string) (Engine, error) {
	if engine, ok := f.engines[name]; ok {
		return engine, nil
	}

	return nil, fmt.Errorf("engine name %q is not found", name)
}
