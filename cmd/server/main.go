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
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"

	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/info"
	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/mcp"
)

func main() {
	// Initialize klog flags and configure output to stderr
	// to avoid interfering with MCP protocol on stdout
	klog.InitFlags(nil)
	if err := flag.Set("logtostderr", "true"); err != nil {
		klog.ErrorS(err, "Failed to set logtostderr flag")
	}
	flag.Parse()
	defer klog.Flush()

	klog.InfoS("Starting server",
		"name", info.Name,
		"version", info.Version,
		"commit", info.GitCommit)

	// Create context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		klog.InfoS("Received shutdown signal", "signal", sig.String())
		cancel()
	}()

	// Create and run the MCP server
	server, err := mcp.NewServer()
	if err != nil {
		klog.ErrorS(err, "Failed to create MCP server")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	klog.InfoS("MCP server initialized, starting stdio transport")

	if err := server.Run(ctx); err != nil && err != context.Canceled {
		klog.ErrorS(err, "MCP server error")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	klog.InfoS("MCP server shutdown complete")
}
