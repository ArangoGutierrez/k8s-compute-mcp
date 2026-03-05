// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/k8s"
)

var (
	// namespaceRegexp validates RFC 1123 label names.
	namespaceRegexp = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	// safePathRegexp validates allowed characters in artifact paths.
	safePathRegexp = regexp.MustCompile(`^[a-zA-Z0-9._/\-]+$`)
)

const (
	// maxCodeBytes is the maximum allowed size for user-provided code/scripts (64 KiB).
	// This prevents abuse via arbitrarily large payloads injected into pod specs,
	// while comfortably fitting real inline scripts. K8s etcd has a 1.5 MiB object limit.
	maxCodeBytes = 64 * 1024
)

// validateNamespace validates a Kubernetes namespace name per RFC 1123.
func validateNamespace(ns string) error {
	if ns == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if len(ns) > 63 {
		return fmt.Errorf("namespace must be at most 63 characters, got %d", len(ns))
	}
	if !namespaceRegexp.MatchString(ns) {
		return fmt.Errorf("namespace %q is invalid: must match RFC 1123 label (lowercase alphanumeric and hyphens, must start and end with alphanumeric)", ns)
	}
	return nil
}

// generateJobName creates a unique job name with the given prefix.
// Format: prefix-timestamp-random (e.g., mpi-20260124-a1b2c)
func generateJobName(prefix string) string {
	timestamp := time.Now().Format("20060102-150405")
	suffix := rand.String(5)
	return fmt.Sprintf("%s-%s-%s", prefix, timestamp, suffix)
}

// resolveNamespace returns the provided namespace or falls back to the client's default.
func (s *Server) resolveNamespace(ns string) string {
	if ns != "" {
		return ns
	}
	return s.k8sClient.Namespace()
}

// validateCode performs basic validation on user-provided code.
// Returns an error if the code appears to be malicious or invalid.
func validateCode(code string) error {
	if len(code) > maxCodeBytes {
		return fmt.Errorf("code exceeds maximum size of %d bytes, got %d", maxCodeBytes, len(code))
	}
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("code cannot be empty")
	}

	return nil
}

// handleSubmitMPIJob handles the submit_mpi_job tool invocation.
// It generates an MPIJob manifest and submits it to the cluster.
//
// This handler returns immediately after submission (non-blocking).
func (s *Server) handleSubmitMPIJob(ctx context.Context, input SubmitMPIJobInput) (*SubmitResult, error) {
	klog.InfoS("Handling submit_mpi_job", "namespace", input.Namespace, "nodes", input.Nodes, "language", input.Language)

	// 1. Validate input
	if err := validateCode(input.Code); err != nil {
		klog.ErrorS(err, "Invalid code in submit_mpi_job")
		return nil, fmt.Errorf("invalid code: %w", err)
	}

	if input.Nodes < 1 {
		return nil, fmt.Errorf("nodes must be at least 1, got %d", input.Nodes)
	}

	// 2. Generate unique job name
	namespace := s.resolveNamespace(input.Namespace)
	if err := validateNamespace(namespace); err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}
	jobName := generateJobName("mpi")

	// 3. Build MPIJob manifest
	cfg := k8s.MPIJobConfig{
		Name:       jobName,
		Namespace:  namespace,
		Replicas:   input.Nodes,
		Code:       input.Code,
		Language:   input.Language,
		Image:      s.config.DefaultImage,
		KueueQueue: s.config.KueueQueue,
		PVCName:    s.config.PVCName,
		MountPath:  s.config.PVCMountPath,
	}

	mpijob, err := k8s.BuildMPIJob(cfg)
	if err != nil {
		klog.ErrorS(err, "Failed to build MPIJob manifest", "jobName", jobName)
		return nil, fmt.Errorf("failed to build MPIJob manifest: %w", err)
	}

	// 4. Apply to cluster
	if err := s.k8sClient.SubmitMPIJob(ctx, mpijob); err != nil {
		klog.ErrorS(err, "Failed to submit MPIJob", "jobName", jobName, "namespace", namespace)
		return nil, fmt.Errorf("failed to submit MPIJob: %w", err)
	}

	// TODO: decrement on job completion (requires background reconciler)
	ActiveJobs.WithLabelValues(namespace, "mpijob").Inc()

	// 5. Return job ID immediately (non-blocking)
	return &SubmitResult{
		JobID:     jobName,
		JobType:   "mpijob",
		Namespace: namespace,
		Message:   fmt.Sprintf("MPIJob '%s' submitted successfully with %d worker nodes", jobName, input.Nodes),
	}, nil
}

