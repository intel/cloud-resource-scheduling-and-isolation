/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package resource

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-helpers/storage/volume"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	externalinformer "sigs.k8s.io/IOIsolation/generated/ioi/informers/externalversions"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	common "sigs.k8s.io/IOIsolation/pkg/scheduler/util"
)

type IOEventHandler struct {
	cache      map[string]Handle // resourceType -> handle
	coreClient kubernetes.Interface
	vClient    versioned.Interface // versioned client for CRDs
	f          informers.SharedInformerFactory
}

func NewIOEventHandler(cache map[string]Handle, h framework.Handle) *IOEventHandler {
	return &IOEventHandler{
		cache:      cache,
		coreClient: h.ClientSet(),
		vClient:    IoiContext.VClient,
		f:          h.SharedInformerFactory(),
	}
}

func (eh *IOEventHandler) AddNodeStaticIOInfo(obj interface{}) {
	nio, ok := obj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[AddNodeStaticIOInfo]cannot convert oldObj to v1.NodeStaticIOInfo: %v", obj)
		return
	}
	node := nio.Spec.NodeName
	IoiContext.Lock()
	defer IoiContext.Unlock()
	// set initial generation
	if _, err := IoiContext.GetReservedPods(node); err != nil {
		IoiContext.SetReservedPods(node, &pb.PodList{
			Generation: 0,
			PodList:    make(map[string]*pb.PodRequest),
		})
	}
	// fill cache
	for k, rc := range nio.Spec.ResourceConfig {
		if _, ok := eh.cache[k]; ok {
			err := eh.cache[k].(CacheHandle).AddCacheNodeInfo(node, rc)
			if err != nil {
				klog.Errorf("AddCacheNodeInfo fail: %v", node)
			}
		} else {
			klog.Errorf("resource %v cache not registed", k)
		}
	}
	// fill reserved pod
	namespaces, err := eh.coreClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("get namespaces error: %v", err)
	}
	for _, ns := range namespaces.Items {
		if IoiContext.InNamespaceWhiteList(ns.Name) {
			continue
		}
		pods, err := eh.coreClient.CoreV1().Pods(ns.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			klog.Errorf("get pods error: %v", err)
		}
		for _, pod := range pods.Items {
			// filter pod with allocated-io annotation
			if _, ok := pod.Annotations[utils.AllocatedIOAnno]; !ok {
				continue
			}
			if pod.Spec.NodeName == node {
				requests := eh.calculatePodRequest(&pod, node)
				IoiContext.Reservedpod[node].PodList[string(pod.UID)] = requests
				klog.V(utils.DBG).Info("ReservedPod: pod:", pod.Name, "  req:", requests.String())
			}
		}
	}
	// sync reserved pod
	spec, err := IoiContext.GetReservedPodsWithNameNS(node)
	if err != nil {
		// should not happen
		klog.Error("Internal err", err)
	}
	sts, serr := common.GetNodeIOStatus(eh.vClient, node)
	// CR not exist, create status CR and init node status
	if serr != nil && errors.IsNotFound(serr) {
		err = common.CreateNodeIOStatus(eh.vClient, spec)
		if err != nil {
			klog.Error("Create NodeIOStatus error:", err)
			return
		}
		// update NodeIOStatusStatus
		if err = common.UpdateNodeIOStatusInType(eh.vClient, nio.Spec.NodeName, composeAllocableBW(nio, "")); err != nil {
			klog.Errorf("[AddNodeStaticIOInfo]UpdateNodeIOStatusInType fails due to err: %v", err)
			return
		}
		for k, rc := range nio.Spec.ResourceConfig {
			if _, ok := eh.cache[k]; ok {
				eh.cache[k].(CacheHandle).InitNodeStatus(nio.Spec.NodeName, rc)
			} else {
				klog.Errorf("resource %v cache not registed", k)
			}
		}
		klog.V(utils.INF).Infof("[AddNodeStaticIOInfo] node %v is handled", node)
		return
	} else if serr != nil {
		klog.Error("Internal err:", serr)
		return
	}
	// CR exist but the status is outdated.
	if !common.ComparePodList(sts.Spec.ReservedPods, spec.ReservedPods) ||
		sts.Status.ObservedGeneration != sts.Spec.Generation {
		// fill initial node status
		for k, rc := range nio.Spec.ResourceConfig {
			if _, ok := eh.cache[k]; ok {
				eh.cache[k].(CacheHandle).InitNodeStatus(nio.Spec.NodeName, rc)
			} else {
				klog.Errorf("resource %v cache not registed", k)
			}
		}
		// update CR status and generation + 1
		copy := sts.DeepCopy()
		copy.Spec.Generation += 1
		copy.Spec.ReservedPods = spec.ReservedPods
		if err = IoiContext.SetPodListGen(node, copy.Spec.Generation); err != nil {
			klog.Error("Internal error:", err)
		}
		if err = common.UpdateNodeIOStatusSpec(eh.vClient, node, &copy.Spec); err != nil {
			klog.Error("UpdateNodeIOStatusGeneration error:", err)
		}
		//	CR exists and pod list equals
	} else {
		if err = IoiContext.SetPodListGen(node, sts.Spec.Generation); err != nil {
			klog.Error("Internal error:", err)
		}

		// fill initial node status
		for k, rc := range nio.Spec.ResourceConfig {
			if _, ok := eh.cache[k]; ok {
				eh.cache[k].(CacheHandle).InitNodeStatus(nio.Spec.NodeName, rc)
			} else {
				klog.Errorf("resource %v cache not registed", k)
			}
		}
	}
	klog.V(utils.INF).Infof("[AddNodeStaticIOInfo] node %v is handled", node)
}

