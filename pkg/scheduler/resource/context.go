/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package resource

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	common "sigs.k8s.io/IOIsolation/pkg/scheduler/util"
)

var IoiContext *ResourceIOContext

type StorageInfo struct {
	DevID            string
	RequestedStorage int64
	NodeName         string
}

type syncContext struct {
	spec *v1.NodeIOStatusSpec
	pod  *corev1.Pod
	req  *pb.PodRequest
}

type ResourceIOContext struct {
	Client         kubernetes.Interface
	VClient        versioned.Interface
	Reservedpod    map[string]*pb.PodList // nodename -> PodList
	NsWhiteList    []string
	ClaimDevMap    map[string]*StorageInfo // key->pvcname.ns
	queue          workqueue.RateLimitingInterface
	lastUpdatedGen map[string]int64
	sync.Mutex
}

func NewContext(rl workqueue.RateLimiter, wl []string, h framework.Handle) (*ResourceIOContext, error) {
	if rl == nil {
		return nil, fmt.Errorf("rate limiter is nil")
	}
	queue := workqueue.NewNamedRateLimitingQueue(rl, "ResourceIO plugin")
	cfg := h.KubeConfig()
	cfg.ContentType = "application/json"
	c, err := versioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &ResourceIOContext{
		Reservedpod:    make(map[string]*pb.PodList),
		NsWhiteList:    wl,
		ClaimDevMap:    make(map[string]*StorageInfo),
		VClient:        c,
		Client:         h.ClientSet(),
		queue:          queue,
		lastUpdatedGen: make(map[string]int64),
	}, nil
}

func (c *ResourceIOContext) RunWorkerQueue(ctx context.Context) {
	defer c.queue.ShutDown()

	for {
		obj, shutdown := c.queue.Get()
		if shutdown {
			break
		}
		err := func() error {
			defer c.queue.Done(obj)

			switch obj := obj.(type) {
			case *syncContext: // update Reserved Pods
				return c.updateContext(ctx, obj.spec, obj.pod, obj.req)
			default:
				klog.Warningf("unexpected work item %#v", obj)
			}
			return nil
		}()
		if err != nil {
			klog.Errorf("work queue handle data error: %v", err)
			klog.Warningf("Retrying %#v after %d failures", obj, c.queue.NumRequeues(obj))
			c.queue.AddRateLimited(obj)
		} else {
			c.queue.Forget(obj)
		}
	}
	<-ctx.Done()
}

func (c *ResourceIOContext) GetStorageInfo(pvc string) (*StorageInfo, error) {
	if v, ok := c.ClaimDevMap[pvc]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("pvc %v doesn't exist in reserved pod", pvc)
}

func (c *ResourceIOContext) GetNsWhiteList() []string {
	return c.NsWhiteList
}

func (c *ResourceIOContext) GetReservedPods(node string) (*pb.PodList, error) {
	if v, ok := c.Reservedpod[node]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("node %v doesn't exist in reserved pod", node)
}

func (c *ResourceIOContext) SetPodListGen(node string, gen int64) error {
	if v, ok := c.Reservedpod[node]; ok {
		if v == nil {
			v = &pb.PodList{
				PodList: make(map[string]*pb.PodRequest),
			}
		}
		v.Generation = gen
		return nil
	}
	return fmt.Errorf("node %v doesn't exist in reserved pod", node)
}

func (c *ResourceIOContext) GetReservedPodsWithNameNS(node string) (*v1.NodeIOStatusSpec, error) {
	if v, ok := c.Reservedpod[node]; ok {
		pods := map[string]v1.PodRequest{}
		if v == nil {
			return nil, fmt.Errorf("node %v's pod list is nil", node)
		}
		for uid, pr := range v.PodList {
			pods[uid] = v1.PodRequest{
				Name:      pr.PodName,
				Namespace: pr.PodNamespace,
			}
		}

		return &v1.NodeIOStatusSpec{
			NodeName:     node,
			Generation:   v.Generation,
			ReservedPods: pods,
		}, nil
	}
	return nil, fmt.Errorf("node %v doesn't exist in reserved pod", node)
}

func (c *ResourceIOContext) SetReservedPods(node string, pl *pb.PodList) {
	c.Reservedpod[node] = pl
	c.lastUpdatedGen[node] = -1
}

func (c *ResourceIOContext) updateContext(ctx context.Context, spec *v1.NodeIOStatusSpec, p *corev1.Pod, req *pb.PodRequest) error {
	if spec == nil || len(spec.NodeName) == 0 {
		klog.Error("Invalid NodeIOStatusSpec, ignore it")
		return nil
	}
	gen, ok := c.lastUpdatedGen[spec.NodeName]
	if !ok {
		gen = -1
	}
	c.Lock()
	defer c.Unlock()
	if spec.Generation <= gen {
		klog.V(utils.DBG).Infof("The Spec generation is small than the latest update, skip it. ")
		return nil
	}

	if req != nil {
		// add disk allocation to pod annotation
		reqBW, err := common.PodRequest2String(req)
		if err != nil {
			return err
		}
		err = common.AddOrUpdateAnnoOnPod(ctx, c.Client, p, map[string]string{
			utils.AllocatedIOAnno: reqBW,
		})
		if err != nil {
			return fmt.Errorf("AddOrUpdateAnnoOnPod fails: %v", err)

		}
	} else {
		// delete pod annotation
		err := common.DeleteAnnoOnPod(ctx, c.Client, p, utils.AllocatedIOAnno)
		if err != nil && !kubeerr.IsNotFound(err) {
			return fmt.Errorf("DeleteAnnoOnPod fails: %v", err)
		}
	}
	if err := common.UpdateNodeIOStatusSpec(c.VClient, spec.NodeName, spec); err != nil {
		return err
	}
	c.lastUpdatedGen[spec.NodeName] = spec.Generation
	return nil
}

func (c *ResourceIOContext) UpdateReservedPods(node string, pod *corev1.Pod, req *pb.PodRequest) {
	reservedPod, err := c.GetReservedPodsWithNameNS(node)
	if err != nil {
		klog.Errorf("reserved pod period update reserved pod err: %v", err)
	}
	c.queue.Add(&syncContext{
		spec: reservedPod,
		pod:  pod,
		req:  req,
	})
}

func (c *ResourceIOContext) RemoveNode(node string) {
	delete(c.Reservedpod, node)
}

func (c *ResourceIOContext) InNamespaceWhiteList(ns string) bool {
	for _, n := range c.NsWhiteList {
		if ns == n {
			return true
		}
	}
	return false
}

func (c *ResourceIOContext) AddPod(ctx context.Context, reqlist *pb.PodRequest, pod *corev1.Pod, nodeName string) error {
	c.Lock()
	defer c.Unlock()
	podreq, err := c.GetReservedPods(nodeName)
	if err != nil {
		return fmt.Errorf("get reserved pods error: %v", err)
	}
	podreq.Generation += 1
	podreq.PodList[string(pod.UID)] = reqlist
	c.SetReservedPods(nodeName, podreq)
	c.UpdateReservedPods(nodeName, pod, reqlist)
	return nil
}

func (c *ResourceIOContext) RemovePod(ctx context.Context, pod *corev1.Pod, nodeName string) error {
	c.Lock()
	defer c.Unlock()
	v, err := c.GetReservedPods(pod.Spec.NodeName)
	if err != nil {
		return fmt.Errorf("get reserved pods error: %v", err)
	}
	v.Generation += 1
	delete(v.PodList, string(pod.UID))
	c.UpdateReservedPods(nodeName, pod, nil)
	return nil
}
