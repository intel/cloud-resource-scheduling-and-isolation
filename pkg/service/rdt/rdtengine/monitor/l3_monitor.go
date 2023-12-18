/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package monitor

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"
	"sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/utils"
)

var (
	RdtPodL3Usage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rdt_pods_l3_usage",
			Help: "rdt l3 usage of running pods.",
		},
		[]string{"node", "pod", "namespace", "podname"},
	)

	TotPodL3Usage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rdt_tot_l3_usage",
			Help: "total rdt l3 usage.",
		},
		[]string{"node"},
	)
)

const (
	L3_Monitor = "l3_monitor"
)

func init() {
	rdt.RegisterMonitor(L3_Monitor, l3MonitorNewer)
}

type l3Config struct {
	Interval int
}

type L3Monitor struct {
	rdt.BaseObject
	Type     string
	Interval int
}

func (e *L3Monitor) Initialize() error {
	return nil
}

func (e *L3Monitor) Start() error {
	rdt.Reg.MustRegister(RdtPodL3Usage)
	rdt.Reg.MustRegister(TotPodL3Usage)
	for {
		monPath := filepath.Join(utils.ResctrlRoot, "mon_data")
		files, err := os.ReadDir(monPath)
		if err != nil {
			return err
		}

		var l3 uint64
		for _, file := range files {
			name := file.Name()
			if strings.HasPrefix(name, "mon_L3_") {
				now, err := readFileUint64(filepath.Join(monPath, name, "llc_occupancy"))
				if err != nil {
					return fmt.Errorf("failed to read monitor data llc_occupancy: %v", err)
				}
				l3 += now
			}
		}
		TotPodL3Usage.WithLabelValues(rdt.Hostname).Set(float64(l3))
	}
	return nil
}

func (e *L3Monitor) Stop() error {
	return nil
}

func l3MonitorNewer(eng rdt.ResourceHandler, config string) (rdt.Monitor, error) {
	var l3Config l3Config
	err := yaml.Unmarshal([]byte(config), &l3Config)
	log.Print(l3Config)
	if err != nil {
		klog.Fatal("error: %s", err)
		return &L3Monitor{}, err
	}

	e := L3Monitor{
		rdt.BaseObject{
			RH: eng,
		},
		L3_Monitor,
		l3Config.Interval,
	}

	return &e, nil
}

func (e *L3Monitor) GetType() string {
	return e.Type
}

func (e *L3Monitor) OnEvent(data rdt.EventData) (rdt.EventResponse, error) {
	t := data.GetEventType()

	switch t {
	case rdt.EventTypeMonitorGroup:
		evt := data.(*rdt.MonitorGroupEvent)
		klog.Infof("Monitor Receive MonitorGroupEvent: %s", evt.Id)
		go e.monitor(evt.Id)
	}
	return nil, nil
}

func (m *L3Monitor) monitor(id string) {
	for {
		l3Bytes, err := m.getMonL3CacheUsage(id)
		if err != nil {
			klog.Errorf("failed to get monitor data: %v", err)
			time.Sleep(time.Duration(m.Interval) * time.Second)
			continue
		}

		klog.Infof("l3 monitor l3 usage is %v bytes", float64(l3Bytes))
		m.RH.ReportData(id, &L3Data{
			L3Cache: float64(l3Bytes),
		})

		//app, err := m.RH.GetApp(id)
		//if err != nil {
		//	klog.Errorf("err is %v, state is %v", err, app)
		//	time.Sleep(time.Duration(m.Interval) * time.Second)
		//	continue
		//}
		//
		//var pod rdt.PodMeta
		//err = utils.Unmarshal(app.Spec.Parameters, &pod)
		//if err != nil {
		//	klog.Errorf("unmarshal err %v", err)
		//}
		//
		//if app.Status.State == -1 {
		//	RdtPodL3Usage.DeleteLabelValues(rdt.Hostname, id, pod.Namespace, pod.PodName)
		//	return
		//}
		//
		//l3Bytes, err := m.getMonL3CacheUsage(id)
		//if err != nil {
		//	klog.Errorf("failed to get monitor data: %v", err)
		//	continue
		//}
		//
		//klog.Infof("l3 monitor l3 usage is %v bytes", float64(l3Bytes))
		//RdtPodL3Usage.WithLabelValues(rdt.Hostname, id, pod.Namespace, pod.PodName).Set(float64(l3Bytes))
		//m.RH.ReportData(id, &L3Data{
		//	l3Cache: float64(l3Bytes),
		//})
		time.Sleep(time.Duration(m.Interval) * time.Second)
	}
}

// return l3 cache bytes
func (m *L3Monitor) getMonL3CacheUsage(id string) (uint64, error) {
	group, err := m.RH.GetMonitorGroup(id)
	if err != nil {
		return 0, fmt.Errorf("failed to get monitor group: %v", err)
	}
	monPath := filepath.Join(utils.GetMonitorGroupPath(group), "mon_data")
	files, err := os.ReadDir(monPath)
	if err != nil {
		return 0, err
	}
	var l3 uint64
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "mon_L3_") {
			now, err := readFileUint64(filepath.Join(monPath, name, "llc_occupancy"))
			if err != nil {
				return 0, fmt.Errorf("failed to read monitor data llc_occupancy: %v", err)
			}
			l3 += now
		}
	}

	klog.Infof("get l3 cache usage %v", l3)
	return l3, nil
}

type L3Data struct {
	L3Cache float64
}

func (m *L3Data) ToString() string {
	return fmt.Sprintf("l3CacheUsage: %v", m.L3Cache)
}

func (m *L3Data) Key() string {
	return L3_Monitor
}

func (l *L3Monitor) EventFilter() []string {
	return []string{""}
}
