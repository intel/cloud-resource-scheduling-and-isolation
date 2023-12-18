/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package monitor

import (
	//	"fmt"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	rdt "sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine"
	"sigs.k8s.io/IOIsolation/pkg/service/rdt/rdtengine/utils"
)

var (
	RdtPodMBUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rdt_pods_mb_usage",
			Help: "rdt l3 usage of running pods.",
		},
		[]string{"node", "podid", "namespace", "name", "ln"},
	)

	TotPodMBUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rdt_tot_mb_usage",
			Help: "rdt l3 usage of running pods.",
		},
		[]string{"node", "totbytes"},
	)
)

const (
	Mb_Monitor = "mb_monitor"
)

func init() {
	rdt.RegisterMonitor(Mb_Monitor, mbMonitorNewer)
}

type mbConfig struct {
	Interval int
}

type MbMonitor struct {
	rdt.BaseObject
	Type     string
	Interval int
}

func (e *MbMonitor) Initialize() error {
	return nil
}

func (e *MbMonitor) Start() error {
	rdt.Reg.MustRegister(RdtPodMBUsage)
	rdt.Reg.MustRegister(TotPodMBUsage)
	for {
		monPath := filepath.Join(utils.ResctrlRoot, "mon_data")
		files, err := os.ReadDir(monPath)
		if err != nil {
			return err
		}
		var tmbBytes uint64
		var lmbBytes uint64
		for _, file := range files {
			name := file.Name()
			if strings.HasPrefix(name, "mon_L3_") {
				told, err := readFileUint64(filepath.Join(monPath, name, "mbm_total_bytes"))
				if err != nil {
					return fmt.Errorf("failed to read monitor data mbm_total_bytes: %v", err)
				}
				lold, err := readFileUint64(filepath.Join(monPath, name, "mbm_local_bytes"))
				if err != nil {
					return fmt.Errorf("failed to read monitor data: mbm_local_bytes %v", err)
				}
				time.Sleep(1 * time.Second)
				tnew, err := readFileUint64(filepath.Join(monPath, name, "mbm_total_bytes"))
				if err != nil {
					return fmt.Errorf("failed to read monitor data mbm_total_bytes: %v", err)
				}
				lnew, err := readFileUint64(filepath.Join(monPath, name, "mbm_local_bytes"))
				if err != nil {
					return fmt.Errorf("failed to read monitor data mbm_local_bytes: %v", err)
				}
				tmbBytes += (tnew - told)
				lmbBytes += (lnew - lold)
			}
		}

		TotPodMBUsage.WithLabelValues(rdt.Hostname, "LocalMb").Set(float64(lmbBytes) / 1024 / 1024)
		TotPodMBUsage.WithLabelValues(rdt.Hostname, "TotalMb").Set(float64(tmbBytes) / 1024 / 1024)
		klog.Infof("mb monitor node level localMB is %v MB, totalMB is %v MB", float64(lmbBytes)/1024/1024, float64(tmbBytes)/1024/1024)

		time.Sleep(time.Second)
	}
}

func (e *MbMonitor) Stop() error {
	return nil
}

func mbMonitorNewer(eng rdt.ResourceHandler, config string) (rdt.Monitor, error) {
	var mbConfig mbConfig
	err := yaml.Unmarshal([]byte(config), &mbConfig)
	log.Print(mbConfig)
	if err != nil {
		klog.Fatal("error: %s", err)
		return &MbMonitor{}, err
	}

	e := MbMonitor{
		rdt.BaseObject{
			RH: eng,
		},
		Mb_Monitor,
		mbConfig.Interval,
	}

	return &e, nil
}

func (e *MbMonitor) GetType() string {
	return e.Type
}

func (e *MbMonitor) OnEvent(data rdt.EventData) (rdt.EventResponse, error) {
	t := data.GetEventType()

	switch t {
	case rdt.EventTypeMonitorGroup:
		evt := data.(*rdt.MonitorGroupEvent)
		klog.Infof("Monitor Receive MonitorGroupEvent: %s", evt.Id)
		go e.monitor(evt.Id)
	}
	return nil, nil
}

