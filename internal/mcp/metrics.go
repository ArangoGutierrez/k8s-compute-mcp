// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/prometheus/client_golang/prometheus"
)

// RED phase stub: metric variables declared but not implemented.

var (
	ToolRequestsTotal   *prometheus.CounterVec
	ToolDurationSeconds *prometheus.HistogramVec
	ToolErrorsTotal     *prometheus.CounterVec
	ActiveJobs          *prometheus.GaugeVec
)

// InitMetricsWithRegistry registers metrics with the given registry.
func InitMetricsWithRegistry(_ *prometheus.Registry) {
	// TODO: implement
}

// InstrumentToolCall wraps a tool call function with metrics instrumentation.
func InstrumentToolCall(_ string, fn func() (*mcp.CallToolResult, error)) (*mcp.CallToolResult, error) {
	// TODO: implement — just pass through for now
	return fn()
}

// RegisterMetricsHandler registers the /metrics endpoint on the given mux.
func RegisterMetricsHandler(_ *http.ServeMux, _ *prometheus.Registry) {
	// TODO: implement
}
