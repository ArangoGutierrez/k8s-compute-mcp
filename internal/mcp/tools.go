// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

// SubmitMPIJobInput defines the input schema for the submit_mpi_job tool.
type SubmitMPIJobInput struct {
	// Code is the source code to run (Python or C++).
	Code string `json:"code" jsonschema:"required,description=Source code to execute"`

	// Language specifies the programming language of the code.
	Language string `json:"language" jsonschema:"required,enum=python,enum=cpp,description=Programming language"`

	// Nodes is the number of MPI worker nodes to use.
	Nodes int `json:"nodes" jsonschema:"required,minimum=1,maximum=64,description=Number of MPI nodes"`
}

// SubmitMonteCarloInput defines the input schema for the submit_monte_carlo_batch tool.
type SubmitMonteCarloInput struct {
	// Script is the Python script for the Monte Carlo simulation.
	Script string `json:"script" jsonschema:"required,description=Python script for simulation"`

	// Replicas is the number of parallel simulation replicas.
	Replicas int `json:"replicas" jsonschema:"required,minimum=1,maximum=1000,description=Number of parallel replicas"`
}

// SubmitReducerInput defines the input schema for the submit_reducer tool.
type SubmitReducerInput struct {
	// Script is the Python script for result aggregation.
	Script string `json:"script" jsonschema:"required,description=Python script for aggregation"`

	// InputPattern is an optional glob pattern for input files.
	InputPattern string `json:"input_pattern,omitempty" jsonschema:"description=Glob pattern for input files"`
}

// CheckStatusInput defines the input schema for the check_status tool.
type CheckStatusInput struct {
	// JobID is the unique identifier of the job to check.
	JobID string `json:"job_id" jsonschema:"required,description=Job identifier"`

	// JobType specifies the type of job (mpijob, jobset, or job).
	JobType string `json:"job_type" jsonschema:"required,enum=mpijob,enum=jobset,enum=job,description=Type of job"`
}

// ReadArtifactInput defines the input schema for the read_artifact_head tool.
type ReadArtifactInput struct {
	// Path is the path to the file within /mnt/data.
	Path string `json:"path" jsonschema:"required,description=Path to file in /mnt/data"`

	// Lines is the number of lines to read from the file head.
	Lines int `json:"lines,omitempty" jsonschema:"minimum=1,maximum=1000,default=100,description=Number of lines to read"`
}

// SubmitResult is the response returned by submit_* tools.
type SubmitResult struct {
	// JobID is the unique identifier assigned to the submitted job.
	JobID string `json:"job_id"`

	// JobType indicates the type of job submitted.
	JobType string `json:"job_type"`

	// Message is a human-readable status message.
	Message string `json:"message"`
}

// StatusResult is the response returned by the check_status tool.
type StatusResult struct {
	// JobID is the job identifier.
	JobID string `json:"job_id"`

	// Phase is the current phase (Pending, Running, Succeeded, Failed).
	Phase string `json:"phase"`

	// Ready is the number of ready replicas.
	Ready int `json:"ready"`

	// Succeeded is the number of succeeded replicas.
	Succeeded int `json:"succeeded"`

	// Failed is the number of failed replicas.
	Failed int `json:"failed"`

	// Message is a human-readable status description.
	Message string `json:"message"`
}

// ArtifactResult is the response returned by the read_artifact_head tool.
type ArtifactResult struct {
	// Path is the file path that was read.
	Path string `json:"path"`

	// Content is the file content (first N lines).
	Content string `json:"content"`

	// Lines is the number of lines returned.
	Lines int `json:"lines"`
}
