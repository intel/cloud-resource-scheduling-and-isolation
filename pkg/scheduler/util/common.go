/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientretry "k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	crv1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
)

type ScoreStrategyType string
type ResourceType string

const (
	AdminConfigmapName = "admin-configmap"
	NamespaceWhiteList = "namespaceWhitelist"
	NotReservedErr     = "not reserved in cache"
	// LeastAllocated strategy prioritizes nodes with least allocated resources.
	LeastAllocated ScoreStrategyType = "LeastAllocated"
	// MostAllocated strategy prioritizes nodes with most allocated resources.
	MostAllocated            ScoreStrategyType = "MostAllocated"
	DefaultBinPack           string            = "FirstFit"
	BlockIO                  ResourceType      = "BlockIO"
	NetworkIO                ResourceType      = "NetworkIO"
	RDT                      ResourceType      = "RDT"
	RDTQuantity              ResourceType      = "RDTQuantity"
	All                      ResourceType      = "All"
	float64EqualityThreshold float64           = 1e-9
)

var UpdateBackoff = wait.Backoff{
	Steps:    3,
	Duration: 100 * time.Millisecond, // 0.1s
	Jitter:   1.0,
}

type DeviceDiff struct {
	Adds map[string]crv1.IOAllocatableBandwidth
	Dels map[string]crv1.IOAllocatableBandwidth
}

func GetNamespaceWhiteList(client kubernetes.Interface) []string {
	list := []string{utils.IoiNamespace, utils.KubesystemNamespace}
	cm, err := client.CoreV1().ConfigMaps(utils.IoiNamespace).Get(context.Background(), AdminConfigmapName, metav1.GetOptions{})
	if err != nil {
		klog.Warningf("failed to get admin-configmap, err: %v", err)
		return list
	}
	nsWhiteListStr, ok := cm.Data[NamespaceWhiteList]
	if !ok {
		klog.Warningf("namespaceWhitelist not found in admin-configmap")
		return list
	}
	additionalList := strings.Fields(nsWhiteListStr)
	list = append(list, additionalList...)
	return list
}

func RemovePVCsFromVolDiskCm(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim, nodename string) error {
	cm, err := GetVolumeDeviceMap(client, nodename)
	if kubeerr.IsNotFound(err) {
		return fmt.Errorf("configMap for volume device binding does not exists on node %v", nodename)
	} else if err != nil {
		return fmt.Errorf("failed to get ConfigMap volume-device-map: nodename %s, err: %v ", nodename, err)
	}
	if len(cm.Data) > 0 {
		cm = cm.DeepCopy()
		keysToRemove := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			if k == pvc.Name+"."+pvc.Namespace {
				keysToRemove = append(keysToRemove, k)
			}
		}

		if len(keysToRemove) > 0 {
			for _, k := range keysToRemove {
				delete(cm.Data, k)
			}
		}
		configmap, err := client.CoreV1().ConfigMaps(utils.IoiNamespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
		if err != nil {
			klog.ErrorS(err, "failed to update volume/disk mapping info for pvc", "pvc", pvc.Name)
			return err
		}
		klog.V(utils.DBG).InfoS("ConfigMap volume-device-map has been updated", "node", nodename, "data", configmap.Data)
	}
	return nil

}

func RemoveNodeFromVolDiskCm(client kubernetes.Interface, nodename string) error {
	cm, err := GetVolumeDeviceMap(client, nodename)
	if kubeerr.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get ConfigMap volume-device-map: nodename %s, err: %v ", nodename, err)
	}
	// remove all pods on the node
	cm.Data = nil
	configmap, err := client.CoreV1().ConfigMaps(utils.IoiNamespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		klog.ErrorS(err, "failed to delete volume/disk mapping info for node", "node", nodename)
		return err
	}
	klog.V(utils.DBG).InfoS("ConfigMap volume-device-map has been updated", "node", nodename, "data", configmap.Data)
	return nil

}