// handleSubmitMonteCarlo handles the submit_monte_carlo_batch tool invocation.
// It generates a JobSet manifest for parallel Monte Carlo simulations.
//
// CRITICAL: Injects JOB_COMPLETION_INDEX into output paths to prevent data races.
func (s *Server) handleSubmitMonteCarlo(ctx context.Context, input SubmitMonteCarloInput) (*SubmitResult, error) {
	klog.InfoS("Handling submit_monte_carlo_batch", "namespace", input.Namespace, "replicas", input.Replicas)

	// 1. Validate script
	if err := validateCode(input.Script); err != nil {
		klog.ErrorS(err, "Invalid script in submit_monte_carlo_batch")
		return nil, fmt.Errorf("invalid script: %w", err)
	}

	if input.Replicas < 1 {
		return nil, fmt.Errorf("replicas must be at least 1, got %d", input.Replicas)
	}

	// Note: Scripts should use JOB_COMPLETION_INDEX env var for unique output paths.
	// The JobSet controller injects this automatically.

	// 2. Generate unique JobSet name
	namespace := s.resolveNamespace(input.Namespace)
	if err := validateNamespace(namespace); err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}
	jobName := generateJobName("mc")

	// 3. Build JobSet manifest
	cfg := k8s.JobSetConfig{
		Name:       jobName,
		Namespace:  namespace,
		Replicas:   input.Replicas,
		Script:     input.Script,
		Image:      s.config.DefaultImage,
		KueueQueue: s.config.KueueQueue,
		PVCName:    s.config.PVCName,
		MountPath:  s.config.PVCMountPath,
	}

	jobset, err := k8s.BuildJobSet(cfg)
	if err != nil {
		klog.ErrorS(err, "Failed to build JobSet manifest", "jobName", jobName)
		return nil, fmt.Errorf("failed to build JobSet manifest: %w", err)
	}

	// 4. Apply to cluster
	if err := s.k8sClient.SubmitJobSet(ctx, jobset); err != nil {
		klog.ErrorS(err, "Failed to submit JobSet", "jobName", jobName, "namespace", namespace)
		return nil, fmt.Errorf("failed to submit JobSet: %w", err)
	}

	// TODO: decrement on job completion (requires background reconciler)
	ActiveJobs.WithLabelValues(namespace, "jobset").Inc()

	// 5. Return job ID immediately (non-blocking)
	return &SubmitResult{
		JobID:     jobName,
		JobType:   "jobset",
		Namespace: namespace,
		Message: fmt.Sprintf("JobSet '%s' submitted successfully with %d parallel replicas. "+
			"Use JOB_COMPLETION_INDEX env var for unique output paths.", jobName, input.Replicas),
	}, nil
}

// handleSubmitReducer handles the submit_reducer tool invocation.
// It generates a Batch Job manifest for result aggregation.
func (s *Server) handleSubmitReducer(ctx context.Context, input SubmitReducerInput) (*SubmitResult, error) {
	klog.InfoS("Handling submit_reducer", "namespace", input.Namespace, "inputPattern", input.InputPattern)

	// 1. Validate input
	if err := validateCode(input.Script); err != nil {
		klog.ErrorS(err, "Invalid script in submit_reducer")
		return nil, fmt.Errorf("invalid script: %w", err)
	}

	// 2. Generate unique job name
	namespace := s.resolveNamespace(input.Namespace)
	if err := validateNamespace(namespace); err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}
	jobName := generateJobName("reduce")

	// 3. Build Batch Job manifest
	cfg := k8s.ReducerJobConfig{
		Name:         jobName,
		Namespace:    namespace,
		Script:       input.Script,
		InputPattern: input.InputPattern,
		Image:        s.config.DefaultImage,
		KueueQueue:   s.config.KueueQueue,
		PVCName:      s.config.PVCName,
		MountPath:    s.config.PVCMountPath,
	}

	job, err := k8s.BuildReducerJob(cfg)
	if err != nil {
		klog.ErrorS(err, "Failed to build reducer Job manifest", "jobName", jobName)
		return nil, fmt.Errorf("failed to build reducer Job manifest: %w", err)
	}

	// 4. Apply to cluster
	if err := s.k8sClient.SubmitJob(ctx, job); err != nil {
		klog.ErrorS(err, "Failed to submit reducer Job", "jobName", jobName, "namespace", namespace)
		return nil, fmt.Errorf("failed to submit reducer Job: %w", err)
	}

	// TODO: decrement on job completion (requires background reconciler)
	ActiveJobs.WithLabelValues(namespace, "job").Inc()

	// 5. Return job ID immediately (non-blocking)
	msg := fmt.Sprintf("Reducer Job '%s' submitted successfully", jobName)
	if input.InputPattern != "" {
		msg += fmt.Sprintf(" with input pattern '%s'", input.InputPattern)
	}

	return &SubmitResult{
		JobID:     jobName,
		JobType:   "job",
		Namespace: namespace,
		Message:   msg,
	}, nil
}

// handleCheckStatus handles the check_status tool invocation.
// It queries the status of a specific job by ID and type.
func (s *Server) handleCheckStatus(ctx context.Context, input CheckStatusInput) (*StatusResult, error) {
	klog.InfoS("Handling check_status", "jobID", input.JobID, "jobType", input.JobType, "namespace", input.Namespace)

	// 1. Validate input
	if input.JobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}

	namespace := s.resolveNamespace(input.Namespace)
	if err := validateNamespace(namespace); err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}

	// 2. Query appropriate API based on job_type
	switch input.JobType {
	case "mpijob":
		return s.getMPIJobStatus(ctx, namespace, input.JobID)
	case "jobset":
		return s.getJobSetStatus(ctx, namespace, input.JobID)
	case "job":
		return s.getBatchJobStatus(ctx, namespace, input.JobID)
	default:
		return nil, fmt.Errorf("unsupported job_type: %s (must be mpijob, jobset, or job)", input.JobType)
	}
}

