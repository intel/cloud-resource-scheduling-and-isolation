/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package aggregator

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	externalinformer "sigs.k8s.io/IOIsolation/generated/ioi/informers/externalversions"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/api/nodeagent"
)

func (aggr *IoisolationAggregator) WatchNodeInfo() error {
	nodeInfoInformerFactory := externalinformer.NewSharedInformerFactory(aggr.client, 0)
	staticInfoInformer := nodeInfoInformerFactory.Ioi().V1().NodeStaticIOInfos().Informer()
	staticInfoHandler := cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			_, ok := obj.(*v1.NodeStaticIOInfo)
			if !ok {
				klog.Errorf("cannot convert to *v1.NodeDiskIOStatusInfo: %v", obj)
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    aggr.NodeStaticIOInfoCrAdd,
			UpdateFunc: aggr.NodeStaticIOInfoCrUpdate,
			DeleteFunc: aggr.NodeStaticIOInfoCrDelete,
		},
	}
	if _, err := staticInfoInformer.AddEventHandler(staticInfoHandler); err != nil {
		return err
	}

	statusInfoInformer := nodeInfoInformerFactory.Ioi().V1().NodeIOStatuses().Informer()
	statusInfoHandler := cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			_, ok := obj.(*v1.NodeIOStatus)
			if !ok {
				klog.Errorf("cannot convert to *v1.NodeIOStatusInfo: %v", obj)
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    aggr.AddNodeIOStatusCR,
			UpdateFunc: aggr.UpdateNodeIOStatusCR,
		},
	}
	if _, err := statusInfoInformer.AddEventHandler(statusInfoHandler); err != nil {
		return err
	}

	nodeInfoInformerFactory.Start(nil)
	return nil
}
func (aggr *IoisolationAggregator) WatchConfigMap() error {
	corefactory := informers.NewSharedInformerFactory(aggr.coreclient, 0)
	coreInformer := corefactory.Core().V1().ConfigMaps().Informer()

	handler := cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			adm, ok := obj.(*corev1.ConfigMap)
			if !ok || adm.Name != utils.AdminName {
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    aggr.AddAdminConfigData,
			UpdateFunc: aggr.UpdateAdminConfigData,
		},
	}
	if _, err := coreInformer.AddEventHandler(handler); err != nil {
		return err
	}
	corefactory.Start(nil)
	return nil
}

func (aggr *IoisolationAggregator) AddAdminConfigData(obj interface{}) {
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
}

func (aggr *IoisolationAggregator) UpdateAdminConfigData(oldObj, newObj interface{}) {
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
}

func (aggr *IoisolationAggregator) AddNodeIOStatusCR(obj interface{}) {
	nodeStatus, ok := obj.(*v1.NodeIOStatus)
	if !ok {
		klog.Errorf("[AddNodeIOStatus]cannot convert oldObj to *v1.NodeIOStatus: %v", obj)
		return
	}
	node := nodeStatus.Spec.NodeName
	aggr.Lock()
	defer aggr.Unlock()
	if _, ok := aggr.nodeInfo4Agent[node]; !ok {
		aggr.nodeInfo4Agent[node] = &NodeInfo4Agent{
			reservedPod:      make(map[string]v1.PodRequest),
			latestGeneration: nodeStatus.Spec.Generation,
		}
	}
	// meaningful status, regist node into cache
	if len(nodeStatus.Status.AllocatableBandwidth) > 0 {
		aggr.nodeInfo4Sched[node] = &nodeStatus.Status
		aggr.nodeInfo4Sched[node].ObservedGeneration = nodeStatus.Spec.Generation
	}
	aggr.ReservedPodCh <- &nodeStatus.Spec
	klog.V(utils.INF).Infof("[AddNodeIOStatus] node %v is handled", nodeStatus.Spec.NodeName)
}

