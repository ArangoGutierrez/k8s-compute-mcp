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
	"os"

	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/info"
	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/k8s"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ServerConfig holds configuration for the MCP server.
type ServerConfig struct {
	// PVCName is the name of the shared PVC for job data.
	PVCName string

	// PVCMountPath is the mount path for the shared PVC.
	PVCMountPath string

	// KueueQueue is the default Kueue queue name for scheduling.
	KueueQueue string

	// DefaultImage is the default container image for Python workloads.
	DefaultImage string
}

// DefaultServerConfig returns the default server configuration,
// reading from environment variables with sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		PVCName:      getEnvOrDefault("PVC_NAME", "compute-data"),
		PVCMountPath: getEnvOrDefault("PVC_MOUNT_PATH", "/mnt/data"),
		KueueQueue:   getEnvOrDefault("KUEUE_QUEUE", "default-queue"),
		DefaultImage: getEnvOrDefault("DEFAULT_IMAGE", "python:3.11-slim"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// Server represents the MCP server instance.
type Server struct {
	mcpServer *server.MCPServer
	k8sClient *k8s.Client
	config    ServerConfig
}

// NewServer creates a new MCP server instance.
//
// The K8s client is configured via environment variables:
//   - KUBECONFIG: Path to kubeconfig file (default: ~/.kube/config)
//   - KUBE_CONTEXT: Kubernetes context to use (default: current context)
//
// Additional configuration via environment variables:
//   - PVC_NAME: Name of the shared PVC (default: compute-data)
//   - PVC_MOUNT_PATH: Mount path for the PVC (default: /mnt/data)
//   - KUEUE_QUEUE: Default Kueue queue name (default: default-queue)
//   - DEFAULT_IMAGE: Default container image (default: python:3.11-slim)
func NewServer() (*Server, error) {
	// Load configuration from environment
	config := DefaultServerConfig()

	// Initialize K8s client using environment configuration
	client, err := k8s.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Create MCP server with tool capabilities enabled
	mcpServer := server.NewMCPServer(
		info.Name,
		info.Version,
		server.WithToolCapabilities(true),
		server.WithRecovery(), // Panic recovery for robustness
		server.WithInstructions(
			"k8s-compute-mcp enables LLMs to offload heavy mathematical and financial "+
				"computations to a Kubernetes cluster using specialized operators like "+
				"Kubeflow MPI-Operator and JobSet.",
		),
	)

	s := &Server{
		mcpServer: mcpServer,
		k8sClient: client,
		config:    config,
	}

	// Register tools
	s.registerTools()

	log.Printf("Server config: PVC=%s, MountPath=%s, KueueQueue=%s",
		config.PVCName, config.PVCMountPath, config.KueueQueue)

	return s, nil
}

// registerTools registers all MCP tools with the server.
func (s *Server) registerTools() {
	// submit_mpi_job - Submit MPI workloads
	submitMPIJobTool := mcp.NewTool(
		"submit_mpi_job",
		mcp.WithDescription(
			"Submit an MPI workload to the Kubernetes cluster. "+
				"Returns immediately with a job ID (non-blocking). "+
				"Use check_status to monitor progress.",
		),
		mcp.WithInputSchema[SubmitMPIJobInput](),
	)
	s.mcpServer.AddTool(submitMPIJobTool, newToolHandler(s.handleSubmitMPIJob))

	// submit_monte_carlo_batch - Submit parallel Monte Carlo simulations
	submitMonteCarloTool := mcp.NewTool(
		"submit_monte_carlo_batch",
		mcp.WithDescription(
			"Submit a batch of parallel Monte Carlo simulations using JobSet. "+
				"Each replica gets a unique JOB_COMPLETION_INDEX for output path uniqueness. "+
				"Returns immediately with a job ID (non-blocking).",
		),
		mcp.WithInputSchema[SubmitMonteCarloInput](),
	)
	s.mcpServer.AddTool(submitMonteCarloTool, newToolHandler(s.handleSubmitMonteCarlo))

	// submit_reducer - Submit result aggregation jobs
	submitReducerTool := mcp.NewTool(
		"submit_reducer",
		mcp.WithDescription(
			"Submit a job to aggregate results from Monte Carlo simulations or other batch jobs. "+
				"Returns immediately with a job ID (non-blocking).",
		),
		mcp.WithInputSchema[SubmitReducerInput](),
	)
	s.mcpServer.AddTool(submitReducerTool, newToolHandler(s.handleSubmitReducer))

	// check_status - Query job status
	checkStatusTool := mcp.NewTool(
		"check_status",
		mcp.WithDescription(
			"Check the status of a submitted job. "+
				"Returns phase (Pending/Running/Succeeded/Failed) and replica counts.",
		),
		mcp.WithInputSchema[CheckStatusInput](),
	)
	s.mcpServer.AddTool(checkStatusTool, newToolHandler(s.handleCheckStatus))

	// read_artifact_head - Read result files
	readArtifactTool := mcp.NewTool(
		"read_artifact_head",
		mcp.WithDescription(
			"Read the first N lines of a result file from the shared storage (PVC at /mnt/data). "+
				"Useful for inspecting job outputs without downloading entire files.",
		),
		mcp.WithInputSchema[ReadArtifactInput](),
	)
	s.mcpServer.AddTool(readArtifactTool, newToolHandler(s.handleReadArtifactHead))

	log.Printf("Registered %d MCP tools", 5)
}

// newToolHandler creates a type-safe MCP tool handler wrapper.
// This is a generic function that bridges typed handler signatures with mcp-go.
func newToolHandler[T any, R any](
	handler func(context.Context, T) (*R, error),
) server.ToolHandlerFunc {
	return mcp.NewTypedToolHandler(
		func(ctx context.Context, req mcp.CallToolRequest, args T) (*mcp.CallToolResult, error) {
			result, err := handler(ctx, args)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructured(result, ""), nil
		},
	)
}

// Run starts the MCP server with stdio transport.
// It blocks until the context is cancelled or an error occurs.
func (s *Server) Run(ctx context.Context) error {
	log.Printf("MCP server %s ready, context: %s, namespace: %s",
		info.Version, s.k8sClient.Context(), s.k8sClient.Namespace())

	// Start stdio transport - this blocks until completion
	// Note: ServeStdio doesn't accept context, so we use a goroutine pattern
	errChan := make(chan error, 1)

	go func() {
		errChan <- server.ServeStdio(s.mcpServer)
	}()

	// Wait for either context cancellation or server error
	select {
	case <-ctx.Done():
		log.Printf("Context cancelled, shutting down MCP server")
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}