func composeAllocableBW(ni *v1.NodeStaticIOInfo, typ string) map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth {
	bw := map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth{}
	if ni == nil || ni.Spec.ResourceConfig == nil {
		return nil
	}
	for k, rc := range ni.Spec.ResourceConfig {
		if len(typ) > 0 && k != typ {
			continue
		}
		dev := map[string]v1.IOAllocatableBandwidth{}
		for n := range rc.Devices {
			dev[n] = v1.IOAllocatableBandwidth{
				IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
					v1.WorkloadGA: {
						Total:    -1,
						In:       -1,
						Out:      -1,
						Pressure: -1,
					},
					v1.WorkloadBE: {
						Total:    -1,
						In:       -1,
						Out:      -1,
						Pressure: -1,
					},
				},
			}
		}
		bw[v1.WorkloadIOType(k)] = v1.DeviceAllocatableBandwidth{
			DeviceIOStatus: dev,
		}
		if len(typ) > 0 {
			break
		}
	}
	return bw
}

func composeDeviceAllocableBW(adds, dels map[string]v1.Device) (map[string]v1.IOAllocatableBandwidth, map[string]v1.IOAllocatableBandwidth) {
	cAdds := make(map[string]v1.IOAllocatableBandwidth)
	cDels := make(map[string]v1.IOAllocatableBandwidth)
	for k := range adds {
		cAdds[k] = v1.IOAllocatableBandwidth{
			IOPoolStatus: map[v1.WorkloadQoSType]v1.IOStatus{
				v1.WorkloadGA: {
					Total:    -1,
					In:       -1,
					Out:      -1,
					Pressure: -1,
				},
				v1.WorkloadBE: {
					Total:    -1,
					In:       -1,
					Out:      -1,
					Pressure: -1,
				},
			},
		}
	}
	for k := range dels {
		cDels[k] = v1.IOAllocatableBandwidth{}
	}
	return cAdds, cDels
}

func (eh *IOEventHandler) calculatePodRequest(pod *corev1.Pod, node string) *pb.PodRequest {
	l := &pb.PodRequest{
		PodName:      pod.Name,
		PodNamespace: pod.Namespace,
		Request:      []*pb.IOResourceRequest{},
	}
	for k := range eh.cache {
		// pvc or emptyDir, get pvc's device from configmap
		requests, _, err := eh.cache[k].(CacheHandle).GetRequestByAnnotation(pod, node, true)
		if err != nil {
			klog.Errorf("Get existing pod io resource error:", err)
		}
		for _, req := range requests {
			l.Request = append(l.Request, req)
		}

	}
	return l
}

