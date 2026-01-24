// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

// SubmitMPIJobInput defines the input schema for the submit_mpi_job tool.
type SubmitMPIJobInput struct {
	// Code is the source code to run (Python or C++).
	Code string `json:"code" jsonschema:"required" jsonschema_description:"Source code to execute (Python or C++)"`

	// Language specifies the programming language of the code.
	Language string `json:"language" jsonschema:"required,enum=python,enum=cpp" jsonschema_description:"Programming language of the code"`

	// Nodes is the number of MPI worker nodes to use.
	Nodes int `json:"nodes" jsonschema:"required,minimum=1,maximum=64" jsonschema_description:"Number of MPI worker nodes"`

	// Namespace is the Kubernetes namespace for the job (optional).
	Namespace string `json:"namespace,omitempty" jsonschema_description:"Kubernetes namespace (defaults to client default)"`
}

// SubmitMonteCarloInput defines the input schema for the submit_monte_carlo_batch tool.
type SubmitMonteCarloInput struct {
	// Script is the Python script for the Monte Carlo simulation.
	Script string `json:"script" jsonschema:"required" jsonschema_description:"Python script for Monte Carlo simulation"`

	// Replicas is the number of parallel simulation replicas.
	Replicas int `json:"replicas" jsonschema:"required,minimum=1,maximum=1000" jsonschema_description:"Number of parallel simulation replicas"`

	// Namespace is the Kubernetes namespace for the job (optional).
	Namespace string `json:"namespace,omitempty" jsonschema_description:"Kubernetes namespace (defaults to client default)"`
}

// SubmitReducerInput defines the input schema for the submit_reducer tool.
type SubmitReducerInput struct {
	// Script is the Python script for result aggregation.
	Script string `json:"script" jsonschema:"required" jsonschema_description:"Python script for aggregating results"`

	// InputPattern is an optional glob pattern for input files.
	InputPattern string `json:"input_pattern,omitempty" jsonschema_description:"Glob pattern for input files (e.g., /mnt/data/results/*.json)"`

	// Namespace is the Kubernetes namespace for the job (optional).
	Namespace string `json:"namespace,omitempty" jsonschema_description:"Kubernetes namespace (defaults to client default)"`
}

// CheckStatusInput defines the input schema for the check_status tool.
type CheckStatusInput struct {
	// JobID is the unique identifier of the job to check.
	JobID string `json:"job_id" jsonschema:"required" jsonschema_description:"Job identifier returned by submit_* tools"`

	// JobType specifies the type of job (mpijob, jobset, or job).
	JobType string `json:"job_type" jsonschema:"required,enum=mpijob,enum=jobset,enum=job" jsonschema_description:"Type of Kubernetes job resource"`

	// Namespace is the Kubernetes namespace for the job (optional).
	Namespace string `json:"namespace,omitempty" jsonschema_description:"Kubernetes namespace (defaults to client default)"`
}

// ReadArtifactInput defines the input schema for the read_artifact_head tool.
type ReadArtifactInput struct {
	// Path is the path to the file within /mnt/data.
	Path string `json:"path" jsonschema:"required" jsonschema_description:"Path to file within /mnt/data"`

	// Lines is the number of lines to read from the file head.
	Lines int `json:"lines,omitempty" jsonschema:"minimum=1,maximum=1000,default=100" jsonschema_description:"Number of lines to read (default: 100)"`
}

// SubmitResult is the response returned by submit_* tools.
type SubmitResult struct {
	// JobID is the unique identifier assigned to the submitted job.
	JobID string `json:"job_id" jsonschema_description:"Unique job identifier for status checks"`

	// JobType indicates the type of job submitted.
	JobType string `json:"job_type" jsonschema_description:"Type of Kubernetes resource created"`

	// Namespace is the namespace where the job was created.
	Namespace string `json:"namespace" jsonschema_description:"Kubernetes namespace of the job"`

	// Message is a human-readable status message.
	Message string `json:"message" jsonschema_description:"Human-readable status message"`
}

// StatusResult is the response returned by the check_status tool.
type StatusResult struct {
	// JobID is the job identifier.
	JobID string `json:"job_id" jsonschema_description:"Job identifier"`

	// Phase is the current phase (Pending, Running, Succeeded, Failed).
	Phase string `json:"phase" jsonschema_description:"Current job phase"`

	// Ready is the number of ready replicas.
	Ready int `json:"ready" jsonschema_description:"Number of ready replicas"`

	// Succeeded is the number of succeeded replicas.
	Succeeded int `json:"succeeded" jsonschema_description:"Number of succeeded replicas"`

	// Failed is the number of failed replicas.
	Failed int `json:"failed" jsonschema_description:"Number of failed replicas"`

	// Message is a human-readable status description.
	Message string `json:"message" jsonschema_description:"Human-readable status description"`
}

// ArtifactResult is the response returned by the read_artifact_head tool.
type ArtifactResult struct {
	// Path is the file path that was read.
	Path string `json:"path" jsonschema_description:"File path that was read"`

	// Content is the file content (first N lines).
	Content string `json:"content" jsonschema_description:"File content (first N lines)"`

	// Lines is the number of lines returned.
	Lines int `json:"lines" jsonschema_description:"Number of lines returned"`
}
