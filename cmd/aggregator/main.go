/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"sigs.k8s.io/IOIsolation/pkg/aggregator"
)

var (
	leaseLockName      = "ioisolation-aggregator-lease"
	leaseLockNamespace = "ioi-system"
	initialized        = false
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	var cfg aggregator.Config
	flag.BoolVar(&cfg.IsMtls, "mtls", false, "whether to enable mtls.")
	flag.BoolVar(&cfg.Direct, "direct", true, "update nodes' nodeiostatus at once when it receives node's status update(true), or update in each time interval(false)")
	flag.DurationVar(&cfg.UpdateInterval, "updateInterval", 5*time.Second, "how long aggregator update nodeiostatus's status.")
	flag.Parse()
	aggr := &aggregator.IoisolationAggregator{}
	ctx := context.Background()
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	// build k8s config and client
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Error("New Aggregator error:", err)
		os.Exit(1)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("Core clientset init err: ", err)
		os.Exit(1)
	}
	id := os.Getenv("POD_NAME")
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: leaseLockNamespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}
	go func() {
		<-c
		// Stop Aggregator
		if aggr != nil {
			aggr.Stop()
		}
		os.Exit(0)
	}()
	// start the leader election code loop
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   10 * time.Second,
		RenewDeadline:   5 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Infof("current aggregator pod selected to be leader: %s", id)

				// initialize leader instance
				if !initialized {
					aggr, err = aggregator.NewIoisolationAggregator(&cfg, config, client, true)
					if err != nil {
						klog.Error("initialize aggregator error:", err)
						os.Exit(1)
					}
					aggr.Run(ctx, c)
					aggr.UpdateEndpointLabelToPod(true)
					initialized = true
					return
				}
				aggr.UpdateEndpointLabelToPod(true) // for service endpoint discovery
				aggr.SetLeader(true)                // set aggregator flag
			},
			OnStoppedLeading: func() {
				klog.Infof("leader lost: %s", id)
				aggr.UpdateEndpointLabelToPod(false)
				aggr.SetLeader(false)
			},
			OnNewLeader: func(identity string) {
				klog.Infof("new leader elected: %s", identity)
				// initialize non-leader instance
				if identity == id {
					return
				}
				if !initialized {
					aggr, err = aggregator.NewIoisolationAggregator(&cfg, config, client, false)
					if err != nil {
						klog.Error("initialize aggregator error:", err)
						os.Exit(1)
					}
					aggr.Run(ctx, c)
					initialized = true
				}
			},
		},
	})
}
