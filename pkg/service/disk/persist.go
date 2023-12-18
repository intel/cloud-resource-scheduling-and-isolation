/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"encoding/json"
	"fmt"

	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

const (
	BucketName string = "DISK"
)

func AddAppInfo(d *DiskEngine, appInfo AppInfo) error {
	appBytes, err := json.Marshal(appInfo)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	bktName := fmt.Sprintf("%s.%s", BucketName, "APPS")
	err = d.persist.Save(bktName, appInfo.AppName, appBytes)
	if err != nil {
		return fmt.Errorf("could not insert app: %v", err)
	}

	return err
}

func GetAllAppInfos(d *DiskEngine, key string) (map[string]*AppInfo, error) {
	allApps := make(map[string]*AppInfo)
	bktName := fmt.Sprintf("%s.%s", BucketName, "APPS")
	siBytes, err := d.persist.Load(bktName, key)
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

func SaveInterval(d *DiskEngine, config utils.IntervalConfigInfo) error {
	confBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	err = d.persist.Save(BucketName, "INTERVAL", confBytes)
	if err != nil {
		return fmt.Errorf("save interval fail: %v", err)
	}

	return err
}

func LoadInterval(d *DiskEngine) (config utils.IntervalConfigInfo, err error) {
	siBytes, err := d.persist.Load(BucketName, "INTERVAL")
	if err := json.Unmarshal(siBytes["INTERVAL"], &config); err != nil {
		return utils.IntervalConfigInfo{}, err
	}

	return config, err
}

func DeleteAppInfo(d *DiskEngine, appName string) error {
	bktName := fmt.Sprintf("%s.%s", BucketName, "APPS")
	err := d.persist.Delete(bktName, appName)
	if err != nil {
		return fmt.Errorf("could not delete app: %v", err)
	}

	return err
}