func (eh *IOEventHandler) UpdateNodeStaticIOInfo(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[UpdateNodeStaticIOInfo]cannot convert oldObj to v1.NodeStaticIOInfo: %v", oldObj)
		return
	}
	nio, ok := newObj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[UpdateNodeStaticIOInfo]cannot convert newObj to v1.NodeStaticIOInfo: %v", newObj)
		return
	}
	for k, n := range nio.Spec.ResourceConfig {
		if _, ok := eh.cache[k]; ok { //resource handle exists
			o, ok := old.Spec.ResourceConfig[k]
			if ok { // old spec has the resource too
				if utils.HashObject(n) != utils.HashObject(o) && k == string(v1.BlockIO) { //currently only BlockIO support dynamic change device
					adds, dels, err := eh.cache[k].(CacheHandle).UpdateDeviceNodeInfo(nio.Spec.NodeName, &o, &n)
					if err != nil {
						klog.Errorf("UpdateDeviceNodeInfo fails: %v", err)
						return
					}
					cAdds, cDels := composeDeviceAllocableBW(adds, dels)
					if err := common.UpdateNodeIOStatusInDevice(eh.vClient, nio.Spec.NodeName, v1.WorkloadIOType(k), cAdds, cDels); err != nil {
						klog.Errorf("[UpdateNodeStaticIOInfo]PatchNodeIOStatusStatus fails due to err: %v", err)
						return
					}
				}
			} else { // old spec does not have the resource but new spec has
				err := eh.cache[k].(CacheHandle).AddCacheNodeInfo(nio.Spec.NodeName, n)
				if err != nil {
					klog.Errorf("AddCacheNodeInfo fails: %v", err)
					return
				}
				eh.cache[k].(CacheHandle).InitNodeStatus(nio.Spec.NodeName, n)
				// patch NodeIOStatusStatus
				if err = common.UpdateNodeIOStatusInType(eh.vClient, nio.Spec.NodeName, composeAllocableBW(nio, k)); err != nil {
					klog.Errorf("[UpdateNodeStaticIOInfo]PatchNodeIOStatusStatus fails due to err: %v", err)
					return
				}
			}
		}
	}
	klog.V(utils.INF).Infof("[UpdateNodeStaticIOInfo] node %v is handled", old.Spec.NodeName)
}

func (eh *IOEventHandler) DeleteNodeStaticIOInfo(obj interface{}) {
	nio, ok := obj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[DeleteNodeNetworkIOInfo]cannot convert to v1.NodeStaticIOInfo: %v", obj)
		return
	}
	for _, h := range eh.cache {
		err := h.(CacheHandle).DeleteCacheNodeInfo(nio.Spec.NodeName)
		if err != nil {
			klog.Errorf("[DeleteNodeNetworkIOInfo]cannot convert to v1.NodeStaticIOInfo: %v", err.Error())
		}
		h.(CacheHandle).PrintCacheInfo()
	}
	IoiContext.Lock()
	defer IoiContext.Unlock()
	if _, err := IoiContext.GetReservedPods(nio.Spec.NodeName); err == nil {
		IoiContext.RemoveNode(nio.Spec.NodeName)
	}
	klog.V(utils.INF).Infof("[DeleteNodeNetworkIOInfo] node %v is handled", nio.Spec.NodeName)
}

func (eh *IOEventHandler) DeletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("[DeletePod]cannot convert to *v1.Pod: %v", obj)
		return
	}
	// ignore pod in not scheduled by ioi plugin
	if _, ok := pod.Annotations[utils.AllocatedIOAnno]; !ok {
		return
	}
	for _, h := range eh.cache {
		err := h.(CacheHandle).RemovePod(pod, pod.Spec.NodeName, nil) // client nil means do not clear pvc mount info
		if err != nil {
			klog.Error("Remove pod err: ", err)
		}
		klog.V(utils.DBG).Info("After delete pod")
		h.(CacheHandle).PrintCacheInfo()
	}
	// update reserved pod
	err := IoiContext.RemovePod(context.TODO(), pod, pod.Spec.NodeName)
	if err != nil {
		klog.Errorf("fail to remove pod in ReservedPod: %v", err)
	}
	klog.V(utils.INF).Infof("[DeletePod] pod %v.%v is handled", pod.Name, pod.Namespace)
}