func GetVolumeDeviceMap(client kubernetes.Interface, nodename string) (*v1.ConfigMap, error) {
	if len(nodename) == 0 {
		return nil, fmt.Errorf("node name cannot be empty")
	}
	if client == nil {
		return nil, fmt.Errorf("kubernetes configmap client cannot be nil")
	}
	cmName := fmt.Sprintf("%s-%s", utils.DeviceIDConfigMap, nodename)
	cm, err := client.CoreV1().ConfigMaps(utils.IoiNamespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	return cm, err
}

func compareFloat(a, b float64) bool {
	return math.Abs(a-b) <= float64EqualityThreshold
}

// 0 -> not under pressure
// 1 -> under pressure
func IsUnderPressure(p float64) bool {
	return !compareFloat(p, 0)
}

func GetNodeIOStatus(client versioned.Interface, n string) (*crv1.NodeIOStatus, error) {
	if client == nil {
		return nil, fmt.Errorf("kubernetes configmap client cannot be nil")
	}
	if len(n) == 0 {
		return nil, fmt.Errorf("node name cannot be empty")
	}
	obj, err := client.IoiV1().NodeIOStatuses(utils.CrNamespace).Get(context.TODO(), n+utils.NodeStatusInfoSuffix, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func CreateNodeIOStatus(client versioned.Interface, spec *crv1.NodeIOStatusSpec) error {
	if spec == nil || len(spec.NodeName) == 0 {
		return fmt.Errorf("spec or node name in spec cannot be null or empty")
	}
	nodeStatusInfo := &crv1.NodeIOStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: utils.APIVersion,
			Kind:       utils.NodeIOStatusCR,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.CrNamespace,
			Name:      spec.NodeName + utils.NodeStatusInfoSuffix,
		},
	}
	nodeStatusInfo.Spec = *spec

	_, err := client.IoiV1().NodeIOStatuses(utils.CrNamespace).Create(context.TODO(), nodeStatusInfo, metav1.CreateOptions{})
	if err != nil {
		klog.Error("CreateNodeIOStatus fails: ", err)
		return err
	}
	return nil
}

func UpdateNodeIOStatusSpec(client versioned.Interface, name string, obj *crv1.NodeIOStatusSpec) error {
	return clientretry.RetryOnConflict(UpdateBackoff, func() error {
		var err error
		var sts *crv1.NodeIOStatus

		sts, err = GetNodeIOStatus(client, name)
		if err != nil {
			return err
		}

		// Make a copy of the pod and update the annotations.
		newSts := sts.DeepCopy()
		// newSts.Spec will not be nil
		newSts.Spec = *obj

		oldData, err := json.Marshal(sts)
		if err != nil {
			return fmt.Errorf("failed to marshal the existing NodeIOStatus %#v: %v", sts, err)
		}
		newData, err := json.Marshal(newSts)
		if err != nil {
			return fmt.Errorf("failed to marshal the new NodeIOStatus %#v: %v", newSts, err)
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &crv1.NodeIOStatus{})
		if err != nil {
			return fmt.Errorf("failed to create a two-way merge patch: %v", err)
		}
		if _, err := client.IoiV1().NodeIOStatuses(utils.CrNamespace).Patch(context.TODO(), name+utils.NodeStatusInfoSuffix, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
			return fmt.Errorf("failed to patch the NodeIOStatus: %v", err)
		}
		return nil
	})
}

func UpdateNodeIOStatusInType(client versioned.Interface, name string, toUpdate map[crv1.WorkloadIOType]crv1.DeviceAllocatableBandwidth) error {
	return clientretry.RetryOnConflict(UpdateBackoff, func() error {
		var err error
		var sts *crv1.NodeIOStatus

		sts, err = GetNodeIOStatus(client, name)
		if err != nil {
			return err
		}

		// Make a copy of the pod and update the annotations.
		newSts := sts.DeepCopy()
		// newSts.Status will not be nil
		if newSts.Status.AllocatableBandwidth == nil {
			newSts.Status.AllocatableBandwidth = make(map[crv1.WorkloadIOType]crv1.DeviceAllocatableBandwidth)
		}
		for key, val := range toUpdate {
			newSts.Status.AllocatableBandwidth[key] = val
		}
		if _, err := client.IoiV1().NodeIOStatuses(utils.CrNamespace).UpdateStatus(context.TODO(), newSts, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to patch the NodeIOStatus: %v", err)
		}
		return nil
	})
}

func UpdateNodeIOStatusInDevice(client versioned.Interface, name string, t crv1.WorkloadIOType, adds, dels map[string]crv1.IOAllocatableBandwidth) error {
	return clientretry.RetryOnConflict(UpdateBackoff, func() error {
		var err error
		var sts *crv1.NodeIOStatus

		sts, err = GetNodeIOStatus(client, name)
		if err != nil {
			return err
		}
		// Make a copy of the pod and update the annotations.
		newSts := sts.DeepCopy()
		// newSts.Status will not be nil
		if newSts.Status.AllocatableBandwidth == nil {
			newSts.Status.AllocatableBandwidth = make(map[crv1.WorkloadIOType]crv1.DeviceAllocatableBandwidth)
		}
		if _, ok := newSts.Status.AllocatableBandwidth[t]; !ok {
			return fmt.Errorf("WorkloadIOType %v not found in NodeIOStatus %v", t, name)
		}
		deviceAllocBandwidth := newSts.Status.AllocatableBandwidth[t]
		if deviceAllocBandwidth.DeviceIOStatus == nil {
			deviceAllocBandwidth.DeviceIOStatus = make(map[string]crv1.IOAllocatableBandwidth)
		}
		for key, val := range adds {
			deviceAllocBandwidth.DeviceIOStatus[key] = val
		}
		for key := range dels {
			delete(deviceAllocBandwidth.DeviceIOStatus, key)
		}
		newSts.Status.AllocatableBandwidth[t] = deviceAllocBandwidth

		if _, err := client.IoiV1().NodeIOStatuses(utils.CrNamespace).UpdateStatus(context.TODO(), newSts, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to patch the NodeIOStatus: %v", err)
		}
		return nil
	})
}

func ComparePodList(pl1 map[string]crv1.PodRequest, pl2 map[string]crv1.PodRequest) bool {
	pl1Keys, pl2Keys := []string{}, []string{}
	for key := range pl1 {
		pl1Keys = append(pl1Keys, key)
	}
	for key := range pl2 {
		pl2Keys = append(pl2Keys, key)
	}
	sort.Strings(pl1Keys)
	sort.Strings(pl2Keys)
	return reflect.DeepEqual(pl1Keys, pl2Keys)
}

func AddOrUpdateAnnoOnPod(ctx context.Context, kubeClient kubernetes.Interface, pod *v1.Pod, AnnoToUpdate map[string]string) error {
	return clientretry.RetryOnConflict(UpdateBackoff, func() error {
		var err error
		var p *v1.Pod

		if len(AnnoToUpdate) == 0 {
			return fmt.Errorf("the annotation is unset")
		}

		if pod == nil || len(pod.Name) == 0 || len(pod.Namespace) == 0 {
			return fmt.Errorf("pod info is unset")
		}

		p, err = kubeClient.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Make a copy of the pod and update the annotations.
		newPod := p.DeepCopy()
		if newPod.Annotations == nil {
			newPod.Annotations = make(map[string]string)
		}
		for key, value := range AnnoToUpdate {
			newPod.Annotations[key] = value
		}

		oldData, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal the existing pod %#v: %v", pod, err)
		}
		newData, err := json.Marshal(newPod)
		if err != nil {
			return fmt.Errorf("failed to marshal the new pod %#v: %v", newPod, err)
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &v1.Pod{})
		if err != nil {
			return fmt.Errorf("failed to create a two-way merge patch: %v", err)
		}
		if _, err := kubeClient.CoreV1().Pods(pod.Namespace).Patch(ctx, pod.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
			return fmt.Errorf("failed to patch the pod: %v", err)
		}
		return nil
	})
}

