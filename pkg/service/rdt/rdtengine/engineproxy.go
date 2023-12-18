/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package rdtengine

// proxy for an Engine instance
type engineProxy struct {
	engine Engine
}

func (e *engineProxy) GetType() string {
	return e.engine.GetType()
}

func (e *engineProxy) AddMonitorReceiver(receiver MonitorReceiver) {
	e.engine.AddMonitorReceiver(receiver)
}

func (e *engineProxy) Start() error {
	return e.engine.Start()
}

func (e *engineProxy) Stop() error {
	return e.engine.Stop()
}

func (e *engineProxy) RegisterApp(name string, app ApplicationSpec) error {
	return e.engine.RegisterApp(name, app)
}

func (e *engineProxy) UnRegisterApp(name string) error {
	return e.engine.UnRegisterApp(name)
}

func (e *engineProxy) GetApp(name string) (Application, error) {
	return e.engine.GetApp(name)
}

func (e *engineProxy) GetAppControlSchema(name string, version string) (string, error) {
	return e.engine.GetAppControlSchema(name, version)
}