func (eh *IOEventHandler) UpdateNode(oldObj, newObj interface{}) {
	oldNode, ok := oldObj.(*corev1.Node)
	if !ok {
		klog.Errorf("[UpdateNode]cannot convert to *v1.Node: %v", oldObj)
		return
	}
	newNode, ok := newObj.(*corev1.Node)
	if !ok {
		klog.Errorf("[UpdateNode]cannot convert to *v1.Node: %v", newObj)
		return
	}
	for k := range eh.cache {
		if eh.cache[k].(CacheHandle).NodeHasIoiLabel(oldNode) && !eh.cache[k].(CacheHandle).NodeHasIoiLabel(newNode) {
			klog.V(utils.DBG).Info("Label deleted in node ", newNode.Name)
			err := eh.cache[k].(CacheHandle).DeleteCacheNodeInfo(newNode.Name)
			if err != nil {
				klog.Errorf("DeleteCacheNodeInfo for node %v fails: %v", newNode.Name, err)
				return
			}
		}
	}
	klog.V(utils.INF).Infof("[UpdateNode]node %s is handled", newNode.Name)
}

func (eh *IOEventHandler) DeleteNode(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("[DeleteNode]cannot convert to *v1.Node: %v", obj)
		return
	}
	for k := range eh.cache {
		err := eh.cache[k].(CacheHandle).DeleteCacheNodeInfo(node.Name)
		if err != nil {
			klog.Errorf("DeleteCacheNodeInfo for node %v fails: %v", node.Name, err)
			return
		}
	}
	IoiContext.Lock()
	defer IoiContext.Unlock()
	if _, err := IoiContext.GetReservedPods(node.Name); err == nil {
		IoiContext.RemoveNode(node.Name)
	}
	// remove pod from volume-device-map
	err := common.RemoveNodeFromVolDiskCm(eh.coreClient, node.Name)
	if err != nil {
		klog.Error("Remove all pods from volume-device-map err: ", err)
	}
	klog.V(utils.INF).Infof("[DeleteNode]node %s is handled", node.Name)
}

func (eh *IOEventHandler) DeletePVC(obj interface{}) {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		klog.Errorf("[DeletePVC]cannot convert to *v1.PersistentVolumeClaim: %v", obj)
		return
	}
	// revert storage in cache
	IoiContext.Lock()
	defer IoiContext.Unlock()
	key := fmt.Sprintf("%s.%s", pvc.Name, pvc.Namespace)
	info, err := IoiContext.GetStorageInfo(key)
	if err != nil {
		klog.V(utils.INF).Infof("pvc %s was not bound to any disk", key)
		return
	}
	for k := range eh.cache {
		err := eh.cache[k].(CacheHandle).UpdatePVC(pvc, info.NodeName, info.DevID, info.RequestedStorage, false)
		if err != nil {
			klog.Errorf("update PVC error: %v", err)
		}
	}
	klog.V(utils.INF).Infof("[DeletePVC] PVC %v.%v is handled", pvc.Name, pvc.Namespace)
}

func (eh *IOEventHandler) getBoundDeviceId(pvc *corev1.PersistentVolumeClaim, node string) (string, int64, error) {
	if pvc == nil {
		return "", -1, fmt.Errorf("pvc cannot be empty")
	}
	key := fmt.Sprintf("%s.%s", pvc.Name, pvc.Namespace)
	IoiContext.Lock()
	defer IoiContext.Unlock()
	_, err := IoiContext.GetStorageInfo(key)
	if err != nil {
		return "", -1, fmt.Errorf("PVC %v was not found in IoIContext", key)
	}
	quantity, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if !ok {
		// No capacity to check for. Should not happen.
		return "", -1, fmt.Errorf("the PVC storage request is not specified")
	}
	cm, err := common.GetVolumeDeviceMap(eh.coreClient, node)
	if err != nil {
		return "", quantity.Value(), fmt.Errorf("unable to find the volume device map for node %s", node)
	}
	deviceId := ""
	for k, v := range cm.Data {
		if k == key {
			strs := strings.Split(v, ":")
			if len(strs) == 2 {
				deviceId = strs[0]
			}
			break
		}
	}
	return deviceId, quantity.Value(), nil
}

