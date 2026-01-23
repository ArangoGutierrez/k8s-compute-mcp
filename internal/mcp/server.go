// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

// Package mcp provides the MCP (Model Context Protocol) server implementation.
//
// The server exposes tools for submitting computational workloads to Kubernetes:
//   - submit_mpi_job: Submit MPI workloads via Kubeflow MPI Operator
//   - submit_monte_carlo_batch: Submit parallel simulations via JobSet
//   - submit_reducer: Submit aggregation jobs
//   - check_status: Query job/workload status
//   - read_artifact_head: Read result files from shared storage
package mcp

import (
	"context"
	"fmt"
	"log"

	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/info"
	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/k8s"
)

// Server represents the MCP server instance.
type Server struct {
	k8sClient *k8s.Client
}

// NewServer creates a new MCP server instance.
//
// The K8s client is configured via environment variables:
//   - KUBECONFIG: Path to kubeconfig file (default: ~/.kube/config)
//   - KUBE_CONTEXT: Kubernetes context to use (default: current context)
func NewServer() (*Server, error) {
	// Initialize K8s client using environment configuration
	client, err := k8s.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	return &Server{
		k8sClient: client,
	}, nil
}

// Run starts the MCP server with stdio transport.
// It blocks until the context is cancelled or an error occurs.
func (s *Server) Run(ctx context.Context) error {
	log.Printf("MCP server %s ready, listening on stdio", info.Version)

	// TODO: Implement MCP protocol handling with mcp-go
	// This will be implemented in issue #4

	<-ctx.Done()
	return ctx.Err()
}
