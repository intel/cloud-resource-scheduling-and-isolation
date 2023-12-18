/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package disk

import (
	"math"
	"sync"

	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/IOIsolation/pkg/agent/common"
	"sigs.k8s.io/IOIsolation/pkg/agent/metrics"
)

var (
	BlockSizes = []string{"512B", "1k", "4k", "8k", "16k", "32k"}
	/* disk metrics */
	// pod level
	PodRequestBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pods_request_bw",
			Help: "Requested bw of running pods.",
		},
		[]string{"node", "disk", "pod", "wtype", "rw", "bs"},
	)
	PodLimitBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pods_limit_bw",
			Help: "Limited bw of running pods.",
		},
		[]string{"node", "disk", "pod", "wtype", "rw", "bs"},
	)
	PodRawBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pods_real_untran_bw",
			Help: "Bw of real running pods with different block size.",
		},
		[]string{"node", "disk", "pod", "wtype", "rw", "bs"},
	)
	PodTranBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pods_real_tran_bw",
			Help: "Bw of real running pods with transferred block size.",
		},
		[]string{"node", "disk", "pod", "wtype", "rw", "bs"},
	)
	// disk level
	DiskTotalBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disks_total_bw",
			Help: "Total bw of disks.",
		},
		[]string{"node", "disk", "wtype", "rwsum"},
	)
	DiskRequestSumBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disks_request_sum_bw",
			Help: "The sum of pods requested bw.",
		},
		[]string{"node", "disk", "wtype", "rwsum"},
	)
	DiskSchedAllocBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disks_sched_alloc_bw",
			Help: "The allocatable sched bw.",
		},
		[]string{"node", "disk", "wtype", "rwsum"},
	)
	BePodsBw = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "be_pods_bw",
			Help: "The min be pod bw.",
		},
		[]string{"node", "disk", "rwsum"},
	)
)

type NodeDiskBw map[string]*DiskBw // diskId

type DiskMetric struct {
	mutex    sync.Mutex
	Cache    map[string]NodeDiskBw // nodeName
	PodCache map[string]*PodBw     // nodeName
}

type DiskBw struct {
	DiskRequestBw []*utils.Workload // GA, BT, BE
	DiskAllocBw   []*utils.Workload
	DiskTotalBw   []*utils.Workload
}

type PodBw struct {
	PodRequestBw map[string]PodInfos
	PodLimitBw   map[string]PodInfos // diskId
	PodTranBw    map[string]PodInfos
	PodRawBw     map[string]PodInfos
}

type PodInfos map[string]PodInfo //podId

type PodInfo struct {
	PodName  string
	Type     string
	Workload *utils.Workload
	WlBs     *map[string]*utils.Workload
}

func (e *DiskMetric) Type() string {
	return "Disk"
}

func (e *DiskMetric) updateDiskBePodsBw(data *prometheus.GaugeVec, node string, disk string, bw float64) {
	if data == BePodsBw {
		bw = math.Max(bw, 0)
	}
	data.WithLabelValues(
		node,
		disk,
		"read",
	).Set(bw)

	data.WithLabelValues(
		node,
		disk,
		"write",
	).Set(bw)

	data.WithLabelValues(
		node,
		disk,
		"total",
	).Set(bw * 2)
}

func (e *DiskMetric) updateDiskBw(data *prometheus.GaugeVec, node string, disk string, wtype string, rwsum string, bw float64) {
	if data == DiskSchedAllocBw {
		bw = math.Max(bw, 0)
	}
	data.WithLabelValues(
		node,
		disk,
		wtype,
		rwsum,
	).Set(bw)
}

func (e *DiskMetric) updateDiskBwAll(metric *prometheus.GaugeVec, node string, did string, dbw []*utils.Workload) {
	if len(dbw) == 3 {
		e.updateDiskBw(metric, node, did, "GA", "read", dbw[0].InBps)
		e.updateDiskBw(metric, node, did, "GA", "write", dbw[0].OutBps)
		e.updateDiskBw(metric, node, did, "GA", "total", dbw[0].TotalBps)
		e.updateDiskBw(metric, node, did, "BT", "read", dbw[1].InBps)
		e.updateDiskBw(metric, node, did, "BT", "write", dbw[1].OutBps)
		e.updateDiskBw(metric, node, did, "BT", "total", dbw[1].TotalBps)
		e.updateDiskBw(metric, node, did, "BE", "read", dbw[2].InBps)
		e.updateDiskBw(metric, node, did, "BE", "write", dbw[2].OutBps)
		e.updateDiskBw(metric, node, did, "BE", "total", dbw[2].TotalBps)
	}
}

func (e *DiskMetric) updatePodBw(data *prometheus.GaugeVec, node string, disk string, pod string, wtype string, rw string, bs string, bw float64) {
	if data != nil {
		data.WithLabelValues(
			node,
			disk,
			pod,
			wtype,
			rw,
			bs,
		).Set(bw)
	} else {
		klog.Warning("Prometheus GaugeVec is nil")
	}
}

func (e *DiskMetric) updatePodBwAll(metric *prometheus.GaugeVec, node string, disk string, pod string, wtype string, bs string, dif *utils.Workload) {
	if dif != nil {
		klog.V(utils.DBG).Infof("node:%s, disk:%s, pod:%s, wtype:%s, bs:%s, dif:%v", node, disk, pod, wtype, bs, *dif)
		e.updatePodBw(metric, node, disk, pod, wtype, "read", bs, dif.InBps)
		e.updatePodBw(metric, node, disk, pod, wtype, "write", bs, dif.OutBps)
		if metric == PodTranBw || metric == PodRawBw {
			e.updatePodBw(metric, node, disk, pod, wtype, "total", bs, dif.TotalBps)
		}
	} else {
		klog.V(utils.DBG).Info("updatePodBwAll dif is nil")
	}
}

