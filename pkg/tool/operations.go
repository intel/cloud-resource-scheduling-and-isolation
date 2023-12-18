/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package tool

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientretry "k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

const (
	NodeStaticInfoSuffix string = "-nodestaticioinfo"
	Namespace            string = "ioi-system"
)

var UpdateBackoff = wait.Backoff{
	Steps:    3,
	Duration: 100 * time.Millisecond, // 0.1s
	Jitter:   1.0,
}

func ListDisk(kubeClient versioned.Interface, node string) {
	obj, err := kubeClient.IoiV1().NodeStaticIOInfos(Namespace).Get(context.TODO(), node+NodeStaticInfoSuffix, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get the NodeIOStatus: %v", err)
		os.Exit(1)
	}
	if obj == nil {
		klog.Errorf("NodeIOStatus not found")
		os.Exit(1)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight|tabwriter.Debug)
	defer w.Flush()

	fmt.Fprintln(w, "Device\tStatus\tReason")
	for _, state := range obj.Status.DeviceStates {
		s := fmt.Sprintf("%s\t%v\t%s", state.Name, state.State, state.Reason)
		fmt.Fprintln(w, s)
	}
}

func AddDisk(kubeClient versioned.Interface, node, devName string) {
	err := clientretry.RetryOnConflict(UpdateBackoff, func() error {
		obj, err := kubeClient.IoiV1().NodeStaticIOInfos(Namespace).Get(context.TODO(), node+NodeStaticInfoSuffix, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get the NodeIOStatus: %v", err)
			os.Exit(1)
		}
		dev, ok := devNameToKey(devName, obj)
		if !ok {
			klog.Errorf("device %s not exists in node %v", devName, node)
			os.Exit(1)
		}
		if obj.Status.DeviceStates[dev].State == utils.Profiling || obj.Status.DeviceStates[dev].State == utils.Ready {
			klog.Errorf("device %s is in %v state, cannot add", devName, obj.Status.DeviceStates[dev].State)
			os.Exit(1)
		}
		ds := obj.Status.DeviceStates[dev]
		ds.State = utils.Profiling
		ds.Reason = ""
		obj.Status.DeviceStates[dev] = ds
		_, err = kubeClient.IoiV1().NodeStaticIOInfos(Namespace).UpdateStatus(context.TODO(), obj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		klog.Errorf("failed to patch the NodeIOStatus: %v", err)
		os.Exit(1)
	}
}

func DeleteDisk(kubeClient versioned.Interface, cClient kubernetes.Interface, node, devName string) {
	err := clientretry.RetryOnConflict(UpdateBackoff, func() error {
		obj, err := kubeClient.IoiV1().NodeStaticIOInfos(Namespace).Get(context.TODO(), node+NodeStaticInfoSuffix, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get the NodeIOStatus: %v", err)
			os.Exit(1)
		}
		dev, ok := devNameToKey(devName, obj)
		if !ok {
			klog.Errorf("device %s not exists in node %v", devName, node)
			os.Exit(1)
		}
		if _, ok := obj.Status.DeviceStates[dev]; !ok {
			klog.Errorf("device %s not exists in node %v", dev, node)
			os.Exit(1)
		}
		cm, err := cClient.CoreV1().ConfigMaps(Namespace).Get(context.TODO(), fmt.Sprintf("%s-%s", utils.DeviceIDConfigMap, node), metav1.GetOptions{})
		if err == nil {
			if v, ok := cm.Data[utils.DeviceInUse]; ok {
				devices, err := utils.JsonToStrings(v)
				if err == nil {
					for _, d := range devices {
						if d == dev {
							klog.Errorf("device %s is in use", devName)
							os.Exit(1)
						}
					}
				}
			}
		}
		if obj.Status.DeviceStates[dev].State == utils.Profiling || obj.Status.DeviceStates[dev].State == utils.NotEnabled {
			klog.Errorf("device %s is in %v state, cannot remove", devName, obj.Status.DeviceStates[dev].State)
			os.Exit(1)
		}
		if _, ok := obj.Spec.ResourceConfig[string(v1.BlockIO)]; !ok {
			klog.Errorf("BlockIO not enabled in node %v", node)
			os.Exit(1)
		}
		if _, ok := obj.Spec.ResourceConfig[string(v1.BlockIO)].Devices[dev]; !ok {
			klog.Errorf("device %s not exists in BlockIO resource config", devName)
			os.Exit(1)
		}
		if obj.Spec.ResourceConfig[string(v1.BlockIO)].Devices[dev].Type == v1.DefaultDevice {
			klog.Errorf("device %s is default device, cannot remove", devName)
			os.Exit(1)

		}
		state := obj.Status.DeviceStates[dev]
		state.State = utils.NotEnabled
		obj.Status.DeviceStates[dev] = state
		_, err = kubeClient.IoiV1().NodeStaticIOInfos(Namespace).UpdateStatus(context.TODO(), obj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		klog.Errorf("failed to patch the NodeIOStatus: %v", err)
		os.Exit(1)
	}
}

func devNameToKey(devName string, obj *v1.NodeStaticIOInfo) (string, bool) {
	if obj == nil {
		klog.Errorf("NodeIOStatus not found")
		os.Exit(1)
	}
	for key, state := range obj.Status.DeviceStates {
		if state.Name == devName {
			return key, true
		}
	}
	return "", false
}
