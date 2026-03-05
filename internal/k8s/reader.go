// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

const (
	// DefaultReadTimeout is the default timeout for ephemeral reader pods.
	DefaultReadTimeout = 30 * time.Second

	// ReaderPodImage is the lightweight image used for reading files.
	ReaderPodImage = "busybox:1.36"

	// ReaderLabelKey is the label key for identifying reader pods.
	ReaderLabelKey = "k8s-compute-mcp/reader"

	// ReaderLabelValue is the label value for reader pods.
	ReaderLabelValue = "ephemeral"

	// MaxLogBytes is the maximum number of bytes to read from pod logs (1MB).
	MaxLogBytes = int64(1 << 20)
)

// ReadArtifactConfig contains configuration for reading an artifact from PVC.
type ReadArtifactConfig struct {
	// Path is the file path within the PVC to read.
	Path string

	// Lines is the number of lines to read from the head of the file.
	Lines int

	// Namespace is the target namespace.
	Namespace string

	// PVCName is the name of the shared PVC.
	PVCName string

	// MountPath is the mount path for the PVC.
	MountPath string

	// Timeout is the maximum time to wait for the reader pod.
	Timeout time.Duration
}

// ReadArtifactResult contains the result of reading an artifact.
type ReadArtifactResult struct {
	// Content is the file content (first N lines).
	Content string

	// Lines is the actual number of lines returned.
	Lines int
}

// buildReaderPodName generates a unique name for the reader pod.
func buildReaderPodName() string {
	return fmt.Sprintf("reader-%s-%s", time.Now().Format("20060102-150405"), rand.String(5))
}

// BuildReaderPod creates a pod spec for reading files from a PVC.
func BuildReaderPod(cfg ReadArtifactConfig) (*corev1.Pod, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if cfg.PVCName == "" {
		return nil, fmt.Errorf("PVC name is required")
	}

	// Set defaults
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	if cfg.MountPath == "" {
		cfg.MountPath = "/mnt/data"
	}
	if cfg.Lines <= 0 {
		cfg.Lines = 100
	}

	podName := buildReaderPodName()

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				ReaderLabelKey: ReaderLabelValue,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "reader",
					Image:   ReaderPodImage,
					Command: []string{"head"},
					Args:    []string{"-n", strconv.Itoa(cfg.Lines), "--", cfg.Path},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: cfg.MountPath,
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: cfg.PVCName,
							ReadOnly:  true,
						},
					},
				},
			},
		},
	}

	return pod, nil
}

// ReadArtifactHead reads the first N lines of a file from the shared PVC.
// It creates an ephemeral pod, waits for completion, reads logs, and cleans up.
func (c *Client) ReadArtifactHead(ctx context.Context, cfg ReadArtifactConfig) (*ReadArtifactResult, error) {
	// Set defaults
	if cfg.Namespace == "" {
		cfg.Namespace = c.namespace
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultReadTimeout
	}

	// Build the reader pod spec
	pod, err := BuildReaderPod(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build reader pod: %w", err)
	}

	// Create the pod with retry for transient errors (each attempt gets its own timeout)
	var podName string
	if err := RetryOnTransient(ctx, "CreateReaderPod", func() error {
		createCtx, createCancel := context.WithTimeout(ctx, 10*time.Second)
		defer createCancel()

		created, err := c.clientset.CoreV1().Pods(cfg.Namespace).Create(createCtx, pod, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create reader pod: %w", err)
		}
		podName = created.Name
		return nil
	}); err != nil {
		return nil, err
	}

	// Ensure cleanup happens regardless of outcome
	defer func() {
		// Use background context for cleanup since original context may be canceled
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = c.deleteReaderPod(cleanupCtx, cfg.Namespace, podName)
	}()

	// Wait for pod completion with timeout
	if err := c.waitForPodCompletion(ctx, cfg.Namespace, podName, cfg.Timeout); err != nil {
		return nil, err
	}

	// Check pod status for errors (with retry for transient API errors)
	var finalPod *corev1.Pod
	if err := RetryOnTransient(ctx, "GetReaderPodStatus", func() error {
		p, err := c.clientset.CoreV1().Pods(cfg.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get reader pod status: %w", err)
		}
		finalPod = p
		return nil
	}); err != nil {
		return nil, err
	}

	// Check if the pod failed
	if finalPod.Status.Phase == corev1.PodFailed {
		// Try to get logs for error details
		logs, err := c.getPodLogs(ctx, cfg.Namespace, podName)
		if err != nil {
			klog.ErrorS(err, "Failed to get pod logs", "pod", podName, "namespace", cfg.Namespace)
		}
		if logs != "" {
			return nil, fmt.Errorf("reader pod failed: %s", logs)
		}
		return nil, fmt.Errorf("reader pod failed (no logs available)")
	}

	// Get pod logs (the file content)
	content, err := c.getPodLogs(ctx, cfg.Namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact content: %w", err)
	}

	// Count actual lines returned
	lineCount := countLines(content)

	return &ReadArtifactResult{
		Content: content,
		Lines:   lineCount,
	}, nil
}

// waitForPodCompletion waits for a pod to reach a terminal state.
func (c *Client) waitForPodCompletion(ctx context.Context, namespace, name string, timeout time.Duration) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get pod %s/%s: %w", namespace, name, err)
		}

		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			// Terminal state reached
			return true, nil
		case corev1.PodPending, corev1.PodRunning:
			// Still waiting
			return false, nil
		default:
			// Unknown state - continue waiting
			return false, nil
		}
	})
}

// getPodLogs retrieves the logs from a pod's main container.
// Limits output to MaxLogBytes (1MB) to prevent memory exhaustion.
func (c *Client) getPodLogs(ctx context.Context, namespace, name string) (string, error) {
	limitBytes := MaxLogBytes
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{
		Container:  "reader",
		LimitBytes: &limitBytes,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get log stream: %w", err)
	}
	defer func() { _ = stream.Close() }()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(stream, MaxLogBytes)); err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return buf.String(), nil
}

// deleteReaderPod deletes an ephemeral reader pod with retry for transient errors.
func (c *Client) deleteReaderPod(ctx context.Context, namespace, name string) error {
	return RetryOnTransient(ctx, "DeleteReaderPod", func() error {
		return c.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	})
}

// countLines counts the number of lines in a string.
func countLines(s string) int {
	if s == "" {
		return 0
	}

	count := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			count++
		}
	}

	// Don't count trailing newline as extra line
	if len(s) > 0 && s[len(s)-1] == '\n' {
		count--
	}

	return count
}

// CleanupOrphanedReaderPods deletes any reader pods that may have been left behind.
// This can be called during startup or periodically to clean up orphaned pods.
func (c *Client) CleanupOrphanedReaderPods(ctx context.Context, namespace string) error {
	if namespace == "" {
		namespace = c.namespace
	}

	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", ReaderLabelKey, ReaderLabelValue),
	}

	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("failed to list reader pods: %w", err)
	}

	var errs []error
	for _, pod := range pods.Items {
		if err := c.deleteReaderPod(ctx, namespace, pod.Name); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