func (aggr *IoisolationAggregator) UpdateNodeIOStatusCR(oldObj, newObj interface{}) {
	oldStatus, ok := oldObj.(*v1.NodeIOStatus)
	if !ok {
		klog.Errorf("[UpdateNodeIOStatus]cannot convert oldObj to *v1.NodeIOStatus: %v", oldObj)
		return
	}
	newStatus, ok := newObj.(*v1.NodeIOStatus)
	if !ok {
		klog.Errorf("[UpdateNodeIOStatus]cannot convert newObj to *v1.NodeIOStatus: %v", newObj)
		return
	}
	// podList has update
	if !reflect.DeepEqual(oldStatus.Spec, newStatus.Spec) {
		aggr.ReservedPodCh <- &newStatus.Spec
	}
	// status has updates
	node := newStatus.Spec.NodeName
	_, ok = aggr.nodeInfo4Sched[node]
	// first time update status
	if !ok && len(oldStatus.Status.AllocatableBandwidth) == 0 && len(newStatus.Status.AllocatableBandwidth) > 0 {
		// scheduler node initialize, regist node into cache
		aggr.nodeInfo4Sched[node] = &newStatus.Status
		aggr.nodeInfo4Sched[node].ObservedGeneration = newStatus.Spec.Generation
	}
	// add new resources type
	if len(oldStatus.Status.AllocatableBandwidth) > 0 && len(oldStatus.Status.AllocatableBandwidth) < len(newStatus.Status.AllocatableBandwidth) {
		// new resource type registed
		if _, ok := aggr.nodeInfo4Sched[node]; !ok {
			klog.Errorf("node %v not registed in cache", node)
		} else {
			aggr.addResourcetoCache(node, oldStatus.Status.AllocatableBandwidth, newStatus.Status.AllocatableBandwidth)
		}
	}
	// update device info
	if len(oldStatus.Status.AllocatableBandwidth) == len(newStatus.Status.AllocatableBandwidth) {
		for iotype, ninfo := range newStatus.Status.AllocatableBandwidth {
			// check whether blockio has device update
			if iotype != v1.BlockIO {
				continue
			}
			oinfo, ok := oldStatus.Status.AllocatableBandwidth[iotype]
			if !ok || len(ninfo.DeviceIOStatus) == len(oinfo.DeviceIOStatus) {
				continue
			}
			addDev := make(map[string]v1.IOAllocatableBandwidth)
			delDev := make(map[string]v1.IOAllocatableBandwidth)
			for dev, info := range oinfo.DeviceIOStatus {
				if _, ok := ninfo.DeviceIOStatus[dev]; !ok {
					delDev[dev] = info
				}
			}
			for dev, info := range ninfo.DeviceIOStatus {
				if _, ok := oinfo.DeviceIOStatus[dev]; !ok {
					addDev[dev] = info
				}
			}
			// update device info
			if len(addDev) > 0 {
				aggr.addDeviceToCache(node, iotype, addDev)
			}
			if len(delDev) > 0 {
				aggr.delDeviceToCache(node, iotype, delDev)
			}
		}
	}
	klog.V(utils.INF).Infof("[UpdateNodeIOStatus] node %v is handled", node)
}

func (aggr *IoisolationAggregator) NodeStaticIOInfoCrAdd(obj interface{}) {
	nodeInfo, ok := obj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[NodeStaticIOInfoCrAdd]cannot convert oldObj to *v1.NodeStaticIOInfo: %v", obj)
		return
	}
	node := nodeInfo.Spec.NodeName
	klog.V(utils.DBG).Info("Watch: NodeStaticIOInfoCrAdd:", node, " ", nodeInfo.Spec)
	aggr.Lock()
	defer aggr.Unlock()
	if _, ok := aggr.nodeInfo4Agent[node]; !ok {
		aggr.nodeInfo4Agent[node] = &NodeInfo4Agent{
			reservedPod: make(map[string]v1.PodRequest),
		}
	}
	if nodeInfo.Spec.EndPoint != "" {
		err := InitClient(aggr.nodeInfo4Agent[node], nodeInfo, aggr.mtls)
		if err != nil {
			klog.Errorf("failed to connect to node agent service %v: %v", nodeInfo.Spec.EndPoint, err)
		} else {
			klog.V(utils.INF).Infof("grpc connected to node %v: %v", nodeInfo.Spec.NodeName, nodeInfo.Spec.EndPoint)
		}
	}
	klog.V(utils.INF).Infof("[NodeStaticIOInfoCrAdd] node %v is handled", nodeInfo.Spec.NodeName)
}

