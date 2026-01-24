// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// ReducerJobConfig contains configuration for building a reducer Job manifest.
type ReducerJobConfig struct {
	// Name is the job name.
	Name string

	// Namespace is the target namespace.
	Namespace string

	// Script is the Python script for aggregation.
	Script string

	// InputPattern is an optional glob pattern for input files.
	InputPattern string

	// Image is the container image to use.
	Image string

	// KueueQueue is the Kueue queue name for scheduling.
	KueueQueue string

	// PVCName is the name of the shared PVC.
	PVCName string

	// MountPath is the mount path for the PVC.
	MountPath string
}

// BuildReducerJob creates a Batch Job manifest for result aggregation.
func BuildReducerJob(cfg ReducerJobConfig) (*batchv1.Job, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("job name is required")
	}
	if cfg.Script == "" {
		return nil, fmt.Errorf("job script is required")
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

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To[int32](3),
			TTLSecondsAfterFinished: ptr.To[int32](3600), // Cleanup after 1 hour
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "reducer",
							Image: cfg.Image,
							Command: []string{
								"python", "-c", cfg.Script,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "INPUT_PATTERN",
									Value: cfg.InputPattern,
								},
								{
									Name:  "MOUNT_PATH",
									Value: cfg.MountPath,
								},
							},
						},
					},
				},
			},
		},
	}

	// Add Kueue annotation if specified
	if cfg.KueueQueue != "" {
		if job.Annotations == nil {
			job.Annotations = make(map[string]string)
		}
		job.Annotations["kueue.x-k8s.io/queue-name"] = cfg.KueueQueue
	}

	// Add PVC volume mount if specified
	if cfg.PVCName != "" {
		job.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: cfg.PVCName,
					},
				},
			},
		}
		job.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: cfg.MountPath,
			},
		}
	}

	return job, nil
}

// SubmitJob applies a Batch Job manifest to the cluster.
func (c *Client) SubmitJob(ctx context.Context, job *batchv1.Job) error {
	namespace := job.Namespace
	if namespace == "" {
		namespace = c.namespace
	}

	_, err := c.clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Job: %w", err)
	}

	return nil
}

// GetJobStatus retrieves the status of a Batch Job.
func (c *Client) GetJobStatus(ctx context.Context, namespace, name string) (*batchv1.JobStatus, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	job, err := c.clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Job %s/%s: %w", namespace, name, err)
	}

	return &job.Status, nil
}
