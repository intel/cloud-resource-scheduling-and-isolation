/*
Copyright (C) 2024 Intel Corporation

This software and the related documents are Intel copyrighted materials,
and your use of them is governed by the express license under which they
were provided to you ("License"). Unless the License provides otherwise,
you may not use, modify, copy, publish, distribute, disclose or transmit
this software or the related documents without Intel's prior written
permission.

This software and the related documents are provided as is, with no
express or implied warranties, other than those that are expressly stated
in the License.
*/

package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/api/nodeagent"
)

const (
	ChanDepth     = 32
	CommonName    = "AggregatorServer"
	CAKeyFile     = "root-ca.key"
	CACertFile    = "root-ca.crt"
	AggrKeyFile   = "/etc/ioi/control/pki/aggr-server.key"
	AggrCertFile  = "/etc/ioi/control/pki/aggr-server.crt"
	FQDN          = "aggregator.ioi-system.svc.cluster.local"
	EndpointLabel = "endpoint-aggregator"
)

type IoisolationAggregator struct {
	aggregator.UnimplementedIOAggregator4AgentServer
	server4Agent   *grpc.Server
	nodeInfo4Agent map[string]*NodeInfo4Agent        // nodeName -> nodeInfo
	nodeInfo4Sched map[string]*v1.NodeIOStatusStatus // nodeName -> nodeStatus
	NodeStatusCh   chan *aggregator.UpdateNodeIOStatusRequest
	ReservedPodCh  chan *v1.NodeIOStatusSpec // nodeName
	queue          workqueue.RateLimitingInterface
	timer          *time.Ticker
	client         versioned.Interface // to watch cr
	coreclient     kubernetes.Interface
	mtls           bool
	direct         bool
	isLeader       bool
	sync.Mutex
}

func NewIoisolationAggregator(cfg *Config, config *rest.Config, coreclient kubernetes.Interface, isLeader bool) (*IoisolationAggregator, error) {
	aggregator := &IoisolationAggregator{
		nodeInfo4Agent: make(map[string]*NodeInfo4Agent, ChanDepth),
		nodeInfo4Sched: make(map[string]*v1.NodeIOStatusStatus),
		NodeStatusCh:   make(chan *aggregator.UpdateNodeIOStatusRequest, ChanDepth),
		ReservedPodCh:  make(chan *v1.NodeIOStatusSpec, ChanDepth),
		timer:          time.NewTicker(cfg.UpdateInterval),
		mtls:           cfg.IsMtls,
		direct:         cfg.Direct,
		isLeader:       isLeader,
	}
	if aggregator.mtls {
		// sign certificate from root CA
		err := utils.CertSetup(filepath.Join(utils.TlsPath, CAKeyFile), filepath.Join(utils.TlsPath, CACertFile), CommonName, []string{FQDN, "localhost"}, AggrKeyFile, AggrCertFile)
		if err != nil {
			klog.Errorf("cannot create aggregator cert: %v", err)
		}
	}
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 10*time.Second)
	aggregator.queue = workqueue.NewNamedRateLimitingQueue(rateLimiter, "aggregator")
	client, err := versioned.NewForConfig(config)
	if err != nil {
		klog.Error("Clientset init err")
		return nil, err
	}
	aggregator.client = client
	aggregator.coreclient = coreclient
	err = aggregator.WatchNodeInfo()
	if err != nil {
		klog.Error("Watch node static Info error")
		return nil, err
	}
	err = aggregator.WatchConfigMap()
	if err != nil {
		klog.Error("Watch configmap error")
		return nil, err
	}
	return aggregator, nil
}

func (aggr *IoisolationAggregator) Run(ctx context.Context, c chan os.Signal) {
	go aggr.runServer4Agent(ctx)
	go aggr.StartWorkQueue(ctx)
	go aggr.SendReservedPodtoQueue(ctx, c) // can also specify thread num
	go aggr.SendNodeStatustoQueue(ctx, c)
}
func (aggr *IoisolationAggregator) SetLeader(isLeader bool) {
	aggr.Lock()
	defer aggr.Unlock()
	aggr.isLeader = isLeader
}

