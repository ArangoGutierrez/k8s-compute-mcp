//go:build e2e

// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// SetupStorage creates PV and PVC for shared data access.
func SetupStorage(ctx context.Context) error {
	fmt.Println("Setting up storage (PV and PVC)...")

	// Apply PersistentVolume
	if err := applyManifest(ctx, "testdata/pv-hostpath.yaml"); err != nil {
		return fmt.Errorf("failed to create PV: %w", err)
	}

	// Apply PersistentVolumeClaim
	if err := applyManifest(ctx, "testdata/pvc-shared.yaml"); err != nil {
		return fmt.Errorf("failed to create PVC: %w", err)
	}

	// Wait for PVC to be bound
	if err := waitForPVCBound(ctx, "shared-data", "default"); err != nil {
		return fmt.Errorf("PVC not bound: %w", err)
	}

	fmt.Println("Storage setup complete")
	return nil
}

// applyManifest applies a Kubernetes manifest file.
func applyManifest(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// waitForPVCBound waits for a PVC to be in Bound state.
func waitForPVCBound(ctx context.Context, name, namespace string) error {
	fmt.Printf("Waiting for PVC '%s' to be bound...\n", name)

	timeout := 1 * time.Minute
	start := time.Now()

	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for PVC bound")
		}

		cmd := exec.CommandContext(ctx, "kubectl", "get", "pvc", name,
			"-n", namespace,
			"-o", "jsonpath={.status.phase}")
		output, err := cmd.Output()

		if err == nil && string(output) == "Bound" {
			fmt.Println("PVC is bound")
			return nil
		}

		time.Sleep(2 * time.Second)
	}
}
