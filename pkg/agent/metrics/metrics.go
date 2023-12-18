/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package metrics

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/pkg/agent"
)

var (
	TypeMetrics = "Metrics"
	metricsPort = "8080"

	TlsKeyDir  = "/etc/ioi/control/pki/"
	MetricsCrt = "na-server.crt"
	MetricsKey = "na-server.key"
)

var (
	MessageHeader = "Heartbeat IO"
	TypeDiskIO    = "HEARTBEATIO"
)

// MetricsEngine

var metric_engine *MetricsEngine

func GetMetricEngine() *MetricsEngine {
	return metric_engine
}

type MetricsEngine struct {
	agent.IOEngine
	metrics        map[string]IOIMetric
	PrometheusReg  *prometheus.Registry
	LastUpdateTime time.Time
}

type IOIMetric interface {
	Type() string
	FlushData()
}

func (e *MetricsEngine) Type() string {
	return TypeMetrics
}

func (e *MetricsEngine) AddMetric(name string, m IOIMetric) {
	e.metrics[name] = m
}

func (e *MetricsEngine) handler(w http.ResponseWriter, r *http.Request) {
	for _, m := range e.metrics {
		m.FlushData()
	}
	promhttp.HandlerFor(e.PrometheusReg, promhttp.HandlerOpts{Registry: e.PrometheusReg}).ServeHTTP(w, r)
}

func (e *MetricsEngine) healthyHandler(w http.ResponseWriter, r *http.Request) {
	klog.V(5).Infof("heartbeat last update time is %v", e.LastUpdateTime)
	if time.Since(e.LastUpdateTime) < 2*time.Minute {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("healthy"))
	} else {
		klog.Errorf("the heartbeat update time is more than 2 minutes: %v", e.LastUpdateTime)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("event loop failed"))
	}
}

func (e *MetricsEngine) Routing(port string) {
	s := &http.Server{
		Addr:    ":" + port,
		Handler: nil,
	}
	http.HandleFunc("/metrics", e.handler)
	http.HandleFunc("/healthy", e.healthyHandler)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func (e *MetricsEngine) HeartBeat() {
	for {
		data := &agent.HeartBeatData{
			LastUpdateTime: time.Now(),
		}
		agent.HeartBeatChan <- data

		time.Sleep(1 * time.Minute)
	}
}

func (e *MetricsEngine) Initialize(coreClient *kubernetes.Clientset, client *versioned.Clientset, mtls bool) error {
	klog.Info("3.3 initializing the metrics engine")
	go e.Routing(metricsPort)
	go e.HeartBeat()
	return nil
}

func (e *MetricsEngine) ProcessHeartBeat(data agent.EventData) error {
	if data == nil {
		klog.Error("event data is nil")
		return errors.New("event data is nil")
	}
	hData := data.(*agent.HeartBeatData)
	e.LastUpdateTime = hData.LastUpdateTime

	return nil
}

// Uninitialize
func (e *MetricsEngine) Uninitialize() error {
	return nil
}

func init() {
	a := agent.GetAgent()

	engine := &MetricsEngine{
		agent.IOEngine{
			Flag:          0,
			ExecutionFlag: 0,
		},
		make(map[string]IOIMetric),
		prometheus.NewRegistry(),
		time.Now(),
	}

	metric_engine = engine

	a.RegisterEngine(engine)
	a.RegisterEventHandler(agent.HeartBeatChan, agent.EVENT_HEARTBEAT, engine.ProcessHeartBeat, MessageHeader, engine, false)
}