func (m *MbMonitor) monitor(id string) {
	for {
		totalBps, localBps, err := m.getMonMbUsage(id)
		if err != nil {
			klog.Errorf("failed to get monitor data: %v", err)
			time.Sleep(time.Duration(m.Interval) * time.Second)
			continue
		}
		m.RH.ReportData(id, &MbData{
			LocalMb: float64(localBps) / 1024 / 1024, // Bps to MBps
			TotalMb: float64(totalBps) / 1024 / 1024,
		})
		time.Sleep(time.Duration(m.Interval) * time.Second)
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
		//	RdtPodMBUsage.DeleteLabelValues(rdt.Hostname, id, pod.Namespace, pod.PodName, "LocalMb")
		//	RdtPodMBUsage.DeleteLabelValues(rdt.Hostname, id, pod.Namespace, pod.PodName, "TotalMb")
		//	return
		//}
		//
		//totalBps, localBps, err := m.getMonMbUsage(id)
		//if err != nil {
		//	klog.Errorf("failed to get monitor data: %v", err)
		//	continue
		//}
		//
		//klog.Infof("mb monitor localMB is %v MB/s, totalMB is %v MB/s", float64(localBps)/1024/1024, float64(totalBps)/1024/1024)
		//RdtPodMBUsage.WithLabelValues(rdt.Hostname, id, pod.Namespace, pod.PodName, "LocalMb").Set(float64(localBps) / 1024 / 1024)
		//RdtPodMBUsage.WithLabelValues(rdt.Hostname, id, pod.Namespace, pod.PodName, "TotalMb").Set(float64(totalBps) / 1024 / 1024)
		//
		//m.RH.ReportData(id, &MbData{
		//	LocalMb: float64(localBps) / 1024 / 1024, // Bps to MBps
		//	TotalMb: float64(totalBps) / 1024 / 1024,
		//})
		//time.Sleep(time.Duration(m.Interval) * time.Second)
	}
}

// return mb bps
func (m *MbMonitor) getMonMbUsage(id string) (uint64, uint64, error) {
	group, err := m.RH.GetMonitorGroup(id)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get monitor group: %v", err)
	}
	monPath := filepath.Join(utils.GetMonitorGroupPath(group), "mon_data")
	files, err := os.ReadDir(monPath)
	if err != nil {
		return 0, 0, err
	}
	var tmbBytes uint64
	var lmbBytes uint64
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "mon_L3_") {
			told, err := readFileUint64(filepath.Join(monPath, name, "mbm_total_bytes"))
			if err != nil {
				return 0, 0, fmt.Errorf("failed to read monitor data mbm_total_bytes: %v", err)
			}
			lold, err := readFileUint64(filepath.Join(monPath, name, "mbm_local_bytes"))
			if err != nil {
				return 0, 0, fmt.Errorf("failed to read monitor data: mbm_local_bytes %v", err)
			}
			time.Sleep(1 * time.Second)
			tnew, err := readFileUint64(filepath.Join(monPath, name, "mbm_total_bytes"))
			if err != nil {
				return 0, 0, fmt.Errorf("failed to read monitor data mbm_total_bytes: %v", err)
			}
			lnew, err := readFileUint64(filepath.Join(monPath, name, "mbm_local_bytes"))
			if err != nil {
				return 0, 0, fmt.Errorf("failed to read monitor data mbm_local_bytes: %v", err)
			}
			tmbBytes += (tnew - told)
			lmbBytes += (lnew - lold)
		}
	}

	return tmbBytes, lmbBytes, nil
}

func readFileUint64(path string) (uint64, error) {
	data, err := readFileString(path)
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(data, 10, 64)
}

func readFileString(path string) (string, error) {
	data, err := os.ReadFile(path)
	return strings.TrimSpace(string(data)), err
}

type MbData struct {
	LocalMb float64
	TotalMb float64
}

func (m *MbData) ToString() string {
	return fmt.Sprintf("mbUsage: LocalMb %v, TotalMb %v", m.LocalMb, m.TotalMb)
}

func (m *MbData) Key() string {
	return Mb_Monitor
}

func (l *MbMonitor) EventFilter() []string {
	return []string{""}
}