// a workqueue running to send reservedPod to node agents and update NodeIOStatus's IO status.
func (aggr *IoisolationAggregator) StartWorkQueue(ctx context.Context) {
	for {
		obj, shutdown := aggr.queue.Get()
		if shutdown {
			klog.V(utils.INF).Info("workqueue is shutdown")
			return
		}
		err := func() error {
			defer aggr.queue.Done(obj)

			switch obj := obj.(type) {
			case string: // update Status
				return aggr.updateNodeIOStatus(ctx, obj)
			case *nodeagent.UpdateReservedPodsRequest: //Update ResevedPod
				return aggr.updatePodList(ctx, obj)
			default:
				klog.Warningf("unexpected work item %#v", obj)
			}

			return nil
		}()
		if err != nil {
			klog.Errorf("work queue handle data error: %v", err)
			klog.Warningf("Retrying %#v after %d failures", obj, aggr.queue.NumRequeues(obj))
			aggr.queue.AddRateLimited(obj)
		} else {
			aggr.queue.Forget(obj)
		}
	}
}

// prepare podlist to be sent to node agent, pod in added map fills its resource request details in the pod list.
func (aggr *IoisolationAggregator) GetPodList(gen int64, pods, added map[string]v1.PodRequest) (*aggregator.PodList, error) {
	pl := &aggregator.PodList{
		PodList: make(map[string]*aggregator.PodRequest),
	}
	pl.Generation = gen
	for id := range pods {
		if v, ok := added[id]; ok {
			p, err := aggr.coreclient.CoreV1().Pods(v.Namespace).Get(context.TODO(), v.Name, metav1.GetOptions{})
			if err != nil {
				klog.Warningf("get pods %v error: %v", v.Name, err)
				continue
			}
			v, ok := p.Annotations[utils.AllocatedIOAnno]
			if !ok {
				klog.Warningf("annotation %v not exist in pod %v", utils.AllocatedIOAnno, p.Name)
				pl.PodList[id] = nil
				continue
			}
			allocatedIO := utils.AllocatedIO{}
			err = json.Unmarshal([]byte(v), &allocatedIO)
			if err != nil {
				klog.Errorf("cannot parse to block io request/limit: %v", err)
				continue
			}
			pl.PodList[id] = formatPodRequest(allocatedIO)
			pl.PodList[id].PodName = p.Name
			pl.PodList[id].PodNamespace = p.Namespace
		} else {
			pl.PodList[id] = nil
		}
	}
	return pl, nil
}

func formatPodRequest(ori utils.AllocatedIO) *aggregator.PodRequest {
	podRequest := &aggregator.PodRequest{
		Request: []*aggregator.IOResourceRequest{},
	}
	for iotype, devices := range ori {
		for n, dev := range devices {
			req := &aggregator.IOResourceRequest{}
			req.IoType = aggregator.IOType(iotype)
			req.WorkloadType = dev.QoS
			req.DevRequest = &aggregator.DeviceRequirement{
				DevName: n,
				Request: &aggregator.Quantity{
					Inbps:  dev.Request.Inbps,
					Outbps: dev.Request.Outbps,
				},
				Limit: &aggregator.Quantity{
					Inbps:  dev.Limit.Inbps,
					Outbps: dev.Limit.Outbps,
				},
				RawRequest: &aggregator.Quantity{
					Inbps:   dev.RawRequest.Inbps,
					Outbps:  dev.RawRequest.Outbps,
					Iniops:  dev.RawRequest.Iniops,
					Outiops: dev.RawRequest.Outiops,
				},
				RawLimit: &aggregator.Quantity{
					Inbps:   dev.RawLimit.Inbps,
					Outbps:  dev.RawLimit.Outbps,
					Iniops:  dev.RawLimit.Iniops,
					Outiops: dev.RawLimit.Outiops,
				},
			}
			podRequest.Request = append(podRequest.Request, req)
		}
	}
	return podRequest
}

// prepare reservedPod request data, update local cache, and send it to work queue
func (aggr *IoisolationAggregator) SendReservedPodtoQueue(ctx context.Context, c chan os.Signal) {
	for {
		select {
		case nodeToUpdate := <-aggr.ReservedPodCh:
			// compare pod list
			_, ok := aggr.nodeInfo4Agent[nodeToUpdate.NodeName]
			if !ok {
				klog.Errorf("node %v not registed in nodeInfo4Agent cache", nodeToUpdate.NodeName)
				continue
			}
			prevPodList := aggr.nodeInfo4Agent[nodeToUpdate.NodeName].reservedPod
			currPodList := nodeToUpdate.ReservedPods
			added := make(map[string]v1.PodRequest)
			for key, value := range currPodList {
				if _, ok := prevPodList[key]; !ok {
					added[key] = value
				}
			}
			// Update Aggr's ReservedPod cache
			aggr.nodeInfo4Agent[nodeToUpdate.NodeName].reservedPod = currPodList
			aggr.nodeInfo4Agent[nodeToUpdate.NodeName].latestGeneration = nodeToUpdate.Generation
			if aggr.isLeader {
				// get Pod annotation and generate grpc msg
				podList, err := aggr.GetPodList(nodeToUpdate.Generation, currPodList, added)
				if err != nil {
					klog.Errorf("cannot get pod list: %v", err)
					continue
				}
				// put reserved pod request into workqueue
				aggr.queue.Add(&nodeagent.UpdateReservedPodsRequest{
					NodeName:     nodeToUpdate.NodeName,
					ReservedPods: podList,
				})
			}
		case <-c:
			klog.V(utils.INF).Info("SendReservedPodtoQueue force stopped")
			return
		}
	}
}

