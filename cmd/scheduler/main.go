/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"os"

	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
	resourceio "sigs.k8s.io/IOIsolation/pkg/scheduler"

	// Ensure scheme package is initialized.
	_ "sigs.k8s.io/IOIsolation/api/config/scheme"
)

func main() {
	// Register custom plugins to the scheduler framework.
	// Later they can consist of scheduler profile(s) and hence
	// used by various kinds of workloads.
	command := app.NewSchedulerCommand(
		app.WithPlugin(resourceio.Name, resourceio.New),
	)

	code := cli.Run(command)
	os.Exit(code)
}
