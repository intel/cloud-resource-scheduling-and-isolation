/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"encoding/json"
	"fmt"

	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

const (
	BucketName string = "NET"
)

func AddGroup(n *NetEngine, group Group) error {
	groupBytes, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	bktName := fmt.Sprintf("%s.%s", BucketName, "GROUPS")
	err = n.persist.Save(bktName, group.Id, groupBytes)
	if err != nil {
		return fmt.Errorf("could not insert group: %v", err)
	}

	return err
}

func AddGroupLimit(n *NetEngine, group string, limit BWLimit) error {
	groupBytes, err := json.Marshal(limit)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	bktName := fmt.Sprintf("%s.%s", BucketName, "GROUPLIMIT")
	err = n.persist.Save(bktName, group, groupBytes)
	if err != nil {
		return fmt.Errorf("could not insert group limit: %v", err)
	}

	return err
}

func AddGroupSettable(n *NetEngine, groupSet string) error {
	groupBytes, err := json.Marshal(groupSet)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	bktName := BucketName
	err = n.persist.Save(bktName, "GROUPSETTABLE", groupBytes)
	if err != nil {
		return fmt.Errorf("could not insert group settable: %v", err)
	}

	return err
}

func AddNic(n *NetEngine, nic NICInfo) error {
	nicBytes, err := json.Marshal(nic)
	if err != nil {
		return fmt.Errorf("could not marshal nic json: %v", err)
	}

	bktName := fmt.Sprintf("%s.%s", BucketName, "NICS")
	err = n.persist.Save(bktName, nic.InterfaceName, nicBytes)
	if err != nil {
		return fmt.Errorf("could not insert nic: %v", err)
	}

	return err
}

func AddCni(n *NetEngine, cni CNI) error {
	cniBytes, err := json.Marshal(cni)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	bktName := fmt.Sprintf("%s.%s", BucketName, "CNIS")
	err = n.persist.Save(bktName, cni.Name, cniBytes)
	if err != nil {
		return fmt.Errorf("could not insert cni: %v", err)
	}

	return err
}

func AddServerInfo(n *NetEngine, si ServerInfo) error {
	serverBytes, err := json.Marshal(si)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	bktName := BucketName
	err = n.persist.Save(bktName, "serverInfo", serverBytes)
	if err != nil {
		return fmt.Errorf("could not insert serverInfo: %v", err)
	}

	return err
}

func AddAppInfo(n *NetEngine, appInfo AppInfo) error {
	appBytes, err := json.Marshal(appInfo)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	bktName := fmt.Sprintf("%s.%s", BucketName, "APPS")
	err = n.persist.Save(bktName, appInfo.AppName, appBytes)
	if err != nil {
		return fmt.Errorf("could not insert app: %v", err)
	}

	return err
}

func GetNics(n *NetEngine) ([]NICInfo, error) {
	nics := make([]NICInfo, 0)
	bktName := fmt.Sprintf("%s.%s", BucketName, "NICS")
	siBytes, err := n.persist.Load(bktName, "")
	if err != nil {
		klog.Warning("could not load nics: ", err)
		return nil, err
	}
	for _, v := range siBytes {
		var nic NICInfo
		if err = json.Unmarshal(v, &nic); err != nil {
			return nil, err
		}
		nics = append(nics, nic)
	}

	return nics, err
}

func GetCNIs(n *NetEngine) ([]CNI, error) {
	cnis := make([]CNI, 0)
	bktName := fmt.Sprintf("%s.%s", BucketName, "CNIS")
	siBytes, err := n.persist.Load(bktName, "")
	if err != nil {
		klog.Warning("could not load cni: ", err)
		return nil, err
	}
	for _, v := range siBytes {
		var cni CNI
		if err = json.Unmarshal(v, &cni); err != nil {
			return nil, err
		}
		cnis = append(cnis, cni)
	}

	return cnis, err
}

func GetServerInfo(n *NetEngine) (ServerInfo, error) {
	var si ServerInfo
	bktName := BucketName
	siBytes, err := n.persist.Load(bktName, "serverInfo")
	if err != nil {
		klog.Warning("could not load server info: ", err)
		return ServerInfo{}, err
	}
	for _, v := range siBytes {
		if err = json.Unmarshal(v, &si); err != nil {
			return ServerInfo{}, err
		}
	}

	return si, err
}

func GetGroups(n *NetEngine) ([]Group, error) {
	groups := make([]Group, 0)
	bktName := fmt.Sprintf("%s.%s", BucketName, "GROUPS")
	siBytes, err := n.persist.Load(bktName, "")
	if err != nil {
		klog.Warning("could not load groups: ", err)
		return nil, err
	}
	for _, v := range siBytes {
		var group Group
		if err = json.Unmarshal(v, &group); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}

	return groups, err
}

func GetGroupLimit(n *NetEngine) (map[string]BWLimit, error) {
	allGroupLimit := make(map[string]BWLimit, 0)
	bktName := fmt.Sprintf("%s.%s", BucketName, "GROUPLIMIT")
	siBytes, err := n.persist.Load(bktName, "")
	if err != nil {
		klog.Warning("could not load group limit: ", err)
		return nil, err
	}

	for k, v := range siBytes {
		var limit BWLimit
		if err = json.Unmarshal(v, &limit); err != nil {
			return nil, err
		}
		allGroupLimit[k] = limit
	}

	return allGroupLimit, err
}

func GetGroupSettable(n *NetEngine) (string, error) {
	bktName := BucketName
	var gSet string
	siBytes, err := n.persist.Load(bktName, "GROUPSETTABLE")
	if err != nil {
		klog.Warning("could not load group settable: ", err)
		return "", err
	}

	if err = json.Unmarshal(siBytes["GROUPSETTABLE"], &gSet); err != nil {
		return "", err
	}

	return gSet, err
}

func GetAllAppInfos(n *NetEngine, key string) (map[string]*AppInfo, error) {
	allApps := make(map[string]*AppInfo)
	bktName := fmt.Sprintf("%s.%s", BucketName, "APPS")
	siBytes, err := n.persist.Load(bktName, key)
	if err != nil {
		klog.Warning("could not load app info: ", err)
		return nil, err
	}

	for k, v := range siBytes {
		var app AppInfo
		if err = json.Unmarshal(v, &app); err != nil {
			return nil, err
		}
		allApps[k] = &app
	}

	return allApps, err
}

func SaveInterval(n *NetEngine, config utils.IntervalConfigInfo) error {
	confBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	err = n.persist.Save(BucketName, "INTERVAL", confBytes)
	if err != nil {
		return fmt.Errorf("could not set config log: %v", err)
	}

	return err
}

func LoadInterval(n *NetEngine) (config utils.IntervalConfigInfo, err error) {
	siBytes, err := n.persist.Load(BucketName, "INTERVAL")
	if err := json.Unmarshal(siBytes["INTERVAL"], &config); err != nil {
		return utils.IntervalConfigInfo{}, err
	}

	return config, err
}

func DeleteAppInfo(n *NetEngine, appName string) error {
	bktName := fmt.Sprintf("%s.%s", BucketName, "APPS")
	err := n.persist.Delete(bktName, appName)
	if err != nil {
		return fmt.Errorf("could not delete app: %v", err)
	}

	return err
}
