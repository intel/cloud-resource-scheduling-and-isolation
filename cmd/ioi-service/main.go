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
	"sigs.k8s.io/IOIsolation/pkg/service"
	_ "sigs.k8s.io/IOIsolation/pkg/service/disk"
	_ "sigs.k8s.io/IOIsolation/pkg/service/net"
	_ "sigs.k8s.io/IOIsolation/pkg/service/rdt"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	klog.Infof("Service started.")
	flag.Parse()

	as := service.GetService()
	pFlag := utils.IsIntelPlatform()

	if pFlag {
		// Run Service
		err := as.Run()
		if err != nil {
			klog.Warning(err)
			os.Exit(1)
		}
	} else {
		klog.Warning("The service cannot run on a non IA platform, press Ctrl+C to exit.")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	klog.Info("Stop Service")
	if pFlag {
		err := as.Close()
		if err != nil {
			klog.Warning(err)
			os.Exit(1)
		}
	}
}
