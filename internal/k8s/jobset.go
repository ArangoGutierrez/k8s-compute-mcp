// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// JobSetGVR is the GroupVersionResource for Kubernetes JobSet.
var JobSetGVR = schema.GroupVersionResource{
	Group:    "jobset.x-k8s.io",
	Version:  "v1alpha2",
	Resource: "jobsets",
}

// JobSetConfig contains configuration for building a JobSet manifest.
type JobSetConfig struct {
	// Name is the JobSet name.
	Name string

	// Namespace is the target namespace.
	Namespace string

	// Replicas is the number of parallel job replicas.
	Replicas int

	// Script is the Python script to execute.
	Script string

	// Image is the container image to use.
	Image string

	// KueueQueue is the Kueue queue name for scheduling.
	KueueQueue string

	// PVCName is the name of the shared PVC.
	PVCName string

	// MountPath is the mount path for the PVC.
	MountPath string
}

// BuildJobSet creates an unstructured JobSet manifest.
//
// CRITICAL: The generated manifest injects JOB_COMPLETION_INDEX into the
// container environment to enable output isolation and prevent data races
// when multiple replicas write to the shared PVC.
//
// Note: We use unstructured here to avoid importing the full jobset
// dependency. For production, consider importing the typed structs from
// sigs.k8s.io/jobset/api/jobset/v1alpha2.
func BuildJobSet(cfg JobSetConfig) (*unstructured.Unstructured, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("JobSet name is required")
	}
	if cfg.Replicas < 1 {
		return nil, fmt.Errorf("JobSet replicas must be >= 1")
	}
	if cfg.Script == "" {
		return nil, fmt.Errorf("JobSet script is required")
	}

	// Set defaults
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	if cfg.Image == "" {
		cfg.Image = "python:3.11-slim"
	}
	if cfg.MountPath == "" {
		cfg.MountPath = "/mnt/data"
	}

	// TODO: Implement full JobSet manifest in issue #7
	// This is a placeholder structure
	jobset := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "jobset.x-k8s.io/v1alpha2",
			"kind":       "JobSet",
			"metadata": map[string]interface{}{
				"name":      cfg.Name,
				"namespace": cfg.Namespace,
			},
			"spec": map[string]interface{}{
				// Placeholder - full spec in issue #7
				// MUST include JOB_COMPLETION_INDEX env var injection
			},
		},
	}

	// Add Kueue annotation if specified
	if cfg.KueueQueue != "" {
		annotations := map[string]interface{}{
			"kueue.x-k8s.io/queue-name": cfg.KueueQueue,
		}
		if err := unstructured.SetNestedField(jobset.Object, annotations, "metadata", "annotations"); err != nil {
			return nil, fmt.Errorf("failed to set annotations: %w", err)
		}
	}

	return jobset, nil
}

// SubmitJobSet applies a JobSet manifest to the cluster.
func (c *Client) SubmitJobSet(ctx context.Context, jobset *unstructured.Unstructured) error {
	namespace := jobset.GetNamespace()
	if namespace == "" {
		namespace = c.namespace
	}

	_, err := c.dynamicClient.Resource(JobSetGVR).Namespace(namespace).Create(ctx, jobset, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create JobSet: %w", err)
	}

	return nil
}

// GetJobSetStatus retrieves the status of a JobSet.
func (c *Client) GetJobSetStatus(ctx context.Context, namespace, name string) (string, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	jobset, err := c.dynamicClient.Resource(JobSetGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get JobSet %s/%s: %w", namespace, name, err)
	}

	// Extract phase from status
	phase, _, err := unstructured.NestedString(jobset.Object, "status", "conditions", "type")
	if err != nil {
		return "Unknown", nil
	}

	return phase, nil
}
