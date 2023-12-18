/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"encoding/json"
	"fmt"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

const (
	BucketName string = "COMMON"
)

func SaveLogLevel(persist IPersist, config utils.LogLevelConfigInfo) error {
	confBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal config json: %v", err)
	}

	err = persist.Save(BucketName, "LOGLEVEL", confBytes)
	if err != nil {
		return fmt.Errorf("could not set config log: %v", err)
	}

	return err
}

func LoadLogLevel(persist IPersist) (config utils.LogLevelConfigInfo, err error) {
	siBytes, err := persist.Load(BucketName, "LOGLEVEL")
	if err := json.Unmarshal(siBytes["LOGLEVEL"], &config); err != nil {
		return utils.LogLevelConfigInfo{}, err
	}

	return config, err
}
