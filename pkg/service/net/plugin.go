/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

const GROUP_SEPARATE_LAZY = 0
const GROUP_SEPARATE_DYNAMIC = 1
const GROUP_SHARE = 2

type Plugin interface {
	Configure(groups []Group) error
	RegisterApp(app *AppInfo) error
	UnRegisterApp(app *AppInfo) error
	SetLimit(group string, app *AppInfo, limit BWLimit) error
}

type CommonPlugin struct {
}

func NewCommonPlugin() CommonPlugin {
	return CommonPlugin{}
}

func (g *CommonPlugin) Configure(groups []Group) error {
	return nil
}

func (g *CommonPlugin) RegisterApp(app *AppInfo) error {
	return nil
}

func (g *CommonPlugin) UnRegisterApp(app *AppInfo) error {
	return nil
}

func (g *CommonPlugin) SetLimit(group string, app *AppInfo, limit BWLimit) error {
	return nil
}