func (aggr *IoisolationAggregator) NodeStaticIOInfoCrUpdate(oldObj, newObj interface{}) {
	oldNodeInfo, ok := oldObj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[NodeStaticIOInfoCrUpdate]cannot convert oldObj to *v1.NodeStaticIOInfo: %v", oldObj)
		return
	}
	newNodeInfo, ok := newObj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[NodeStaticIOInfoCrUpdate]cannot convert newObj to *v1.NodeStaticIOInfo: %v", newObj)
		return
	}
	klog.V(utils.DBG).Info("Watch: NodeStaticIOInfoCrUpdate:", newNodeInfo.Spec.NodeName, " ", newNodeInfo.Spec)
	aggr.Lock()
	defer aggr.Unlock()
	node := newNodeInfo.Spec.NodeName
	if _, ok := aggr.nodeInfo4Agent[node]; !ok {
		aggr.nodeInfo4Agent[node] = &NodeInfo4Agent{
			reservedPod: make(map[string]v1.PodRequest),
		}
	}

	if aggr.nodeInfo4Agent[node].client == nil || oldNodeInfo.Spec.EndPoint != newNodeInfo.Spec.EndPoint {
		if aggr.nodeInfo4Agent[node].conn != nil {
			if err := aggr.nodeInfo4Agent[node].conn.Close(); err != nil {
				klog.Warning("grpc connection close fail:", err)
			}
		}
		err := InitClient(aggr.nodeInfo4Agent[node], newNodeInfo, aggr.mtls)
		if newNodeInfo.Spec.EndPoint != "" {
			if err != nil {
				klog.Errorf("failed to connect to node agent service %v: %v", newNodeInfo.Spec.EndPoint, err)
			} else {
				klog.V(utils.INF).Infof("grpc connected to node %v: %v", newNodeInfo.Spec.NodeName, newNodeInfo.Spec.EndPoint)
			}
		}
	}
	klog.V(utils.INF).Infof("[NodeStaticIOInfoCrUpdate] node %v is handled", newNodeInfo.Spec.NodeName)
}

func (aggr *IoisolationAggregator) addResourcetoCache(node string, old, new map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth) {
	if aggr.nodeInfo4Sched[node].AllocatableBandwidth == nil {
		aggr.nodeInfo4Sched[node].AllocatableBandwidth = make(map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth)
	}
	for iotype, info := range new {
		if _, ok := old[iotype]; !ok {
			aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype] = info
		}
	}
}

func (aggr *IoisolationAggregator) addDeviceToCache(node string, iotype v1.WorkloadIOType, addDev map[string]v1.IOAllocatableBandwidth) {
	if aggr.nodeInfo4Sched[node].AllocatableBandwidth == nil {
		aggr.nodeInfo4Sched[node].AllocatableBandwidth = make(map[v1.WorkloadIOType]v1.DeviceAllocatableBandwidth)
	}
	if _, ok := aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype]; !ok {
		aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype] = v1.DeviceAllocatableBandwidth{
			DeviceIOStatus: make(map[string]v1.IOAllocatableBandwidth),
		}
	}
	if aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype].DeviceIOStatus == nil {
		deviceAllocatableBandwidth := aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype]
		deviceAllocatableBandwidth.DeviceIOStatus = make(map[string]v1.IOAllocatableBandwidth)
		aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype] = deviceAllocatableBandwidth
	}
	for dev, info := range addDev {
		aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype].DeviceIOStatus[dev] = info
	}
}

func (aggr *IoisolationAggregator) delDeviceToCache(node string, iotype v1.WorkloadIOType, addDev map[string]v1.IOAllocatableBandwidth) {
	if _, ok := aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype]; !ok {
		return
	}
	if aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype].DeviceIOStatus == nil {
		return
	}
	for dev := range addDev {
		delete(aggr.nodeInfo4Sched[node].AllocatableBandwidth[iotype].DeviceIOStatus, dev)
	}
}

// update pod label
func (aggr *IoisolationAggregator) UpdateEndpointLabelToPod(add bool) error {
	podName := os.Getenv("POD_NAME")
	namespace := os.Getenv("POD_NAMESPACE")
	pod, err := aggr.coreclient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("fail to get current pod: %v", err)
	}
	if add {
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		pod.Labels["aggregator-role"] = EndpointLabel
	} else {
		if pod.Labels != nil {
			delete(pod.Labels, "aggregator-role")
		}
	}
	_, err = aggr.coreclient.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("fail to update current pod: %v", err)
	}
	return nil
}

// update NodeIOStatus according to node list
func (aggr *IoisolationAggregator) updateNodeIOStatus(ctx context.Context, obj interface{}) error {
	nodesRaw, ok := obj.(string)
	if !ok {
		return fmt.Errorf("cannot convert obj to raw node list type")
	}
	nodes := strings.Split(nodesRaw, ";")
	for _, node := range nodes {
		status, err := aggr.client.IoiV1().NodeIOStatuses(utils.CrNamespace).Get(ctx, node+utils.NodeStatusInfoSuffix, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("cannot get nodeiostatus cr: %v", err)
			continue
		}
		status.Status = *aggr.nodeInfo4Sched[node]
		_, err = aggr.client.IoiV1().NodeIOStatuses(utils.CrNamespace).UpdateStatus(ctx, status, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("cannot update nodeiostatus cr: %v", err)
			continue
		}
	}
	return nil
}

