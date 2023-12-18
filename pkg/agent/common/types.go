/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	utils "sigs.k8s.io/IOIsolation/pkg"
)

// operation 0:compress 1:dont do anything 2:decompress
type LimitOperation struct {
	ReadOp  int
	WriteOp int
	TotalOp int
}

type NetInfo struct {
	Id          string
	Name        string
	Ipaddr      string
	SndGroupBPS float64
	RcvGroupBPS float64
	QueueNum    int
}

type DevInfo struct {
	Id            string
	Name          string
	DevType       string
	DevMajorMinor string
	DInfo         *utils.DiskInfo
	NInfo         *utils.NicInfo
}
