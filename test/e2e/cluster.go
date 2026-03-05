//go:build e2e

// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	ClusterName    = "e2e-compute-mcp"
	DataDir        = "/tmp/e2e-compute-mcp-data"
	KindConfigPath = "testdata/kind-config.yaml"
)

// CreateKindCluster creates a new KIND cluster for E2E testing.
func CreateKindCluster(ctx context.Context) error {
	// Create data directory on host
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Check if cluster already exists
	cmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	output, _ := cmd.CombinedOutput()
	if strings.Contains(string(output), ClusterName) {
		fmt.Printf("KIND cluster '%s' already exists, deleting...\n", ClusterName)
		if err := DeleteKindCluster(ctx); err != nil {
			return err
		}
	}

	// Create cluster with config
	fmt.Printf("Creating KIND cluster '%s'...\n", ClusterName)
	cmd = exec.CommandContext(ctx, "kind", "create", "cluster",
		"--name", ClusterName,
		"--config", KindConfigPath,
		"--wait", "120s")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create KIND cluster: %w", err)
	}

	fmt.Println("KIND cluster created successfully")
	return nil
}

// DeleteKindCluster deletes the KIND cluster and cleans up data directory.
func DeleteKindCluster(ctx context.Context) error {
	fmt.Printf("Deleting KIND cluster '%s'...\n", ClusterName)

	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", ClusterName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete KIND cluster: %w", err)
	}

	// Clean up data directory
	if err := os.RemoveAll(DataDir); err != nil {
		fmt.Printf("Warning: failed to remove data directory: %v\n", err)
	}

	fmt.Println("KIND cluster deleted successfully")
	return nil
}

// WaitForClusterReady waits for the cluster to be ready.
func WaitForClusterReady(ctx context.Context) error {
	fmt.Println("Waiting for cluster to be ready...")

	timeout := 2 * time.Minute
	start := time.Now()

	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for cluster ready")
		}

		cmd := exec.CommandContext(ctx, "kubectl", "get", "nodes", "-o", "jsonpath={.items[*].status.conditions[?(@.type==\"Ready\")].status}")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "True") {
			fmt.Println("Cluster is ready")
			return nil
		}

		time.Sleep(5 * time.Second)
	}
}

// LoadTestImages loads custom Docker images into the KIND cluster.
func LoadTestImages(ctx context.Context) error {
	fmt.Println("Loading custom test images into KIND cluster...")

	// Custom Python MPI image
	mpiImage := "ghcr.io/arangogutierrez/k8s-compute-mcp-python:latest"

	// Check if image exists locally
	checkCmd := exec.CommandContext(ctx, "docker", "image", "inspect", mpiImage)
	if err := checkCmd.Run(); err != nil {
		fmt.Printf("Warning: Image %s not found locally, attempting to build...\n", mpiImage)

		// Try building the image
		buildCmd := exec.CommandContext(ctx, "docker", "build",
			"-t", mpiImage,
			"docker/mpi-python/")
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr

		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("failed to build image %s: %w", mpiImage, err)
		}
		fmt.Printf("Successfully built image: %s\n", mpiImage)
	}

	// Load image into KIND cluster
	fmt.Printf("Loading image %s into KIND cluster...\n", mpiImage)
	cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", mpiImage, "--name", ClusterName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load image into KIND: %w", err)
	}

	fmt.Println("Custom test images loaded successfully")
	return nil
}

// DumpClusterState dumps cluster state for debugging.
func DumpClusterState(ctx context.Context) {
	if os.Getenv("E2E_DEBUG") != "true" {
		return
	}

	fmt.Println("\n=== Cluster State Dump ===")

	cmds := [][]string{
		{"kubectl", "get", "all", "-A"},
		{"kubectl", "get", "pv,pvc"},
		{"kubectl", "describe", "mpijob,jobset,job"},
	}

	for _, args := range cmds {
		fmt.Printf("\n--- Running: %v ---\n", args)
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	fmt.Println("=== End Cluster State Dump ===")
}
