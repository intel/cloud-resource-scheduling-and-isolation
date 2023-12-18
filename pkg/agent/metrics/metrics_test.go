/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetMetricEngine(t *testing.T) {
	engine := GetMetricEngine()
	if engine == nil {
		t.Error("GetMetricEngine returned nil")
	}
}

func TestAddMetric(t *testing.T) {
	engine := GetMetricEngine()

	// Create a mock metric
	mockMetric := &MockMetric{}

	// Add the mock metric to the engine
	engine.AddMetric("mock", mockMetric)

	// Check if the metric was added successfully
	if _, ok := engine.metrics["mock"]; !ok {
		t.Error("Failed to add metric to the engine")
	}
}

// MockMetric is a mock implementation of the IOIMetric interface
type MockMetric struct{}

func (m *MockMetric) Type() string {
	return "mock"
}

func (m *MockMetric) FlushData() {
	// Mock implementation
}

func TestHandler(t *testing.T) {
	engine := GetMetricEngine()

	// Create a mock metric
	mockMetric := &MockMetric{}

	// Add the mock metric to the engine
	engine.AddMetric("mock", mockMetric)

	// Create a request
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Call the handler function
	handler := http.HandlerFunc(engine.handler)
	handler.ServeHTTP(rr, req)

	// Check the response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v", status, http.StatusOK)
	}
}

func TestRouting(t *testing.T) {
	engine := GetMetricEngine()

	// Create a mock metric
	mockMetric := &MockMetric{}

	// Add the mock metric to the engine
	engine.AddMetric("mock", mockMetric)

	metricsTestPort := "8089"
	// Start the routing
	go engine.Routing(metricsTestPort)

	// Wait for the server to start
	time.Sleep(100 * time.Millisecond)

	// Send a request to the /metrics endpoint
	resp, err := http.Get("http://localhost:" + metricsTestPort + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected status code: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestUninitialize(t *testing.T) {
	engine := GetMetricEngine()

	// Call the Uninitialize method
	err := engine.Uninitialize()
	if err != nil {
		t.Errorf("Failed to uninitialize metrics engine: %v", err)
	}

	// Add your assertions here
}
