/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ioiv1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

func MakeVolumeDeviceMapConfigMap(client kubernetes.Interface, ctx context.Context, nodeName string, m map[string]string) error {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.IoiNamespace,
			Name:      fmt.Sprintf("%s-%s", utils.DeviceIDConfigMap, nodeName),
			Labels:    map[string]string{"app": utils.DeviceIDConfigMap},
		},
		Data: m,
	}
	_, err := client.CoreV1().ConfigMaps(utils.IoiNamespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap volume-device-map: node=%v, err=%v", nodeName, err)
	}
	return nil
}

func MakeStorageClass(client kubernetes.Interface, ctx context.Context, name, provisionner string) error {
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			// Namespace: "default",
			Name: name,
		},
		Provisioner: provisionner,
	}
	_, err := client.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create storage class: %v, err=%v", name, err)
	}
	return nil
}

func MakePVC(client kubernetes.Interface, ctx context.Context, claim *v1.PersistentVolumeClaim) error {

	_, err := client.CoreV1().PersistentVolumeClaims("default").Create(context.TODO(), claim, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pvc: %v, err=%v", claim.Name, err)
	}
	return nil
}

// ExpectError calls t.Fatalf if the error does not contain a substr match.
// If substr is empty, a nil error is expected.
// It is useful to call ExpectError from subtests.
func ExpectError(t *testing.T, err error, substr string) {
	if err != nil {
		if len(substr) == 0 {
			t.Fatalf("expect nil error but got %q", err.Error())
		} else if !strings.Contains(err.Error(), substr) {
			t.Fatalf("expect error to contain %q but got %q", substr, err.Error())
		}
	} else if len(substr) > 0 {
		t.Fatalf("expect error to contain %q but got nil error", substr)
	}
}

func MakeNode(cs kubernetes.Interface, node *v1.Node) (*v1.Node, error) {
	return cs.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
}

func MakePod(cs kubernetes.Interface, pod *v1.Pod) (*v1.Pod, error) {
	return cs.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
}

func MakeNodes(cs kubernetes.Interface, prefix string, numNodes int) ([]*v1.Node, error) {
	nodes := make([]*v1.Node, numNodes)
	for i := 0; i < numNodes; i++ {
		nodeName := fmt.Sprintf("%v-%d", prefix, i)
		n := &v1.Node{}
		n.SetName(nodeName)
		node, err := MakeNode(cs, n)
		if err != nil {
			return nodes[:], err
		}
		nodes[i] = node
	}
	return nodes[:], nil
}

func MakeNodeIOStaticInfoWithStatus(client versioned.Interface, spec *ioiv1.NodeStaticIOInfoSpec, sts *ioiv1.NodeStaticIOInfoStatus) error {
	if spec == nil || len(spec.NodeName) == 0 {
		return fmt.Errorf("spec or node name in spec cannot be null or empty")
	}
	cr := &ioiv1.NodeStaticIOInfo{
		TypeMeta: metav1.TypeMeta{
			APIVersion: utils.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.CrNamespace,
			Name:      spec.NodeName + utils.NodeStaticInfoSuffix,
		},
	}
	cr.Spec = *spec
	cr.Status = *sts
	_, err := client.IoiV1().NodeStaticIOInfos(utils.CrNamespace).Create(context.TODO(), cr, metav1.CreateOptions{})
	if err != nil {
		klog.Error("CreateNodeIOStatus fails: ", err)
		return err
	}
	_, err = client.IoiV1().NodeStaticIOInfos(utils.CrNamespace).UpdateStatus(context.TODO(), cr, metav1.UpdateOptions{})
	if err != nil {
		klog.Error("UpdateNodeIOStatus fails: ", err)
		return err
	}
	return nil
}

func MakeNodeIOStatus(client versioned.Interface, spec *ioiv1.NodeIOStatusSpec, sts *ioiv1.NodeIOStatusStatus) error {
	if spec == nil || len(spec.NodeName) == 0 {
		return fmt.Errorf("spec or node name in spec cannot be null or empty")
	}
	nodeStatusInfo := &ioiv1.NodeIOStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: utils.APIVersion,
			// Kind:       utils.NodeIOStatusCR,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.CrNamespace,
			Name:      spec.NodeName + utils.NodeStatusInfoSuffix,
		},
	}
	nodeStatusInfo.Spec = *spec
	nodeStatusInfo.Status = *sts

	_, err := client.IoiV1().NodeIOStatuses(utils.CrNamespace).Create(context.TODO(), nodeStatusInfo, metav1.CreateOptions{})
	if err != nil {
		klog.Error("CreateNodeIOStatus fails: ", err)
		return err
	}
	return nil
}
