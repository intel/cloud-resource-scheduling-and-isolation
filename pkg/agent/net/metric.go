/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"math"
	"sync"

	utils "sigs.k8s.io/IOIsolation/pkg"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
	"sigs.k8s.io/IOIsolation/pkg/agent/common"
	"sigs.k8s.io/IOIsolation/pkg/agent/metrics"
)

var (
	/* net metrics */
	// pod level
	NetPodRequestBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_pods_request_bw",
			Help: "Net request bw of running pods.",
		},
		[]string{"node", "nic", "pod", "wtype", "sr"},
	)
	NetPodLimitBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_pods_limit_bw",
			Help: "Net limited bw of running pods.",
		},
		[]string{"node", "nic", "pod", "wtype", "sr"},
	)
	NetPodRealBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_pods_real_bw",
			Help: "Net bw of real running pods.",
		},
		[]string{"node", "nic", "pod", "wtype", "sr"},
	)
	// nic level
	NetTotalBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_total_bw",
			Help: "Total bw of nics.",
		},
		[]string{"node", "nic", "wtype", "sr"},
	)
	NetRequestSumBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_request_sum_bw",
			Help: "The sum of pods requested bw.",
		},
		[]string{"node", "nic", "wtype", "sr"},
	)
	NetRequestAllocBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_sched_alloc_bw",
			Help: "The allocatable sched bw.",
		},
		[]string{"node", "nic", "wtype", "sr"},
	)
	NetBePodsBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_be_pods_bw",
			Help: "The net min be pod bw.",
		},
		[]string{"node", "nic"},
	)
)

type NodeNetBw map[string]*NetBw // nicId

type NetMetric struct {
	mutex    sync.Mutex
	Cache    map[string]NodeNetBw // nodeName
	PodCache map[string]*PodBw    // nodeName
}

type NetBw struct {
	NetRequestBw []*utils.Workload
	NetAllocBw   []*utils.Workload
	NetTotalBw   []*utils.Workload
}

type PodBw struct {
	PodRequestBw map[string]PodInfos
	PodLimitBw   map[string]PodInfos // diskId
	PodTranBw    map[string]PodInfos
}

type PodInfos map[string]PodInfo //podId

type PodInfo struct {
	PodName  string
	Type     string
	Workload *utils.Workload
}

func (e *NetMetric) Type() string {
	return "Network"
}

func (e *NetMetric) FlushData() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	NetRequestSumBw.Reset()
	NetRequestAllocBw.Reset()
	NetTotalBw.Reset()

	for node, nnbw := range e.Cache {
		for nid, nbw := range nnbw {
			e.updateNetBwAll(NetRequestSumBw, node, nid, nbw.NetRequestBw)
			e.updateNetBwAll(NetRequestAllocBw, node, nid, nbw.NetAllocBw)
			e.updateNetBwAll(NetTotalBw, node, nid, nbw.NetTotalBw)
		}
	}

	NetPodRequestBw.Reset()
	NetPodLimitBw.Reset()
	NetPodRealBw.Reset()
	NetBePodsBw.Reset()
	for node, npbw := range e.PodCache {
		for nid, pif := range npbw.PodRequestBw {
			bePodNum := 0
			for _, nif := range pif {
				poName := nif.PodName
				poType := nif.Type
				e.updateNetPodBwAll(NetPodRequestBw, node, nid, poName, poType, nif.Workload)
				if poType == utils.BE_Type {
					bePodNum = bePodNum + 1
				}
			}
			e.updateNetBePodsBw(NetBePodsBw, node, nid, common.MinPodNetInBw*float64(bePodNum))
		}

		for nid, pif := range npbw.PodLimitBw {
			for _, nif := range pif {
				poName := nif.PodName
				poType := nif.Type
				e.updateNetPodBwAll(NetPodLimitBw, node, nid, poName, poType, nif.Workload)
			}
		}

		for nid, pif := range npbw.PodTranBw {
			for _, nif := range pif {
				poName := nif.PodName
				poType := nif.Type
				e.updateNetPodBwAll(NetPodRealBw, node, nid, poName, poType, nif.Workload)
			}
		}
	}
}

func (e *NetMetric) updateNetBwAll(metric *prometheus.GaugeVec, node string, nic string, nbw []*utils.Workload) {
	if len(nbw) == 3 {
		e.updateNetBw(metric, node, nic, "GA", "snd", nbw[0].OutBps)
		e.updateNetBw(metric, node, nic, "GA", "rcv", nbw[0].InBps)
		e.updateNetBw(metric, node, nic, "BT", "snd", nbw[1].OutBps)
		e.updateNetBw(metric, node, nic, "BT", "rcv", nbw[1].InBps)
		e.updateNetBw(metric, node, nic, "BE", "snd", nbw[2].OutBps)
		e.updateNetBw(metric, node, nic, "BE", "rcv", nbw[2].InBps)
	}
}

