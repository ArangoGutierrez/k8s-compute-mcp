// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockHealthChecker implements HealthChecker for testing.
type mockHealthChecker struct {
	pingErr error
}

func (m *mockHealthChecker) Ping(ctx context.Context) error {
	return m.pingErr
}

func TestHealthz_Returns200(t *testing.T) {
	hs := NewHealthServer(":0", &mockHealthChecker{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	hs.handleHealthz(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("GET /healthz status = %q, want %q", body["status"], "ok")
	}
}

func TestReadyz_Returns200_WhenK8sHealthy(t *testing.T) {
	hs := NewHealthServer(":0", &mockHealthChecker{pingErr: nil})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	hs.handleReadyz(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /readyz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("GET /readyz status = %q, want %q", body["status"], "ok")
	}
}

func TestReadyz_Returns503_WhenK8sUnhealthy(t *testing.T) {
	hs := NewHealthServer(":0", &mockHealthChecker{
		pingErr: errors.New("connection refused"),
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	hs.handleReadyz(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "error" {
		t.Fatalf("GET /readyz status = %q, want %q", body["status"], "error")
	}
	if body["message"] == "" {
		t.Fatal("GET /readyz error response should include a message")
	}
}

func TestHealthServer_GracefulShutdown(t *testing.T) {
	hs := NewHealthServer(":0", &mockHealthChecker{})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- hs.Start(ctx)
	}()

	// Give the server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("HealthServer.Start() returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HealthServer.Start() did not return within 5s after context cancellation")
	}
}