// getMPIJobStatus retrieves the status of an MPIJob.
func (s *Server) getMPIJobStatus(ctx context.Context, namespace, name string) (*StatusResult, error) {
	phase, err := s.k8sClient.GetMPIJobStatus(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get MPIJob status: %w", err)
	}

	return &StatusResult{
		JobID:   name,
		Phase:   phase,
		Message: fmt.Sprintf("MPIJob '%s/%s' is in phase: %s", namespace, name, phase),
	}, nil
}

// getJobSetStatus retrieves the status of a JobSet.
func (s *Server) getJobSetStatus(ctx context.Context, namespace, name string) (*StatusResult, error) {
	phase, err := s.k8sClient.GetJobSetStatus(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get JobSet status: %w", err)
	}

	return &StatusResult{
		JobID:   name,
		Phase:   phase,
		Message: fmt.Sprintf("JobSet '%s/%s' is in phase: %s", namespace, name, phase),
	}, nil
}

// getBatchJobStatus retrieves the status of a Batch Job.
func (s *Server) getBatchJobStatus(ctx context.Context, namespace, name string) (*StatusResult, error) {
	status, err := s.k8sClient.GetJobStatus(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get Job status: %w", err)
	}

	// Determine phase from Job conditions and counts
	var phase string
	switch {
	case status.Succeeded > 0:
		phase = "Succeeded"
	case status.Failed > 0:
		phase = "Failed"
	case status.Active > 0:
		phase = "Running"
	default:
		phase = "Pending"
	}

	// Ready is a pointer in batchv1.JobStatus, handle nil case
	ready := 0
	if status.Ready != nil {
		ready = int(*status.Ready)
	}

	return &StatusResult{
		JobID:     name,
		Phase:     phase,
		Ready:     ready,
		Succeeded: int(status.Succeeded),
		Failed:    int(status.Failed),
		Message: fmt.Sprintf("Job '%s/%s': %s (active=%d, succeeded=%d, failed=%d)",
			namespace, name, phase, status.Active, status.Succeeded, status.Failed),
	}, nil
}

// handleReadArtifactHead handles the read_artifact_head tool invocation.
// It reads the first N lines of a file from the shared PVC using an ephemeral pod.
//
// Security: Validates path is within /mnt/data to prevent path traversal.
//
// Workflow:
//  1. Validate path is within allowed mount path
//  2. Create ephemeral pod with PVC mount running `head -n <lines> <path>`
//  3. Wait for pod completion (with timeout)
//  4. Read pod logs (stdout contains file content)
//  5. Delete pod (cleanup)
func (s *Server) handleReadArtifactHead(ctx context.Context, input ReadArtifactInput) (*ArtifactResult, error) {
	klog.InfoS("Handling read_artifact_head", "path", input.Path, "lines", input.Lines)

	// 1. Validate path is within mount path to prevent path traversal
	if err := validateArtifactPath(input.Path, s.config.PVCMountPath); err != nil {
		klog.ErrorS(err, "Invalid artifact path", "path", input.Path)
		return nil, err
	}

	// Set default lines if not specified
	lines := input.Lines
	if lines <= 0 {
		lines = 100
	}

	// 2. Configure and execute ephemeral reader
	cfg := k8s.ReadArtifactConfig{
		Path:      input.Path,
		Lines:     lines,
		Namespace: s.k8sClient.Namespace(),
		PVCName:   s.config.PVCName,
		MountPath: s.config.PVCMountPath,
		// Use default timeout (30s)
	}

	result, err := s.k8sClient.ReadArtifactHead(ctx, cfg)
	if err != nil {
		klog.ErrorS(err, "Failed to read artifact", "path", input.Path)
		return nil, fmt.Errorf("failed to read artifact: %w", err)
	}

	return &ArtifactResult{
		Path:    input.Path,
		Content: result.Content,
		Lines:   result.Lines,
	}, nil
}

// validateArtifactPath ensures the path is within the allowed mount path.
// Prevents path traversal attacks (e.g., /mnt/data/../../../etc/passwd).
func validateArtifactPath(path, mountPath string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	if mountPath == "" {
		return fmt.Errorf("mount path is not configured")
	}

	// Validate path contains only safe characters
	if !safePathRegexp.MatchString(path) {
		return fmt.Errorf("path contains invalid characters: %s", path)
	}

	// Clean both paths to normalize trailing slashes, double slashes, etc.
	cleaned := filepath.Clean(path)
	cleanMount := filepath.Clean(mountPath)

	// Path must be within the mount path (with trailing slash to prevent prefix attacks)
	if cleaned != cleanMount && !strings.HasPrefix(cleaned, cleanMount+"/") {
		return fmt.Errorf("path must be within %s, got: %s", cleanMount, path)
	}

	return nil
}
