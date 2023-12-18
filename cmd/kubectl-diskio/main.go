/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/pkg/tool"
)

type Config struct {
	op   string
	node string
	dev  string
}

const (
	Kubeconfig string = "KUBECONFIG"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	cfg := Config{}
	flag.StringVar(&cfg.op, "op", "", "add/delete/list disk enabled with disk IO isolation in node")
	flag.StringVar(&cfg.node, "node", "", "node to add/delete/list disk")
	flag.StringVar(&cfg.dev, "disk", "", "disk to add/delete disk")
	flag.Parse()
	var path string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		os.Exit(1)
	}
	path = filepath.Join(homeDir, ".kube/config")
	kubeconfigPath, exist := os.LookupEnv(Kubeconfig)
	if exist {
		path = kubeconfigPath
	}
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		fmt.Printf("fail to build kubernetes config: %s", err.Error())
		os.Exit(1)
	}

	// create the clientset
	kubeClient, err := versioned.NewForConfig(config)
	if err != nil {
		fmt.Printf("fail to build versionned clientset: %s", err.Error())
		os.Exit(1)
	}
	// create the clientset
	cClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("fail to build kubernetes clientset: %s", err.Error())
		os.Exit(1)
	}
	switch cfg.op {
	case "list":
		if cfg.node == "" {
			fmt.Println("Error: --node is required for list operation")
			os.Exit(1)
		}
		tool.ListDisk(kubeClient, cfg.node)
	case "add":
		if cfg.node == "" || cfg.dev == "" {
			fmt.Println("Error: both --node and --disk are required for add operation")
			os.Exit(1)
		}
		tool.AddDisk(kubeClient, cfg.node, cfg.dev)
	case "delete":
		if cfg.node == "" || cfg.dev == "" {
			fmt.Println("Error: both --node and --disk are required for delete operation")
			os.Exit(1)
		}
		tool.DeleteDisk(kubeClient, cClient, cfg.node, cfg.dev)
	case "help":
		fmt.Println("Usage: kubectl diskio --op=<operation> --node=<node> --disk=<dev>")
		fmt.Println("Operations: list, get, delete, help")
	default:
		fmt.Println("Error: unknown operation")
		fmt.Println("Usage: kubectl diskio --op=<operation> --node=<node> --disk=<dev>")
		fmt.Println("Operations: list, get, delete, help")
		os.Exit(1)
	}
}
