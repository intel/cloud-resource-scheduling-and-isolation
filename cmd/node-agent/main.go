/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"flag"
	"os"
	"os/signal"

	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	_ "sigs.k8s.io/IOIsolation/pkg/agent/common"
	_ "sigs.k8s.io/IOIsolation/pkg/agent/csi/csi-engine"
	_ "sigs.k8s.io/IOIsolation/pkg/agent/disk"
	_ "sigs.k8s.io/IOIsolation/pkg/agent/engines"
	_ "sigs.k8s.io/IOIsolation/pkg/agent/metrics"
	_ "sigs.k8s.io/IOIsolation/pkg/agent/net"
	_ "sigs.k8s.io/IOIsolation/pkg/agent/rdt"
	//_ "sigs.k8s.io/IOIsolation/pkg/agent/rdtQuantity"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	flag.Parse()
	// rand.Seed(time.Now().UnixNano())
	klog.Info("1. node agent main start.")

	ag := agent.GetAgent()

	if utils.IsIntelPlatform() {
		// Run Agent
		go func() {
			klog.Info("2. node agent run start.")
			err := agent.GetAgent().Run()
			if err != nil {
				klog.Warning(err)
				os.Exit(1)
			}
		}()
	} else {
		klog.Warning("A non-Intel platform has been detected. Please run the service on an Intel platform.")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	klog.Warning("Going to stop agent")
	// Stop Agent
	err := ag.Stop()
	if err != nil {
		klog.Warning(err)
		os.Exit(1)
	}
}
