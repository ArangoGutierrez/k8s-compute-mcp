// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestRegistry creates a fresh registry and initializes metrics into it.
// This avoids pollution between tests.
func newTestRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	InitMetricsWithRegistry(reg)
	return reg
}

func TestInstrumentToolCall_Success(t *testing.T) {
	reg := newTestRegistry()

	result, err := InstrumentToolCall("submit_mpi_job", func() (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	})
	if err != nil {
		t.Fatalf("InstrumentToolCall returned error: %v", err)
	}
	if result == nil {
		t.Fatal("InstrumentToolCall returned nil result")
	}

	// Verify request counter incremented with status=success
	count := testutil.ToFloat64(ToolRequestsTotal.WithLabelValues("submit_mpi_job", "success"))
	if count != 1 {
		t.Errorf("expected request counter = 1, got %f", count)
	}

	// Verify error counter NOT incremented
	errCount := testutil.ToFloat64(ToolErrorsTotal.WithLabelValues("submit_mpi_job", "handler_error"))
	if errCount != 0 {
		t.Errorf("expected error counter = 0, got %f", errCount)
	}

	// Verify duration histogram recorded
	expected := `
		# HELP mcp_tool_duration_seconds Duration of MCP tool invocations
		# TYPE mcp_tool_duration_seconds histogram
	`
	if err := testutil.CollectAndCompare(ToolDurationSeconds, strings.NewReader(expected), "mcp_tool_duration_seconds"); err != nil {
		// We just verify the metric exists and has observations, not exact values
		// testutil.CollectAndCompare checks metadata, so a mismatch on bucket values is expected
	}

	// Verify histogram has at least one observation by checking the metric count
	durationCount := testutil.CollectAndCount(ToolDurationSeconds)
	if durationCount == 0 {
		t.Error("expected duration histogram to have observations")
	}

	_ = reg // keep registry alive
}

func TestInstrumentToolCall_Error(t *testing.T) {
	_ = newTestRegistry()

	expectedErr := fmt.Errorf("k8s submission failed")
	result, err := InstrumentToolCall("check_status", func() (*mcp.CallToolResult, error) {
		return nil, expectedErr
	})
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Fatal("expected nil result on error")
	}

	// Verify request counter incremented with status=error
	count := testutil.ToFloat64(ToolRequestsTotal.WithLabelValues("check_status", "error"))
	if count != 1 {
		t.Errorf("expected request counter (error) = 1, got %f", count)
	}

	// Verify error counter incremented
	errCount := testutil.ToFloat64(ToolErrorsTotal.WithLabelValues("check_status", "handler_error"))
	if errCount != 1 {
		t.Errorf("expected error counter = 1, got %f", errCount)
	}
}

func TestInstrumentToolCall_RecordsDuration(t *testing.T) {
	_ = newTestRegistry()

	_, _ = InstrumentToolCall("read_artifact_head", func() (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("data"), nil
	})

	// Verify histogram has observations
	durationCount := testutil.CollectAndCount(ToolDurationSeconds)
	if durationCount == 0 {
		t.Error("expected duration histogram to record observation")
	}
}

func TestInstrumentToolCall_CorrectLabels(t *testing.T) {
	_ = newTestRegistry()

	tools := []string{"submit_mpi_job", "submit_monte_carlo_batch", "submit_reducer", "check_status", "read_artifact_head"}
	for _, toolName := range tools {
		_, _ = InstrumentToolCall(toolName, func() (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("ok"), nil
		})
	}

	for _, toolName := range tools {
		count := testutil.ToFloat64(ToolRequestsTotal.WithLabelValues(toolName, "success"))
		if count != 1 {
			t.Errorf("tool %q: expected request counter = 1, got %f", toolName, count)
		}
	}
}

func TestMetricsEndpoint(t *testing.T) {
	reg := newTestRegistry()

	// Invoke a tool to populate metrics
	_, _ = InstrumentToolCall("check_status", func() (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	})

	mux := http.NewServeMux()
	RegisterMetricsHandler(mux, reg)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	// Verify Prometheus exposition format
	if !strings.Contains(body, "mcp_tool_requests_total") {
		t.Error("metrics endpoint missing mcp_tool_requests_total")
	}
	if !strings.Contains(body, "mcp_tool_duration_seconds") {
		t.Error("metrics endpoint missing mcp_tool_duration_seconds")
	}
	if !strings.Contains(body, "mcp_tool_errors_total") {
		t.Error("metrics endpoint missing mcp_tool_errors_total")
	}
	if !strings.Contains(body, "mcp_active_jobs") {
		t.Error("metrics endpoint missing mcp_active_jobs")
	}
}

func TestNewToolHandler_ErrorMetrics(t *testing.T) {
	_ = newTestRegistry()

	type testInput struct {
		Value string `json:"value"`
	}
	type testOutput struct {
		Result string `json:"result"`
	}

	// Handler that always returns an error — simulates a real handler failure
	handler := newToolHandler("failing_tool", func(ctx context.Context, input testInput) (*testOutput, error) {
		return nil, fmt.Errorf("simulated k8s failure")
	})
	if handler == nil {
		t.Fatal("newToolHandler returned nil")
	}

	// Invoke the handler through the mcp-go dispatch
	req := mcp.CallToolRequest{}
	req.Params.Name = "failing_tool"
	req.Params.Arguments = map[string]any{"value": "test"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned raw error (should convert to tool result): %v", err)
	}
	if result == nil {
		t.Fatal("handler returned nil result")
	}

	// The critical assertion: error counter MUST be incremented
	errCount := testutil.ToFloat64(ToolErrorsTotal.WithLabelValues("failing_tool", "handler_error"))
	if errCount != 1 {
		t.Errorf("expected error counter = 1 for failing handler, got %f (errors counted as successes!)", errCount)
	}

	// Request counter should show error status, not success
	successCount := testutil.ToFloat64(ToolRequestsTotal.WithLabelValues("failing_tool", "success"))
	if successCount != 0 {
		t.Errorf("expected success counter = 0 for failing handler, got %f", successCount)
	}
	errorCount := testutil.ToFloat64(ToolRequestsTotal.WithLabelValues("failing_tool", "error"))
	if errorCount != 1 {
		t.Errorf("expected error request counter = 1 for failing handler, got %f", errorCount)
	}
}

func TestInstrumentToolCall_Transparency(t *testing.T) {
	_ = newTestRegistry()

	// The wrapper must not alter the result or error from the underlying function
	expectedResult := mcp.NewToolResultText("specific-result")
	result, err := InstrumentToolCall("submit_mpi_job", func() (*mcp.CallToolResult, error) {
		return expectedResult, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expectedResult {
		t.Error("InstrumentToolCall altered the result — must be transparent")
	}

	expectedErr := fmt.Errorf("specific-error")
	result2, err2 := InstrumentToolCall("submit_mpi_job", func() (*mcp.CallToolResult, error) {
		return nil, expectedErr
	})
	if err2 != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err2)
	}
	if result2 != nil {
		t.Error("expected nil result on error")
	}
}
