// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
)

// handleSubmitMPIJob handles the submit_mpi_job tool invocation.
// It generates an MPIJob manifest and submits it to the cluster.
//
// This handler returns immediately after submission (non-blocking).
func (s *Server) handleSubmitMPIJob(ctx context.Context, input SubmitMPIJobInput) (*SubmitResult, error) {
	// TODO: Implement in issue #9
	// 1. Validate input
	// 2. Generate unique job name
	// 3. Build MPIJob manifest
	// 4. Apply to cluster
	// 5. Return job ID immediately

	return nil, fmt.Errorf("submit_mpi_job not implemented")
}

// handleSubmitMonteCarlo handles the submit_monte_carlo_batch tool invocation.
// It generates a JobSet manifest for parallel Monte Carlo simulations.
//
// CRITICAL: Injects JOB_COMPLETION_INDEX into output paths to prevent data races.
func (s *Server) handleSubmitMonteCarlo(ctx context.Context, input SubmitMonteCarloInput) (*SubmitResult, error) {
	// TODO: Implement in issue #10
	// 1. Validate script contains JOB_COMPLETION_INDEX handling
	// 2. Generate unique JobSet name
	// 3. Build JobSet manifest
	// 4. Apply to cluster
	// 5. Return job ID immediately

	return nil, fmt.Errorf("submit_monte_carlo_batch not implemented")
}

// handleSubmitReducer handles the submit_reducer tool invocation.
// It generates a Batch Job manifest for result aggregation.
func (s *Server) handleSubmitReducer(ctx context.Context, input SubmitReducerInput) (*SubmitResult, error) {
	// TODO: Implement in issue #11
	// 1. Validate input
	// 2. Generate unique job name
	// 3. Build Batch Job manifest
	// 4. Apply to cluster
	// 5. Return job ID immediately

	return nil, fmt.Errorf("submit_reducer not implemented")
}

// handleCheckStatus handles the check_status tool invocation.
// It queries the status of a specific job by ID and type.
func (s *Server) handleCheckStatus(ctx context.Context, input CheckStatusInput) (*StatusResult, error) {
	// TODO: Implement in issue #12
	// 1. Validate input
	// 2. Query appropriate API based on job_type
	// 3. Return structured status

	return nil, fmt.Errorf("check_status not implemented")
}

// handleReadArtifactHead handles the read_artifact_head tool invocation.
// It reads the first N lines of a file from the shared PVC.
//
// Security: Validates path is within /mnt/data to prevent path traversal.
func (s *Server) handleReadArtifactHead(ctx context.Context, input ReadArtifactInput) (*ArtifactResult, error) {
	// TODO: Implement in issue #13
	// 1. Validate path is within /mnt/data
	// 2. Create ephemeral pod or exec into existing pod
	// 3. Read file content
	// 4. Clean up resources
	// 5. Return content

	return nil, fmt.Errorf("read_artifact_head not implemented")
}