func (eh *IOEventHandler) UpdatePVC(oldObj, newObj interface{}) {
	oldPVC, ok := oldObj.(*corev1.PersistentVolumeClaim)
	if !ok {
		klog.Errorf("[UpdatePVC]cannot convert to *v1.PersistentVolumeClaim: %v", oldObj)
		return
	}
	newPVC, ok := newObj.(*corev1.PersistentVolumeClaim)
	if !ok {
		klog.Errorf("[UpdatePVC]cannot convert to *v1.PersistentVolumeClaim: %v", newObj)
		return
	}
	node, ok := newPVC.Annotations[volume.AnnSelectedNode]
	provisioner, ok1 := newPVC.Annotations[volume.AnnStorageProvisioner]
	if !ok || !ok1 || provisioner != utils.StorageProvisioner {
		return
	}

	if oldPVC.Status.Phase != corev1.ClaimBound && newPVC.Status.Phase == corev1.ClaimBound {
		deviceId, qty, err := eh.getBoundDeviceId(newPVC, node)
		if err != nil {
			klog.Error(err)
			return
		}
		if len(deviceId) > 0 && qty >= 0 {
			for k := range eh.cache {
				err := eh.cache[k].(CacheHandle).UpdatePVC(newPVC, node, deviceId, qty, true)
				if err != nil {
					klog.Errorf("update PVC error: %v", err)
				}
			}
		}
	} else if oldPVC.Status.Phase == corev1.ClaimBound && newPVC.Status.Phase == corev1.ClaimLost {
		deviceId, qty, err := eh.getBoundDeviceId(newPVC, node)
		if err != nil {
			klog.Error(err)
			return
		}
		if len(deviceId) > 0 && qty >= 0 {
			for k := range eh.cache {
				err := eh.cache[k].(CacheHandle).UpdatePVC(newPVC, node, deviceId, qty, false)
				if err != nil {
					klog.Errorf("update PVC error: %v", err)
				}
			}
		}
	}
	klog.V(utils.INF).Infof("[UpdatePVC] PVC %v.%v is handled", newPVC.Name, newPVC.Namespace)
}

func (eh *IOEventHandler) UpdateNodeIOStatus(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1.NodeIOStatus)
	if !ok {
		klog.Errorf("[UpdateNodeIOStatus]cannot convert oldObj to v1.NodeIOStatus: %v", oldObj)
		return
	}
	nis, ok := newObj.(*v1.NodeIOStatus)
	if !ok {
		klog.Errorf("[UpdateNodeIOStatus]cannot convert newObj to v1.NodeIOStatus: %v", newObj)
		return
	}
	IoiContext.Lock()
	defer IoiContext.Unlock()
	rp, err := IoiContext.GetReservedPods(nis.Spec.NodeName)
	if err != nil {
		klog.Errorf("[UpdateNodeIOStatus]failed to get reserved pods for node %v", nis.Spec.NodeName)
		return
	}
	if utils.HashObject(old.Status) != utils.HashObject(nis.Status) && len(old.Status.AllocatableBandwidth) == len(nis.Status.AllocatableBandwidth) &&
		nis.Status.ObservedGeneration == rp.Generation {
		// ignore status update if generation does not match
		for k := range eh.cache {
			err := eh.cache[k].(CacheHandle).UpdateCacheNodeStatus(nis.Spec.NodeName, &nis.Status)
			if err != nil {
				klog.Error("UpdateCacheNodeStatus error: ", err.Error())
			}
		}
	}
}

func (eh *IOEventHandler) AddAdminConfigData(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		klog.Errorf("[AddAdminConfigData]cannot convert oldObj to v1.ConfigMap: %v", obj)
		return
	}
	info, debug := false, false
	loglevelData := cm.Data["loglevel"]
	lines := strings.Split(string(loglevelData), "\n")
	for _, line := range lines {
		fields := strings.Split(line, "=")
		if len(fields) == 2 && strings.Contains(fields[0], "level") {
			if fields[1] == "info" {
				info = true
			} else if fields[1] == "debug" {
				info = true
				debug = true
			}
		}
	}
	utils.SetLogLevel(info, debug)
	nsWhiteListStr, ok := cm.Data[common.NamespaceWhiteList]
	if ok {
		list := strings.Fields(nsWhiteListStr)
		list = append(list, utils.KubesystemNamespace)
		list = append(list, utils.IoiNamespace)
		IoiContext.NsWhiteList = list
	}
	// klog.Info("namespace whitelist=", IoiContext.NsWhiteList)
}