func (e *NetMetric) updateNetPodBwAll(metric *prometheus.GaugeVec, node string, nic string, pod string, wtype string, nif *utils.Workload) {
	if metric != nil && nif != nil {
		klog.V(utils.DBG).Infof("node:%s, nic:%s, pod:%s, wtype:%s, dif:%s", node, nic, pod, wtype, nif)
		e.updatePodBw(metric, node, nic, pod, wtype, "snd", nif.OutBps)
		e.updatePodBw(metric, node, nic, pod, wtype, "rcv", nif.InBps)
	} else {
		klog.V(utils.DBG).Info("updatePodBwAll metric or dif is nil")
	}
}

func (e *NetMetric) updateNetBePodsBw(data *prometheus.GaugeVec, node string, nic string, bw float64) {
	if data == NetBePodsBw {
		bw = math.Max(bw, 0)
	}
	data.WithLabelValues(
		node,
		nic,
	).Set(bw)
}

func (e *NetMetric) updateNetBw(data *prometheus.GaugeVec, node string, nic string, wtype string, rwsum string, bw float64) {
	if data.MetricVec == nil {
		return
	}
	data.WithLabelValues(
		node,
		nic,
		wtype,
		rwsum,
	).Set(bw)
}

func (e *NetMetric) updatePodBw(data *prometheus.GaugeVec, node string, nic string, pod string, wtype string, rw string, bw float64) {
	if data.MetricVec == nil {
		return
	}
	data.WithLabelValues(
		node,
		nic,
		pod,
		wtype,
		rw,
	).Set(bw)
}

func updatePodCache(fromPodCache map[string]PodInfos, toPodCache map[string]PodInfos) {
	for nicId, pif := range fromPodCache {
		pifs := make(map[string]PodInfo)
		for podId, dif := range pif {
			pifs[podId] = dif
		}
		toPodCache[nicId] = pifs
	}
}

func (e *NetMetric) UpdateCache(monitorCache *NetMetric) {
	if monitorCache == nil {
		return
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.Cache = make(map[string]NodeNetBw)
	wlgLen := int(utils.GroupIndex)
	for node, nnbw := range monitorCache.Cache {
		if nnbw == nil {
			return
		}
		e.Cache[node] = make(map[string]*NetBw)
		for nid, nbw := range nnbw {
			if nbw == nil {
				return
			}
			e.Cache[node][nid] = &NetBw{}

			e.Cache[node][nid].NetRequestBw = make([]*utils.Workload, wlgLen)
			e.Cache[node][nid].NetAllocBw = make([]*utils.Workload, wlgLen)
			e.Cache[node][nid].NetTotalBw = make([]*utils.Workload, wlgLen)
			for idx := 0; idx < wlgLen; idx++ {
				e.Cache[node][nid].NetRequestBw[idx] = nbw.NetRequestBw[idx]
				e.Cache[node][nid].NetAllocBw[idx] = nbw.NetAllocBw[idx]
				e.Cache[node][nid].NetTotalBw[idx] = nbw.NetTotalBw[idx]

			}
		}
	}

	e.PodCache = make(map[string]*PodBw)
	for node, npbw := range monitorCache.PodCache {
		if npbw == nil {
			return
		}
		e.PodCache[node] = &PodBw{}
		e.PodCache[node].PodRequestBw = make(map[string]PodInfos)
		updatePodCache(npbw.PodRequestBw, e.PodCache[node].PodRequestBw)
		e.PodCache[node].PodLimitBw = make(map[string]PodInfos)
		updatePodCache(npbw.PodLimitBw, e.PodCache[node].PodLimitBw)
		e.PodCache[node].PodTranBw = make(map[string]PodInfos)
		updatePodCache(npbw.PodTranBw, e.PodCache[node].PodTranBw)
	}
	klog.V(utils.DBG).Info("Network UpdateCache-------------------")
	for node, nnbw := range e.Cache {
		klog.V(utils.DBG).Info(node)
		for did, dbw := range nnbw {
			klog.V(utils.DBG).Info(did, *dbw)
		}
	}
	klog.V(utils.DBG).Info("PodCache")
	for node, npbw := range e.PodCache {
		klog.V(utils.DBG).Info(node, *npbw)
	}
}

func (e *NetMetric) InitMetric() {
	pReg := metrics.GetMetricEngine().PrometheusReg
	pReg.MustRegister(NetPodRequestBw)
	pReg.MustRegister(NetPodLimitBw)
	pReg.MustRegister(NetPodRealBw)
	pReg.MustRegister(NetTotalBw)
	pReg.MustRegister(NetRequestSumBw)
	pReg.MustRegister(NetRequestAllocBw)
	pReg.MustRegister(NetBePodsBw)
}
