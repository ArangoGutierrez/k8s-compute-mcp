//go:build e2e

// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	JobSetManifestURL = "https://github.com/kubernetes-sigs/jobset/releases/download/v0.6.0/manifests.yaml"
)

// InstallOperators installs Kueue, MPI Operator, and JobSet controller.
func InstallOperators(ctx context.Context) error {
	fmt.Println("Installing operators (Kueue, MPI Operator, JobSet)...")

	// Install Kueue
	if err := installKueue(ctx); err != nil {
		return err
	}

	// Install MPI Operator
	if err := installMPIOperator(ctx); err != nil {
		return err
	}

	// Install JobSet controller
	if err := installJobSet(ctx); err != nil {
		return err
	}

	fmt.Println("All operators installed successfully")
	return nil
}

// installKueue installs Kueue via Helm OCI registry.
func installKueue(ctx context.Context) error {
	fmt.Println("Installing Kueue...")

	// Install Kueue from OCI registry
	cmd := exec.CommandContext(ctx, "helm", "install", "kueue",
		"oci://registry.k8s.io/kueue/charts/kueue",
		"--namespace", "kueue-system",
		"--create-namespace",
		"--wait",
		"--timeout", "5m")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Kueue: %w", err)
	}

	// Wait for Kueue CRDs to be ready
	time.Sleep(5 * time.Second)

	// Create default LocalQueue
	if err := createDefaultQueue(ctx); err != nil {
		return fmt.Errorf("failed to create default queue: %w", err)
	}

	fmt.Println("Kueue installed successfully")
	return nil
}

// installMPIOperator installs MPI Operator via kubectl with official manifests.
func installMPIOperator(ctx context.Context) error {
	fmt.Println("Installing MPI Operator...")

	// Install MPI Operator using official manifests
	manifestURL := "https://raw.githubusercontent.com/kubeflow/mpi-operator/v0.7.0/deploy/v2beta1/mpi-operator.yaml"

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "--server-side", "-f", manifestURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install MPI Operator: %w", err)
	}

	// Wait for MPI Operator deployment
	if err := waitForDeployment(ctx, "mpi-operator", "mpi-operator"); err != nil {
		return fmt.Errorf("MPI Operator not ready: %w", err)
	}

	fmt.Println("MPI Operator installed successfully")
	return nil
}

// installJobSet installs JobSet controller via manifest.
func installJobSet(ctx context.Context) error {
	fmt.Println("Installing JobSet controller...")

	// Download manifest
	tmpFile, err := os.CreateTemp("", "jobset-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	resp, err := http.Get(JobSetManifestURL) //nolint:gosec // URL is a compile-time constant
	if err != nil {
		return fmt.Errorf("failed to download JobSet manifest: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write JobSet manifest: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Apply manifest using create instead of apply to avoid annotation size limits
	cmd := exec.CommandContext(ctx, "kubectl", "create", "-f", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If already exists, that's fine
		if !strings.Contains(string(output), "AlreadyExists") {
			return fmt.Errorf("failed to create JobSet manifest: %w\nOutput: %s", err, output)
		}
	}

	// Wait for controller pod
	if err := waitForDeployment(ctx, "jobset-controller-manager", "jobset-system"); err != nil {
		return fmt.Errorf("JobSet controller not ready: %w", err)
	}

	fmt.Println("JobSet controller installed successfully")
	return nil
}

// createDefaultQueue creates a default Kueue ClusterQueue and LocalQueue.
func createDefaultQueue(ctx context.Context) error {
	manifest := `
apiVersion: kueue.x-k8s.io/v1beta1
kind: ResourceFlavor
metadata:
  name: default-flavor
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: cluster-queue
spec:
  namespaceSelector: {}
  resourceGroups:
  - coveredResources: ["cpu", "memory"]
    flavors:
    - name: default-flavor
      resources:
      - name: "cpu"
        nominalQuota: 100
      - name: "memory"
        nominalQuota: 200Gi
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: LocalQueue
metadata:
  name: default-queue
  namespace: default
spec:
  clusterQueue: cluster-queue
`

	tmpFile, err := os.CreateTemp("", "kueue-queue-*.yaml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(manifest); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", tmpFile.Name())
	return cmd.Run()
}

// waitForDeployment waits for a deployment to be ready.
func waitForDeployment(ctx context.Context, name, namespace string) error {
	fmt.Printf("Waiting for deployment '%s' to be ready...\n", name)

	timeout := 3 * time.Minute
	start := time.Now()

	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for deployment")
		}

		cmd := exec.CommandContext(ctx, "kubectl", "get", "deployment", name,
			"-n", namespace,
			"-o", "jsonpath={.status.readyReplicas}")
		output, err := cmd.Output()

		if err == nil && string(output) != "" && string(output) != "0" {
			fmt.Printf("Deployment '%s' is ready\n", name)
			return nil
		}

		time.Sleep(5 * time.Second)
	}
}
