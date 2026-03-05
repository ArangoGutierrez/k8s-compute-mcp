// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"fmt"
	"time"

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
// The generated JobSet uses indexed completions to enable parallel Monte Carlo
// simulations. Each pod receives a unique JOB_COMPLETION_INDEX environment
// variable (automatically injected by Kubernetes for indexed jobs).
//
// CRITICAL: Scripts should use JOB_COMPLETION_INDEX to create unique output
// paths, preventing data races when multiple pods write to the shared PVC.
// Example: output_path = f"/mnt/data/results_{os.environ['JOB_COMPLETION_INDEX']}.json"
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

	// Build environment variables
	// JOB_COMPLETION_INDEX is automatically injected by Kubernetes for indexed jobs
	// We add MOUNT_PATH for convenience in scripts
	env := []interface{}{
		map[string]interface{}{
			"name":  "MOUNT_PATH",
			"value": cfg.MountPath,
		},
	}

	// Build container spec
	container := map[string]interface{}{
		"name":    "worker",
		"image":   cfg.Image,
		"command": []interface{}{"python", "-c", cfg.Script},
		"env":     env,
		"resources": map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    "1",
				"memory": "2Gi",
			},
			"requests": map[string]interface{}{
				"cpu":    "500m",
				"memory": "1Gi",
			},
		},
	}

	// Add volume mount if PVC is configured
	if cfg.PVCName != "" {
		container["volumeMounts"] = []interface{}{
			map[string]interface{}{
				"name":      "data",
				"mountPath": cfg.MountPath,
			},
		}
	}

	// Build pod spec
	podSpec := map[string]interface{}{
		"containers":    []interface{}{container},
		"restartPolicy": "Never",
	}

	// Add volumes if PVC is configured
	if cfg.PVCName != "" {
		podSpec["volumes"] = []interface{}{
			map[string]interface{}{
				"name": "data",
				"persistentVolumeClaim": map[string]interface{}{
					"claimName": cfg.PVCName,
				},
			},
		}
	}

	// Build Job template spec
	// completionMode: Indexed enables JOB_COMPLETION_INDEX injection
	jobTemplateSpec := map[string]interface{}{
		"parallelism":    int64(cfg.Replicas),
		"completions":    int64(cfg.Replicas),
		"completionMode": "Indexed",
		"backoffLimit":   int64(3),
		"template": map[string]interface{}{
			"spec": podSpec,
		},
	}

	// Build JobSet spec with replicatedJobs
	// We use a single replicatedJob with indexed completions
	spec := map[string]interface{}{
		"replicatedJobs": []interface{}{
			map[string]interface{}{
				"name":     "workers",
				"replicas": int64(1), // One Job object
				"template": map[string]interface{}{
					"spec": jobTemplateSpec,
				},
			},
		},
		"successPolicy": map[string]interface{}{
			"operator":             "All",
			"targetReplicatedJobs": []interface{}{"workers"},
		},
	}

	// Build metadata
	metadata := map[string]interface{}{
		"name":      cfg.Name,
		"namespace": cfg.Namespace,
	}

	// Add Kueue annotation if specified
	if cfg.KueueQueue != "" {
		metadata["annotations"] = map[string]interface{}{
			"kueue.x-k8s.io/queue-name": cfg.KueueQueue,
		}
	}

	jobset := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "jobset.x-k8s.io/v1alpha2",
			"kind":       "JobSet",
			"metadata":   metadata,
			"spec":       spec,
		},
	}

	return jobset, nil
}

// SubmitJobSet applies a JobSet manifest to the cluster.
// The timeout is applied per-call so that retry wrappers can invoke this
// method multiple times, each attempt getting its own 10s deadline.
func (c *Client) SubmitJobSet(ctx context.Context, jobset *unstructured.Unstructured) error {
	namespace := jobset.GetNamespace()
	if namespace == "" {
		namespace = c.namespace
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

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

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	jobset, err := c.dynamicClient.Resource(JobSetGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get JobSet %s/%s: %w", namespace, name, err)
	}

	return extractConditionPhase(jobset.Object)
}
