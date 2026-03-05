// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"net/http"
	"sync"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var initOnce sync.Once

var (
	// ToolRequestsTotal counts total MCP tool invocations by tool name and status.
	ToolRequestsTotal *prometheus.CounterVec

	// ToolDurationSeconds records the duration of MCP tool invocations.
	ToolDurationSeconds *prometheus.HistogramVec

	// ToolErrorsTotal counts MCP tool errors by tool name and error type.
	ToolErrorsTotal *prometheus.CounterVec

	// ActiveJobs tracks currently active Kubernetes jobs by namespace and type.
	ActiveJobs *prometheus.GaugeVec
)

// InitMetrics registers all metrics with the default Prometheus registerer.
// Safe to call multiple times; registration happens only once.
func InitMetrics() {
	initOnce.Do(func() {
		InitMetricsWithRegistry(nil)
	})
}

// InitMetricsWithRegistry registers all metrics with the given registry.
// If reg is nil, metrics are registered with prometheus.DefaultRegisterer.
func InitMetricsWithRegistry(reg prometheus.Registerer) {
	ToolRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcp_tool_requests_total",
		Help: "Total number of MCP tool invocations",
	}, []string{"tool", "status"})

	ToolDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mcp_tool_duration_seconds",
		Help:    "Duration of MCP tool invocations",
		Buckets: prometheus.DefBuckets,
	}, []string{"tool"})

	ToolErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcp_tool_errors_total",
		Help: "Total number of MCP tool errors",
	}, []string{"tool", "error_type"})

	ActiveJobs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcp_active_jobs",
		Help: "Number of currently active Kubernetes jobs",
	}, []string{"namespace", "type"})

	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	reg.MustRegister(ToolRequestsTotal)
	reg.MustRegister(ToolDurationSeconds)
	reg.MustRegister(ToolErrorsTotal)
	reg.MustRegister(ActiveJobs)

	// Initialize zero-valued label combinations so metrics appear in /metrics output
	// even before any observations occur. This aids dashboard setup and alerting.
	ToolErrorsTotal.WithLabelValues("", "handler_error")
	ActiveJobs.WithLabelValues("", "mpijob")
	ActiveJobs.WithLabelValues("", "jobset")
	ActiveJobs.WithLabelValues("", "job")
}

// InstrumentToolCall wraps a tool call function with metrics instrumentation.
// It records request count, duration, and errors without altering behavior.
func InstrumentToolCall(toolName string, fn func() (*mcpgo.CallToolResult, error)) (*mcpgo.CallToolResult, error) {
	start := time.Now()

	result, err := fn()

	duration := time.Since(start).Seconds()
	ToolDurationSeconds.WithLabelValues(toolName).Observe(duration)

	if err != nil {
		ToolRequestsTotal.WithLabelValues(toolName, "error").Inc()
		ToolErrorsTotal.WithLabelValues(toolName, "handler_error").Inc()
		return result, err
	}

	ToolRequestsTotal.WithLabelValues(toolName, "success").Inc()
	return result, nil
}

// RegisterMetricsHandler registers the /metrics endpoint on the given mux.
// Uses the provided registry for metric collection.
func RegisterMetricsHandler(mux *http.ServeMux, reg prometheus.Gatherer) {
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
}
