// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

// Package main provides the entry point for the k8s-compute-mcp MCP server.
//
// The server communicates over stdio using the MCP protocol and provides
// tools for submitting computational workloads to Kubernetes clusters.
//
// Configuration is via environment variables:
//   - KUBECONFIG: Path to kubeconfig file (default: ~/.kube/config)
//   - KUBE_CONTEXT: Kubernetes context to use (default: current context)
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/info"
	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/mcp"
)

func main() {
	// Configure logging to stderr to avoid interfering with MCP protocol on stdout
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Printf("Starting %s %s (commit: %s)", info.Name, info.Version, info.GitCommit)

	// Create context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, initiating graceful shutdown", sig)
		cancel()
	}()

	// Create and run the MCP server
	server, err := mcp.NewServer()
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	log.Printf("MCP server initialized, starting stdio transport")

	if err := server.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("MCP server error: %v", err)
	}

	log.Printf("MCP server shutdown complete")
}