// update local cache data, and send node status update request to work queue
func (aggr *IoisolationAggregator) SendNodeStatustoQueue(ctx context.Context, c chan os.Signal) {
	updatedNode := []string{}
	for {
		select {
		case req := <-aggr.NodeStatusCh:
			aggr.updateNodeIOStatusCache(req)
			updatedNode = append(updatedNode, req.NodeName)
		case <-aggr.timer.C:
			if len(updatedNode) > 0 {
				data := strings.Join(updatedNode, ";")
				aggr.queue.Add(data)
				updatedNode = []string{}
			}
		case <-c:
			klog.V(utils.INF).Info("SendNodeStatustoQueue force stopped")
			return
		}
	}
}

func (aggr *IoisolationAggregator) updateNodeIOStatusCache(req *aggregator.UpdateNodeIOStatusRequest) {
	status := aggr.nodeInfo4Sched[req.NodeName]
	status.ObservedGeneration = req.ObservedGeneration
	for _, bw := range req.IoBw {
		var iotype v1.WorkloadIOType
		switch bw.IoType {
		case aggregator.IOType_DiskIO:
			iotype = v1.BlockIO
		case aggregator.IOType_NetworkIO:
			iotype = v1.NetworkIO
		case aggregator.IOType_RDT:
			iotype = v1.RDT
		default:
			continue
		}
		devices, ok := status.AllocatableBandwidth[iotype]
		if !ok {
			klog.Errorf("iotype %v not registed", iotype)
			continue
		}
		for dev, info := range bw.DeviceBwInfo {
			if _, ok := devices.DeviceIOStatus[dev]; !ok {
				klog.Errorf("%v device %v not registed", iotype, dev)
				continue
			}
			for qos, st := range info.Status {
				if _, ok := devices.DeviceIOStatus[dev].IOPoolStatus[v1.WorkloadQoSType(qos)]; !ok {
					klog.Errorf("%v device %v qos %v not registed", iotype, dev, qos)
					continue
				}
				newStat := v1.IOStatus{
					Total:    st.Total,
					In:       st.In,
					Out:      st.Out,
					Pressure: st.Pressure,
				}
				devices.DeviceIOStatus[dev].IOPoolStatus[v1.WorkloadQoSType(qos)] = newStat
			}
		}
		status.AllocatableBandwidth[iotype] = devices
	}
}

func (aggr *IoisolationAggregator) RunServer4Agent(ctx context.Context) error {
	// todo: get service port from configmap/conmand line
	l, err := net.Listen("tcp", ":9988")
	if err != nil {
		return fmt.Errorf("failed to start listening %s: %v", "port 9988", err)
	}
	var s *grpc.Server
	if aggr.mtls {
		s = grpc.NewServer(grpc.Creds(utils.LoadKeyPair(filepath.Join(utils.TlsPath, CACertFile), AggrKeyFile, AggrCertFile, true)))
	} else {
		s = grpc.NewServer()
	}
	aggr.server4Agent = s
	defer s.Stop()
	aggregator.RegisterIOAggregator4AgentServer(s, aggr)
	// grpcurl debug only
	reflection.Register(s)
	err = s.Serve(l)
	if err != nil {
		return fmt.Errorf("failed to serve aggregator to node agent: %v", err)
	}
	return nil
}