func DeleteAnnoOnPod(ctx context.Context, kubeClient kubernetes.Interface, pod *v1.Pod, key string) error {
	return clientretry.RetryOnConflict(UpdateBackoff, func() error {
		var err error
		var p *v1.Pod

		if len(key) == 0 {
			return fmt.Errorf("annotation key is unset")
		}

		if pod == nil || len(pod.Name) == 0 || len(pod.Namespace) == 0 {
			return fmt.Errorf("pod info is unset")
		}

		p, err = kubeClient.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Make a copy of the pod and update the annotations.
		newPod := pod.DeepCopy()
		if newPod.Annotations != nil {
			delete(newPod.Annotations, key)
		}

		oldData, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal the existing pod %#v: %v", pod, err)
		}
		newData, err := json.Marshal(newPod)
		if err != nil {
			return fmt.Errorf("failed to marshal the new pod %#v: %v", newPod, err)
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &v1.Pod{})
		if err != nil {
			return fmt.Errorf("failed to create a two-way merge patch: %v", err)
		}
		if _, err := kubeClient.CoreV1().Pods(pod.Namespace).Patch(ctx, pod.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
			return fmt.Errorf("failed to patch the pod: %v", err)
		}
		return nil
	})
}

func PodRequest2String(req *pb.PodRequest) (string, error) {
	podReq := utils.AllocatedIO{}
	if req == nil || req.Request == nil {
		return "", fmt.Errorf("empty request")
	}
	if len(req.Request) == 0 {
		return "", fmt.Errorf("the device requirement is empty")
	}
	for _, r := range req.Request {
		if r.DevRequest == nil {
			continue
		}
		io := utils.DeviceAllocatedIO{
			QoS: r.WorkloadType,
		}
		if r.DevRequest.Request != nil {
			io.Request = utils.Quantity{
				Inbps:  r.DevRequest.Request.Inbps,
				Outbps: r.DevRequest.Request.Outbps,
			}
		}
		if r.DevRequest.Limit != nil {
			io.Limit = utils.Quantity{
				Inbps:  r.DevRequest.Limit.Inbps,
				Outbps: r.DevRequest.Limit.Outbps,
			}
		}

		if r.DevRequest.RawRequest != nil {
			io.RawRequest = utils.Quantity{
				Inbps:   r.DevRequest.RawRequest.Inbps,
				Outbps:  r.DevRequest.RawRequest.Outbps,
				Iniops:  r.DevRequest.RawRequest.Iniops,
				Outiops: r.DevRequest.RawRequest.Outiops,
			}
		}
		if r.DevRequest.RawLimit != nil {
			io.RawLimit = utils.Quantity{
				Inbps:   r.DevRequest.RawLimit.Inbps,
				Outbps:  r.DevRequest.RawLimit.Outbps,
				Iniops:  r.DevRequest.RawLimit.Iniops,
				Outiops: r.DevRequest.RawLimit.Outiops,
			}
		}
		if _, ok := podReq[int64(r.IoType)]; !ok {
			podReq[int64(r.IoType)] = make(map[string]utils.DeviceAllocatedIO)
		}
		podReq[int64(r.IoType)][r.DevRequest.DevName] = io
	}
	byt, err := json.Marshal(podReq)
	if err != nil {
		klog.Error("Invalid AllocatedIO")
		return "", err
	}
	return string(byt), nil
}
