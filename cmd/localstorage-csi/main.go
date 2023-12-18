/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/IOIsolation/pkg/agent/csi/hostpath"
)

func main() {
	version := ""
	var cfg hostpath.Config
	cfg.VendorVersion = version

	flag.StringVar(&cfg.StateDir, "statedir", "/opt/ioi", "directory for storing state information across driver restarts, volumes and snapshots")
	flag.BoolVar(&cfg.Ephemeral, "ephemeral", false, "publish volumes in ephemeral mode even if kubelet did not ask for it (only needed for Kubernetes 1.15)")
	flag.Int64Var(&cfg.MaxVolumesPerNode, "maxvolumespernode", 0, "limit of volumes per node")
	flag.BoolVar(&cfg.EnableAttach, "enable-attach", false, "Enables RPC_PUBLISH_UNPUBLISH_VOLUME capability.")
	flag.BoolVar(&cfg.CheckVolumeLifecycle, "check-volume-lifecycle", false, "Can be used to turn some violations of the volume lifecycle into warnings instead of failing the incorrect gRPC call. Disabled by default because of https://github.com/kubernetes/kubernetes/issues/101911.")
	flag.Int64Var(&cfg.MaxVolumeSize, "max-volume-size", 1024*1024*1024*1024, "maximum size of volumes in bytes (inclusive)")
	flag.BoolVar(&cfg.EnableTopology, "enable-topology", true, "Enables PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS capability.")
	flag.Int64Var(&cfg.AttachLimit, "attach-limit", 0, "Maximum number of attachable volumes on a node. Zero refers to no limit.")

	showVersion := flag.Bool("version", false, "Show version.")
	cfg.NodeID = os.Getenv("Node_Name")
	cfg.Endpoint = os.Getenv("CSI_ENDPOINT")
	cfg.VendorVersion = os.Getenv("CSI_VERSION")
	cfg.DriverName = os.Getenv("CSI_DRIVERNAME")
	klog.InitFlags(nil)
	flag.Parse()

	if *showVersion {
		baseName := path.Base(os.Args[0])
		fmt.Println(baseName, version)
		os.Exit(1)
	}

	if cfg.Ephemeral {
		fmt.Fprintln(os.Stderr, "Deprecation warning: The ephemeral flag is deprecated and should only be used when deploying on Kubernetes 1.15. It will be removed in the future.")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Failed to get in cluster config: %s", err.Error())
		os.Exit(1)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Failed to init core client: %s", err.Error())
		os.Exit(1)
	}

	driver, err := hostpath.NewHostPathDriver(cfg, client, nil)
	if err != nil {
		fmt.Printf("Failed to initialize driver: %s", err.Error())
		os.Exit(1)
	}

	if err := driver.Run(); err != nil {
		fmt.Printf("Failed to run driver: %s", err.Error())
		os.Exit(1)
	}
}
