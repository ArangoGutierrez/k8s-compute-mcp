// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"
)

// HealthChecker abstracts the K8s connectivity check for testability.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthServer serves HTTP health endpoints on the configured address.
type HealthServer struct {
	addr    string
	checker HealthChecker
	server  *http.Server
}

// NewHealthServer creates a new HealthServer.
func NewHealthServer(addr string, checker HealthChecker) *HealthServer {
	hs := &HealthServer{
		addr:    addr,
		checker: checker,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", hs.handleHealthz)
	mux.HandleFunc("/readyz", hs.handleReadyz)

	hs.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return hs
}

// Start starts the HTTP server and blocks until ctx is canceled.
// On context cancellation it performs a graceful shutdown.
func (h *HealthServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", h.addr)
	if err != nil {
		return err
	}
	return h.StartWithListener(ctx, ln)
}

// StartWithListener starts the HTTP server on the provided listener.
// This is useful for testing where the caller controls the listener.
func (h *HealthServer) StartWithListener(ctx context.Context, ln net.Listener) error {
	log.Printf("Health server listening on %s", ln.Addr().String())

	// Shut down gracefully when context is canceled.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Health server shutdown error: %v", err)
		}
	}()

	if err := h.server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (h *HealthServer) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Failed to encode healthz response: %v", err)
	}
}

func (h *HealthServer) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := h.checker.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		}); err != nil {
			log.Printf("Failed to encode readyz error response: %v", err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Failed to encode readyz response: %v", err)
	}
}
