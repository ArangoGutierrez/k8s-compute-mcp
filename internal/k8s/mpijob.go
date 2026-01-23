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

// MPIJobGVR is the GroupVersionResource for Kubeflow MPIJob.
var MPIJobGVR = schema.GroupVersionResource{
	Group:    "kubeflow.org",
	Version:  "v2beta1",
	Resource: "mpijobs",
}

// MPIJobConfig contains configuration for building an MPIJob manifest.
type MPIJobConfig struct {
	// Name is the job name.
	Name string

	// Namespace is the target namespace.
	Namespace string

	// Replicas is the number of worker replicas.
	Replicas int

	// Code is the source code to execute.
	Code string

	// Language is the programming language (python or cpp).
	Language string

	// Image is the container image to use.
	Image string

	// KueueQueue is the Kueue queue name for scheduling.
	KueueQueue string

	// PVCName is the name of the shared PVC.
	PVCName string

	// MountPath is the mount path for the PVC.
	MountPath string
}

// BuildMPIJob creates an unstructured MPIJob manifest.
//
// Note: We use unstructured here to avoid importing the full mpi-operator
// dependency. For production, consider importing the typed structs from
// github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1.
func BuildMPIJob(cfg MPIJobConfig) (*unstructured.Unstructured, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("MPIJob name is required")
	}
	if cfg.Replicas < 1 {
		return nil, fmt.Errorf("MPIJob replicas must be >= 1")
	}
	if cfg.Code == "" {
		return nil, fmt.Errorf("MPIJob code is required")
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

	// TODO: Implement full MPIJob manifest in issue #6
	// This is a placeholder structure
	mpijob := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubeflow.org/v2beta1",
			"kind":       "MPIJob",
			"metadata": map[string]interface{}{
				"name":      cfg.Name,
				"namespace": cfg.Namespace,
			},
			"spec": map[string]interface{}{
				// Placeholder - full spec in issue #6
			},
		},
	}

	// Add Kueue annotation if specified
	if cfg.KueueQueue != "" {
		annotations := map[string]interface{}{
			"kueue.x-k8s.io/queue-name": cfg.KueueQueue,
		}
		if err := unstructured.SetNestedField(mpijob.Object, annotations, "metadata", "annotations"); err != nil {
			return nil, fmt.Errorf("failed to set annotations: %w", err)
		}
	}

	return mpijob, nil
}

// SubmitMPIJob applies an MPIJob manifest to the cluster.
func (c *Client) SubmitMPIJob(ctx context.Context, mpijob *unstructured.Unstructured) error {
	namespace := mpijob.GetNamespace()
	if namespace == "" {
		namespace = c.namespace
	}

	_, err := c.dynamicClient.Resource(MPIJobGVR).Namespace(namespace).Create(ctx, mpijob, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create MPIJob: %w", err)
	}

	return nil
}

// GetMPIJobStatus retrieves the status of an MPIJob.
func (c *Client) GetMPIJobStatus(ctx context.Context, namespace, name string) (string, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	mpijob, err := c.dynamicClient.Resource(MPIJobGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get MPIJob %s/%s: %w", namespace, name, err)
	}

	// Extract phase from status
	phase, _, err := unstructured.NestedString(mpijob.Object, "status", "conditions", "type")
	if err != nil {
		return "Unknown", nil
	}

	return phase, nil
}