func (e *DiskMetric) FlushData() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// agent
	DiskRequestSumBw.Reset()
	DiskSchedAllocBw.Reset()
	DiskTotalBw.Reset()
	BePodsBw.Reset()
	for node, ndbw := range e.Cache {
		for did, dbw := range ndbw {
			e.updateDiskBwAll(DiskRequestSumBw, node, did, dbw.DiskRequestBw)
			e.updateDiskBwAll(DiskSchedAllocBw, node, did, dbw.DiskAllocBw)
			e.updateDiskBwAll(DiskTotalBw, node, did, dbw.DiskTotalBw)
		}
	}
	PodRequestBw.Reset()
	PodLimitBw.Reset()
	PodTranBw.Reset()
	PodRawBw.Reset()
	for node, npbw := range e.PodCache {
		for did, pif := range npbw.PodRequestBw {
			bePodNum := 0
			for _, dbs := range pif {
				bs := "32k"
				poName := dbs.PodName
				poType := dbs.Type
				e.updatePodBwAll(PodRequestBw, node, did, poName, poType, bs, dbs.Workload)
				if poType == utils.BE_Type {
					bePodNum = bePodNum + 1
				}
			}
			e.updateDiskBePodsBw(BePodsBw, node, did, common.MinPodDiskInBw*float64(bePodNum))
		}
		for did, pif := range npbw.PodLimitBw {
			for _, dbs := range pif {
				bs := "32k"
				poName := dbs.PodName
				poType := dbs.Type
				e.updatePodBwAll(PodLimitBw, node, did, poName, poType, bs, dbs.Workload)
			}
		}
		for did, pif := range npbw.PodTranBw {
			for _, dbs := range pif {
				bs := "32k"
				poName := dbs.PodName
				poType := dbs.Type
				e.updatePodBwAll(PodTranBw, node, did, poName, poType, bs, dbs.Workload)
			}
		}
		// client
		for did, pif := range npbw.PodRawBw {
			for _, dbs := range pif {
				poType := dbs.Type
				poName := dbs.PodName
				if dbs.WlBs != nil {
					for bs, dif := range *dbs.WlBs {
						e.updatePodBwAll(PodRawBw, node, did, poName, poType, bs, dif)
					}
				} else {
					bs := "32k"
					e.updatePodBwAll(PodRawBw, node, did, poName, poType, bs, dbs.Workload)
				}
			}
		}
	}
}

func (e *DiskMetric) InitMetric() {
	metrics.GetMetricEngine().PrometheusReg.MustRegister(PodRequestBw)
	metrics.GetMetricEngine().PrometheusReg.MustRegister(PodLimitBw)
	metrics.GetMetricEngine().PrometheusReg.MustRegister(PodRawBw)
	metrics.GetMetricEngine().PrometheusReg.MustRegister(PodTranBw)
	metrics.GetMetricEngine().PrometheusReg.MustRegister(DiskTotalBw)
	metrics.GetMetricEngine().PrometheusReg.MustRegister(DiskRequestSumBw)
	metrics.GetMetricEngine().PrometheusReg.MustRegister(DiskSchedAllocBw)
	metrics.GetMetricEngine().PrometheusReg.MustRegister(BePodsBw)
}

func updatePodCache(fromPodCache map[string]PodInfos, toPodCache map[string]PodInfos) {
	for diskId, pif := range fromPodCache {
		pifs := make(map[string]PodInfo)
		for podId, dif := range pif {
			pifs[podId] = dif
		}
		toPodCache[diskId] = pifs
	}
}

func (e *DiskMetric) UpdateCache(monitorCache *DiskMetric) {
	if monitorCache == nil {
		return
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// update cache with data pass from node agent
	e.Cache = make(map[string]NodeDiskBw)
	wlgLen := int(utils.GroupIndex)
	for node, ndbw := range monitorCache.Cache {
		if ndbw == nil {
			return
		}
		e.Cache[node] = make(map[string]*DiskBw)
		for did, dbw := range ndbw {
			if dbw == nil {
				return
			}
			e.Cache[node][did] = &DiskBw{}
			e.Cache[node][did].DiskRequestBw = make([]*utils.Workload, wlgLen)
			e.Cache[node][did].DiskAllocBw = make([]*utils.Workload, wlgLen)
			e.Cache[node][did].DiskTotalBw = make([]*utils.Workload, wlgLen)

			for idx := 0; idx < wlgLen; idx++ {
				e.Cache[node][did].DiskRequestBw[idx] = dbw.DiskRequestBw[idx]
				e.Cache[node][did].DiskAllocBw[idx] = dbw.DiskAllocBw[idx]
				e.Cache[node][did].DiskTotalBw[idx] = dbw.DiskTotalBw[idx]
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
		// update from service
		e.PodCache[node].PodRawBw = make(map[string]PodInfos)
		updatePodCache(npbw.PodRawBw, e.PodCache[node].PodRawBw)
	}
	klog.V(utils.DBG).Info("Disk UpdateCache-------------------")
	for node, ndbw := range e.Cache {
		klog.V(utils.DBG).Info(node)
		for did, dbw := range ndbw {
			klog.V(utils.DBG).Info(did, *dbw)
		}
	}
	klog.V(utils.DBG).Info("PodCache")
	for node, npbw := range e.PodCache {
		klog.V(utils.DBG).Info(node, *npbw)
	}
}