// update podList to node agent grpc server
func (aggr *IoisolationAggregator) updatePodList(ctx context.Context, obj interface{}) error {
	pl, ok := obj.(*nodeagent.UpdateReservedPodsRequest)
	if !ok {
		return fmt.Errorf("cannot convert obj to UpdateReservedPodsRequest type")
	}
	nodeInfo, ok := aggr.nodeInfo4Agent[pl.NodeName]
	if !ok {
		return fmt.Errorf("node %v not registed in aggregator cache", pl.NodeName)
	}
	if pl.ReservedPods.Generation < aggr.nodeInfo4Agent[pl.NodeName].latestGeneration {
		klog.V(utils.DBG).Infof("request %v out of date", pl.String())
		return nil
	}
	if nodeInfo.client != nil {
		klog.V(utils.DBG).Info("UpdateReservedPods to node agent:", pl.String())
		ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		_, err := nodeInfo.client.UpdateReservedPods(ctxTimeout, pl)
		if err != nil {
			return fmt.Errorf("UpdateReservedPods to nodeagent %v err: %v", pl.NodeName, err)
		}
		klog.V(utils.DBG).Info("Send UpdateReservedPods to nodeagent:", pl.String())
	} else {
		klog.Errorf("node %v grpc client to node agent has not been created", pl.NodeName)
	}
	return nil
}

func parseEndPoint(addr string) (*Endpoint, error) {
	strs := strings.Split(addr, ":")
	if len(strs) != 2 {
		return nil, fmt.Errorf("endpoint format error: %v", addr)
	}
	ip, port := strs[0], strs[1]
	if !verifyEndpoint(ip, port) {
		return nil, fmt.Errorf("endpoint in wrong format: %s:%s", ip, port)
	}
	endpoint := &Endpoint{
		IP:   ip,
		Port: port,
	}
	return endpoint, nil
}

func verifyEndpoint(ip, port string) bool {
	if net.ParseIP(ip) == nil {
		return false
	}
	num, err := strconv.Atoi(port)
	if err != nil || num < 0 || num > 65535 {
		return false
	}
	return true
}

func InitClient(cacheInfo *NodeInfo4Agent, nodeInfo *v1.NodeStaticIOInfo, mtls bool) error {
	endpoint, err := parseEndPoint(nodeInfo.Spec.EndPoint)
	if err != nil {
		return fmt.Errorf("parse endpoint error: %v", err)
	}
	var conn *grpc.ClientConn
	if mtls {
		conn, err = grpc.Dial(ip2FQDN(endpoint.IP)+":"+endpoint.Port, grpc.WithTransportCredentials(utils.LoadKeyPair(filepath.Join(utils.TlsPath, CACertFile), AggrKeyFile, AggrCertFile, false)))
	} else {
		conn, err = grpc.Dial(endpoint.IP+":"+endpoint.Port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if err != nil {
		return fmt.Errorf("grpc dial error: %v", err)
	}
	client := nodeagent.NewNodeAgentClient(conn)
	cacheInfo.client = client
	cacheInfo.conn = conn
	return nil
}

func ip2FQDN(ipaddr string) string {
	return fmt.Sprintf("%s.%s.pod.cluster.local", strings.Replace(ipaddr, ".", "-", -1), utils.IoiNamespace)
}

func (aggr *IoisolationAggregator) NodeStaticIOInfoCrDelete(obj interface{}) {
	nodeInfo, ok := obj.(*v1.NodeStaticIOInfo)
	if !ok {
		klog.Errorf("[DeleteNodeIOInfo]cannot convert oldObj to *v1.NodeIOInfo: %v", obj)
		return
	}
	klog.Info("Watch: DeleteNodeIOInfo:", nodeInfo.Spec.NodeName)
	aggr.Lock()
	defer aggr.Unlock()
	node := nodeInfo.Spec.NodeName
	if _, ok := aggr.nodeInfo4Agent[node]; ok {
		if aggr.nodeInfo4Agent[node].conn != nil {
			if err := aggr.nodeInfo4Agent[node].conn.Close(); err != nil {
				klog.Warning("grpc connection close fail:", err)
			}
		}
		delete(aggr.nodeInfo4Agent, node)
	}
}
