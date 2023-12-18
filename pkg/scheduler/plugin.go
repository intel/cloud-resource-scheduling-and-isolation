/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package scheduler

import (
	"context"
	"time"

	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"sigs.k8s.io/IOIsolation/api/config"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/blockio"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/networkio"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/rdt"
	rdtquantity "sigs.k8s.io/IOIsolation/pkg/scheduler/rdt-quantity"
	"sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	common "sigs.k8s.io/IOIsolation/pkg/scheduler/util"
)

type ResourceIO struct {
	rhs    map[string]resource.Handle
	scorer map[string]Scorer
	client kubernetes.Interface
}

// Name is the name of the plugin used in the Registry and configurations.
const (
	Name           = "ResourceIO"
	stateKeyPrefix = "ResourceIO-"
)

var _ = framework.FilterPlugin(&ResourceIO{})
var _ = framework.ScorePlugin(&ResourceIO{})
var _ = framework.ReservePlugin(&ResourceIO{})

type stateData struct {
	request           interface{}
	nodeResourceState interface{} // change name
	nodeSupportIOI    bool
}

func (d *stateData) Clone() framework.StateData {
	return d
}

// read by reserve and score
func getStateData(cs *framework.CycleState, key string) (*stateData, error) {
	state, err := cs.Read(framework.StateKey(key))
	if err != nil {
		return nil, err
	}
	s, ok := state.(*stateData)
	if !ok {
		return nil, errors.New("unable to convert state into stateData")
	}
	return s, nil
}

func deleteStateData(cs *framework.CycleState, key string) {
	cs.Delete(framework.StateKey(key))
}

// New initializes a new plugin and returns it.
func New(ctx context.Context, configuration runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	args, ok := configuration.(*config.ResourceIOArgs)
	if !ok {
		return nil, fmt.Errorf("want args to be of type *ResourceIOArgs, got %T", args)
	}

	nsWhiteList := common.GetNamespaceWhiteList(handle.ClientSet())
	klog.V(utils.DBG).Info("namespace white list: ", nsWhiteList)

	var ss map[string]Scorer
	var rhs map[string]resource.Handle
	ss = make(map[string]Scorer)
	rhs = make(map[string]resource.Handle)

	rts, err := getResources(args.ResourceType)
	if err != nil {
		return nil, err

	}
	for _, rt := range rts {
		if rt == common.BlockIO {
			b := blockio.New(args.BinPackingStrategy)
			rhs[string(rt)] = b
		} else if rt == common.NetworkIO {
			n := networkio.New()
			rhs[string(rt)] = n
		} else if rt == common.RDT {
			r := rdt.New()
			rhs[string(rt)] = r
		} else if rt == common.RDTQuantity {
			rq := rdtquantity.New()
			rhs[string(rt)] = rq
		}
		s, err := getScorer(common.ScoreStrategyType(args.ScoreStrategy), common.ResourceType(rt))
		if err != nil {
			return nil, err
		}
		ss[string(rt)] = s
	}
	// run resource handle
	for _, rh := range rhs {
		err := rh.Run(resource.NewExtendedCache(), handle.SharedInformerFactory(), handle.ClientSet())
		if err != nil {
			return nil, err
		}
	}
	// ctx := context.Background()
	ratelimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Second) // todo: load from config
	resource.IoiContext, err = resource.NewContext(ratelimiter, nsWhiteList, handle)
	if err != nil {
		return nil, err
	}
	go resource.IoiContext.RunWorkerQueue(ctx)
	// build event handler
	eh := resource.NewIOEventHandler(rhs, handle)
	err = eh.BuildEvtHandler(ctx)
	if err != nil {
		return nil, err
	}
	return &ResourceIO{
		rhs:    rhs,
		scorer: ss,
		client: handle.ClientSet(),
	}, nil
}

func (ps *ResourceIO) Name() string {
	return Name
}

func getResources(rt string) ([]common.ResourceType, error) {
	if rt == "" {
		return nil, fmt.Errorf("resource type is unset: %s", rt)
	}
	var rts []common.ResourceType
	switch common.ResourceType(rt) {
	case common.BlockIO:
		rts = append(rts, common.ResourceType(rt))
	case common.NetworkIO:
		rts = append(rts, common.ResourceType(rt))
	case common.RDT:
		rts = append(rts, common.ResourceType(rt))
	case common.RDTQuantity:
		rts = append(rts, common.ResourceType(rt))
	case common.All:
		rts = append(rts, common.BlockIO)
		rts = append(rts, common.NetworkIO)
		rts = append(rts, common.RDT)
		// rts = append(rts, common.RDTQuantity)
	default:
		return nil, fmt.Errorf("unknown resource type %v in score", rt)
	}
	return rts, nil
}

// Filter invoked at the filter extension point.
// Checks if a node has sufficient resources, such as cpu, memory, gpu, opaque int resources etc to run a pod.
// It returns a list of insufficient resources, if empty, then the node has all the resources requested by the pod.
func (ps *ResourceIO) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	klog.V(utils.DBG).Info("Enter Filter")
	if resource.IoiContext.InNamespaceWhiteList(pod.Namespace) {
		return framework.NewStatus(framework.Success)
	}
	node := nodeInfo.Node()
	nodeName := node.Name

	for _, rh := range ps.rhs {
		exist := rh.(resource.CacheHandle).NodeRegistered(nodeName)
		if !exist {
			if rh.(resource.CacheHandle).CriticalPod(pod.Annotations) {
				return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("node %v without ioisolation support cannot schedule GA/BT workload", nodeName))
			} else {
				state.Write(framework.StateKey(stateKeyPrefix+rh.Name()+nodeName), &stateData{nodeSupportIOI: false})
				continue
			}
		}
		request, err := rh.(resource.CacheHandle).AdmitPod(pod, nodeName)
		if err != nil {
			return framework.NewStatus(framework.Unschedulable, err.Error())
		}
		ok, r, err := rh.(resource.CacheHandle).CanAdmitPod(nodeName, request)
		if !ok {
			return framework.NewStatus(framework.Unschedulable, err.Error())
		}

		state.Write(framework.StateKey(stateKeyPrefix+rh.Name()+nodeName), &stateData{request: request, nodeResourceState: r, nodeSupportIOI: rh.(resource.CacheHandle).NodeHasIoiLabel(node)})
	}
	return framework.NewStatus(framework.Success)
}