// Aggregator grpc server for nodeagent. UpdateNodeStatus updates the node's actual IO status
func (aggr *IoisolationAggregator) UpdateNodeIOStatus(ctx context.Context, req *aggregator.UpdateNodeIOStatusRequest) (*aggregator.Empty, error) {
	klog.V(utils.DBG).Info("Envoke UpdateNodeIOStatus from nodeagent: nodeName=", req.NodeName, ", Observed generation=", req.ObservedGeneration, ", BW=", req.IoBw)
	aggr.Lock()
	defer aggr.Unlock()
	if !aggr.isLeader {
		return &aggregator.Empty{}, fmt.Errorf("aggregator is not leader")
	}
	if _, ok := aggr.nodeInfo4Agent[req.NodeName]; !ok {
		return &aggregator.Empty{}, fmt.Errorf("node %v not registed in aggregator cache for node agent", req.NodeName)
	}
	if _, ok := aggr.nodeInfo4Sched[req.NodeName]; !ok {
		return &aggregator.Empty{}, fmt.Errorf("node %v not registed in aggregator cache for scheduler", req.NodeName)
	}
	if req.ObservedGeneration < aggr.nodeInfo4Agent[req.NodeName].latestGeneration {
		klog.V(utils.DBG).Infof("generation inconsistent: observed generation: %v, latest pod generation %v", req.ObservedGeneration, aggr.nodeInfo4Agent[req.NodeName].latestGeneration)
		if aggr.nodeInfo4Agent[req.NodeName].client != nil && aggr.nodeInfo4Agent[req.NodeName].reservedPod != nil {
			reservedPods := aggr.nodeInfo4Agent[req.NodeName].reservedPod
			podList, err := aggr.GetPodList(aggr.nodeInfo4Agent[req.NodeName].latestGeneration, reservedPods, reservedPods)
			if err != nil {
				klog.Errorf("cannot get pod list: %v", err)
			} else {
				// put reserved pod request into workqueue
				klog.V(utils.DBG).Info("resync request into workqueue")
				aggr.queue.Add(&nodeagent.UpdateReservedPodsRequest{
					NodeName:     req.NodeName,
					ReservedPods: podList,
				})
			}
		}
		return &aggregator.Empty{}, fmt.Errorf("generation inconsistent: reserved generation: %v, reserved pod generation %v", req.ObservedGeneration, aggr.nodeInfo4Agent[req.NodeName].latestGeneration)
	}
	if !aggr.direct { // update status by interval
		timeout := time.NewTimer(time.Second)
		select {
		case aggr.NodeStatusCh <- req:
			return &aggregator.Empty{}, nil
		case <-timeout.C:
			return &aggregator.Empty{}, fmt.Errorf("io status update has not been sent to scheduler")
		}
	} else { // update status directly
		aggr.updateNodeIOStatusCache(req)
		aggr.queue.Add(req.NodeName)
	}
	return &aggregator.Empty{}, nil
}

// Aggregator grpc server for nodeagent.GetReservedPods returns the pod list to node agent
func (aggr *IoisolationAggregator) GetReservedPods(ctx context.Context, req *aggregator.GetReservedPodsRequest) (*aggregator.GetReservedPodsResponse, error) {
	klog.V(utils.DBG).Info("Envoke GetReservedPods from nodeagent: ", req.NodeName)
	aggr.Lock()
	defer aggr.Unlock()
	if !aggr.isLeader {
		return &aggregator.GetReservedPodsResponse{}, fmt.Errorf("aggregator is not leader")
	}
	if _, ok := aggr.nodeInfo4Agent[req.NodeName]; !ok {
		return &aggregator.GetReservedPodsResponse{}, fmt.Errorf("node %s not registed in aggregator", req.NodeName)
	}
	generation := aggr.nodeInfo4Agent[req.NodeName].latestGeneration
	existingPods := aggr.nodeInfo4Agent[req.NodeName].reservedPod
	podList, err := aggr.GetPodList(generation, existingPods, existingPods)
	if err != nil {
		return nil, fmt.Errorf("cannot get pod list: %v", err)
	}
	resp := &aggregator.GetReservedPodsResponse{
		PodList: podList,
	}
	klog.V(utils.DBG).Info("GetReservedPods resp:", resp.String())
	return resp, nil
}

func (aggr *IoisolationAggregator) Stop() {
	for _, nodeInfo := range aggr.nodeInfo4Agent {
		if nodeInfo.conn != nil {
			_ = nodeInfo.conn.Close()
		}
	}
	if aggr.server4Agent != nil {
		aggr.server4Agent.Stop()
	}
	aggr.timer.Stop()
	if aggr.NodeStatusCh != nil {
		close(aggr.NodeStatusCh)
	}
	if aggr.ReservedPodCh != nil {
		close(aggr.ReservedPodCh)
	}
	if aggr.queue != nil {
		aggr.queue.ShutDown()
	}
}

func (aggr *IoisolationAggregator) runServer4Agent(ctx context.Context) {
	err := aggr.RunServer4Agent(ctx)
	if err != nil {
		klog.Error("RunServer4Agent exit: %v ", err)
		aggr.Stop()
		os.Exit(1)
	}
}
