// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

// Package info contains build and version information for the server.
// These values are injected at build time via ldflags.
package info

var (
	// Name is the application name.
	Name = "k8s-compute-mcp"

	// Version is the semantic version of the application.
	// Set via -ldflags "-X github.com/ArangoGutierrez/k8s-compute-mcp/internal/info.Version=v0.1.0"
	Version = "dev"

	// GitCommit is the git commit hash.
	// Set via -ldflags "-X github.com/ArangoGutierrez/k8s-compute-mcp/internal/info.GitCommit=abc123"
	GitCommit = "unknown"
)