// Score invoked at the score extension point.
func (ps *ResourceIO) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	klog.V(utils.DBG).Info("Enter Score ", nodeName)
	if resource.IoiContext.InNamespaceWhiteList(pod.Namespace) {
		return framework.MaxNodeScore, framework.NewStatus(framework.Success)
	}
	score := int64(0)
	for key, rh := range ps.rhs {
		sd, err := getStateData(state, stateKeyPrefix+rh.Name()+nodeName)
		if err != nil {
			return framework.MinNodeScore, framework.NewStatus(framework.Unschedulable, err.Error())
		}
		// BE pod schedule to node without IOI support
		if !sd.nodeSupportIOI {
			score += framework.MaxNodeScore
		} else {
			s, err := ps.scorer[key].Score(sd, rh)
			if err != nil {
				return framework.MinNodeScore, framework.NewStatus(framework.Unschedulable, err.Error())
			} else {
				score += s
			}
		}
	}
	if len(ps.rhs) == 0 {
		return framework.MaxNodeScore, framework.NewStatus(framework.Success)
	}
	return score / int64(len(ps.rhs)), framework.NewStatus(framework.Success)
}

// ScoreExtensions of the Score plugin.
func (ps *ResourceIO) ScoreExtensions() framework.ScoreExtensions {
	return nil
}

// Reserve is the functions invoked by the framework at "reserve" extension point.
func (ps *ResourceIO) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	klog.V(utils.DBG).Info("Enter Reserve")
	if resource.IoiContext.InNamespaceWhiteList(pod.Namespace) {
		return framework.NewStatus(framework.Success, "")
	}
	reqlist := &pb.PodRequest{
		PodName:      pod.Name,
		PodNamespace: pod.Namespace,
		Request:      []*pb.IOResourceRequest{},
	}
	for _, rh := range ps.rhs {
		sd, err := getStateData(state, stateKeyPrefix+rh.Name()+nodeName)
		if err != nil {
			return framework.NewStatus(framework.Unschedulable, err.Error())
		}
		reqs, ok, err := rh.(resource.CacheHandle).AddPod(pod, nodeName, sd.request, ps.client)
		if !ok {
			return framework.NewStatus(framework.Unschedulable, err.Error())
		}
		if err != nil {
			if rh.(resource.CacheHandle).CriticalPod(pod.Annotations) {
				return framework.NewStatus(framework.Unschedulable, err.Error())
			} else {
				continue
			}
		} else {
			reqlist.Request = append(reqlist.Request, reqs...)
		}
		klog.V(utils.DBG).Info("After add pod in cache")
		rh.(resource.CacheHandle).PrintCacheInfo()

		clearStateData(state, ps.client, rh.Name())
	}
	// update reserved pod if the pod has resource request to synchronize
	if reqlist.Request != nil && len(reqlist.Request) > 0 {
		err := resource.IoiContext.AddPod(ctx, reqlist, pod, nodeName)
		if err != nil {
			return framework.NewStatus(framework.Unschedulable, err.Error())
		}
	}
	return framework.NewStatus(framework.Success, "")
}

func (ps *ResourceIO) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	klog.V(utils.DBG).Info("Enter Unreserve")
	if resource.IoiContext.InNamespaceWhiteList(pod.Namespace) {
		return
	}
	for _, rh := range ps.rhs {
		err := rh.(resource.CacheHandle).RemovePod(pod, nodeName, ps.client)
		if err != nil {
			klog.Error("Unreserve pod error: ", err.Error())
		}
	}
	err := resource.IoiContext.RemovePod(ctx, pod, nodeName)
	if err != nil {
		klog.Errorf("fail to remove pod in ReservedPod: %v", err)
	}
}

func clearStateData(state *framework.CycleState, client kubernetes.Interface, rhName string) {
	// delete all cycle states for all nodes
	nodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Get nodes error:%v", err)
	}
	for _, node := range nodes.Items {
		deleteStateData(state, stateKeyPrefix+rhName+node.Name)
	}
}

// EventsToRegister returns the possible events that may make a Pod
// failed by this plugin schedulable.
// NOTE: if in-place-update (KEP 1287) gets implemented, then PodUpdate event
// should be registered for this plugin since a Pod update may free up resources
// that make other Pods schedulable.
func (ps *ResourceIO) EventsToRegister() []framework.ClusterEvent {
	// To register a custom event, follow the naming convention at:
	// https://git.k8s.io/kubernetes/pkg/scheduler/eventhandlers.go#L410-L422

	ce := []framework.ClusterEvent{
		{Resource: framework.Pod, ActionType: framework.All},
		{Resource: framework.Node, ActionType: framework.Delete | framework.UpdateNodeLabel},
		{Resource: framework.GVK(fmt.Sprintf("nodestaticioinfoes.v1.%v", utils.GroupName)), ActionType: framework.All},
		{Resource: framework.GVK(fmt.Sprintf("nodeiostatuses.v1.%v", utils.GroupName)), ActionType: framework.All},
	}
	return ce
}