func (eh *IOEventHandler) UpdateAdminConfigData(oldObj, newObj interface{}) {
	cm, ok := newObj.(*corev1.ConfigMap)
	if !ok {
		klog.Errorf("[AddAdminConfigData]cannot convert oldObj to v1.ConfigMap: %v", newObj)
		return
	}
	info, debug := false, false
	loglevelData := cm.Data["loglevel"]
	lines := strings.Split(string(loglevelData), "\n")
	for _, line := range lines {
		fields := strings.Split(line, "=")
		if len(fields) == 2 && strings.Contains(fields[0], "level") {
			if fields[1] == "info" {
				info = true
			} else if fields[1] == "debug" {
				info = true
				debug = true
			}
		}
	}
	utils.SetLogLevel(info, debug)
	nsWhiteListStr, ok := cm.Data[common.NamespaceWhiteList]
	if ok {
		list := strings.Fields(nsWhiteListStr)
		list = append(list, utils.KubesystemNamespace)
		list = append(list, utils.IoiNamespace)
		IoiContext.NsWhiteList = list
	}
	// klog.Info("namespace whitelist=", IoiContext.NsWhiteList)
}

func (eh *IOEventHandler) BuildEvtHandler(cxt context.Context) error {
	iof := externalinformer.NewSharedInformerFactory(eh.vClient, 0)
	// NodeStaticIOInfo event handler
	sh := cache.ResourceEventHandlerFuncs{
		AddFunc:    eh.AddNodeStaticIOInfo,
		UpdateFunc: eh.UpdateNodeStaticIOInfo,
		DeleteFunc: eh.DeleteNodeStaticIOInfo,
	}
	if _, err := iof.Ioi().V1().NodeStaticIOInfos().Informer().AddEventHandler(sh); err != nil {
		return err
	}

	// NodeIOStatus event handler
	dh := cache.ResourceEventHandlerFuncs{
		UpdateFunc: eh.UpdateNodeIOStatus,
	}
	if _, err := iof.Ioi().V1().NodeIOStatuses().Informer().AddEventHandler(dh); err != nil {
		return err
	}

	iof.Start(cxt.Done())

	if !cache.WaitForCacheSync(cxt.Done(), iof.Ioi().V1().NodeStaticIOInfos().Informer().HasSynced) {
		return fmt.Errorf("timed out waiting for caches to sync resource NodeStaticIOInfo")
	}
	if !cache.WaitForCacheSync(cxt.Done(), iof.Ioi().V1().NodeIOStatuses().Informer().HasSynced) {
		return fmt.Errorf("timed out waiting for caches to sync resource NodeIOStatus")
	}
	podInformer := eh.f.Core().V1().Pods().Informer()
	fhandler := cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				klog.Errorf("cannot convert to *v1.Pod: %v", obj)
				return false
			}
			if IoiContext.InNamespaceWhiteList(pod.Namespace) {
				return false
			}
			if pod.Spec.NodeName == "" {
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			DeleteFunc: eh.DeletePod,
		},
	}
	if _, err := podInformer.AddEventHandler(fhandler); err != nil {
		return err
	}
	nodeInformer := eh.f.Core().V1().Nodes().Informer()
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: eh.UpdateNode,
		DeleteFunc: eh.DeleteNode,
	}
	if _, err := nodeInformer.AddEventHandler(handler); err != nil {
		return err
	}
	pvcInformer := eh.f.Core().V1().PersistentVolumeClaims().Informer()
	handler = cache.ResourceEventHandlerFuncs{
		UpdateFunc: eh.UpdatePVC,
		DeleteFunc: eh.DeletePVC,
	}
	if _, err := pvcInformer.AddEventHandler(handler); err != nil {
		return err
	}
	cmInformer := eh.f.Core().V1().ConfigMaps().Informer()
	cmhandler := cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			adm, ok := obj.(*corev1.ConfigMap)
			if !ok || adm.Name != utils.AdminName {
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    eh.AddAdminConfigData,
			UpdateFunc: eh.UpdateAdminConfigData,
		},
	}

	if _, err := cmInformer.AddEventHandler(cmhandler); err != nil {
		return err
	}
	return nil
}
