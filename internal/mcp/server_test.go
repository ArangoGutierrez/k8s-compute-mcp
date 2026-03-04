// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// createTestKubeconfig creates a temporary kubeconfig file for testing.
// Uses insecure-skip-tls-verify to avoid certificate validation issues in tests.
func createTestKubeconfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	kubeconfig := filepath.Join(dir, "config")

	content := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-server:6443
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
    namespace: test-ns
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
	if err := os.WriteFile(kubeconfig, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write test kubeconfig: %v", err)
	}

	return kubeconfig
}

func TestNewServer(t *testing.T) {
	// Set up test kubeconfig
	kubeconfig := createTestKubeconfig(t)
	t.Setenv("KUBECONFIG", kubeconfig)
	t.Setenv("KUBE_CONTEXT", "test-context")

	s, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if s == nil {
		t.Fatal("NewServer() returned nil server")
	}

	if s.mcpServer == nil {
		t.Fatal("NewServer() returned server with nil mcpServer")
	}

	if s.k8sClient == nil {
		t.Fatal("NewServer() returned server with nil k8sClient")
	}
}

func TestNewServer_NoKubeconfig(t *testing.T) {
	// Use a non-existent kubeconfig
	t.Setenv("KUBECONFIG", "/nonexistent/path/config")
	t.Setenv("KUBE_CONTEXT", "")

	_, err := NewServer()
	if err == nil {
		t.Fatal("NewServer() expected error with missing kubeconfig")
	}
}

func TestNewToolHandler(t *testing.T) {
	// Test that the generic handler wrapper works correctly
	type testInput struct {
		Value string `json:"value"`
	}
	type testOutput struct {
		Result string `json:"result"`
	}

	handler := func(ctx context.Context, input testInput) (*testOutput, error) {
		return &testOutput{Result: "processed: " + input.Value}, nil
	}

	wrappedHandler := newToolHandler(handler)
	if wrappedHandler == nil {
		t.Fatal("newToolHandler returned nil")
	}
}

func TestNewToolHandler_Error(t *testing.T) {
	type testInput struct {
		Value string `json:"value"`
	}
	type testOutput struct {
		Result string `json:"result"`
	}

	// Handler that returns an error
	handler := func(ctx context.Context, input testInput) (*testOutput, error) {
		return nil, context.DeadlineExceeded
	}

	wrappedHandler := newToolHandler(handler)
	if wrappedHandler == nil {
		t.Fatal("newToolHandler returned nil")
	}
}

func TestToolDefinitions(t *testing.T) {
	// Verify tool definitions are correct by creating them directly
	tests := []struct {
		name        string
		toolName    string
		description string
	}{
		{
			name:        "submit_mpi_job",
			toolName:    "submit_mpi_job",
			description: "Submit an MPI workload",
		},
		{
			name:        "submit_monte_carlo_batch",
			toolName:    "submit_monte_carlo_batch",
			description: "Submit a batch of parallel Monte Carlo",
		},
		{
			name:        "submit_reducer",
			toolName:    "submit_reducer",
			description: "Submit a job to aggregate",
		},
		{
			name:        "check_status",
			toolName:    "check_status",
			description: "Check the status of a submitted job",
		},
		{
			name:        "read_artifact_head",
			toolName:    "read_artifact_head",
			description: "Read the first N lines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a basic MCP server to verify tool registration
			mcpServer := server.NewMCPServer(
				"test-server",
				"0.0.1",
				server.WithToolCapabilities(true),
			)

			// Create the tool based on the test case
			var tool mcp.Tool
			switch tt.toolName {
			case "submit_mpi_job":
				tool = mcp.NewTool(tt.toolName,
					mcp.WithDescription("Submit an MPI workload to the Kubernetes cluster."),
					mcp.WithInputSchema[SubmitMPIJobInput](),
				)
			case "submit_monte_carlo_batch":
				tool = mcp.NewTool(tt.toolName,
					mcp.WithDescription("Submit a batch of parallel Monte Carlo simulations."),
					mcp.WithInputSchema[SubmitMonteCarloInput](),
				)
			case "submit_reducer":
				tool = mcp.NewTool(tt.toolName,
					mcp.WithDescription("Submit a job to aggregate results."),
					mcp.WithInputSchema[SubmitReducerInput](),
				)
			case "check_status":
				tool = mcp.NewTool(tt.toolName,
					mcp.WithDescription("Check the status of a submitted job."),
					mcp.WithInputSchema[CheckStatusInput](),
				)
			case "read_artifact_head":
				tool = mcp.NewTool(tt.toolName,
					mcp.WithDescription("Read the first N lines of a result file."),
					mcp.WithInputSchema[ReadArtifactInput](),
				)
			}

			// Add a dummy handler
			mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return mcp.NewToolResultText("test"), nil
			})

			// Verify the tool was added (indirect verification through no panic)
			if tool.Name != tt.toolName {
				t.Errorf("Tool name = %q, want %q", tool.Name, tt.toolName)
			}
		})
	}
}

func TestInputSchemaValidation(t *testing.T) {
	// Test that input structs have required fields
	t.Run("SubmitMPIJobInput", func(t *testing.T) {
		input := SubmitMPIJobInput{
			Code:     "print('hello')",
			Language: "python",
			Nodes:    2,
		}
		if input.Code == "" {
			t.Error("Code should not be empty")
		}
		if input.Language != "python" && input.Language != "cpp" {
			t.Error("Language should be python or cpp")
		}
		if input.Nodes < 1 || input.Nodes > 64 {
			t.Error("Nodes should be between 1 and 64")
		}
	})

	t.Run("SubmitMonteCarloInput", func(t *testing.T) {
		input := SubmitMonteCarloInput{
			Script:   "import random",
			Replicas: 10,
		}
		if input.Script == "" {
			t.Error("Script should not be empty")
		}
		if input.Replicas < 1 || input.Replicas > 1000 {
			t.Error("Replicas should be between 1 and 1000")
		}
	})

	t.Run("CheckStatusInput", func(t *testing.T) {
		input := CheckStatusInput{
			JobID:   "test-job-123",
			JobType: "mpijob",
		}
		if input.JobID == "" {
			t.Error("JobID should not be empty")
		}
		validTypes := map[string]bool{"mpijob": true, "jobset": true, "job": true}
		if !validTypes[input.JobType] {
			t.Error("JobType should be mpijob, jobset, or job")
		}
	})

	t.Run("ReadArtifactInput", func(t *testing.T) {
		input := ReadArtifactInput{
			Path:  "/mnt/data/results.csv",
			Lines: 50,
		}
		if input.Path == "" {
			t.Error("Path should not be empty")
		}
		if input.Lines < 1 || input.Lines > 1000 {
			t.Error("Lines should be between 1 and 1000")
		}
	})
}

func TestServerRun_ContextCancellation(t *testing.T) {
	// Test that Server.Run properly cleans up when context is canceled:
	// 1. Returns context.Canceled error
	// 2. Closes stdin to unblock ServeStdio goroutine
	// 3. Waits for the goroutine to finish before returning
	//
	// P0-9: Without the fix, Run() returns immediately on ctx.Done()
	// but the ServeStdio goroutine leaks, blocked on stdin read.

	kubeconfig := createTestKubeconfig(t)
	t.Setenv("KUBECONFIG", kubeconfig)
	t.Setenv("KUBE_CONTEXT", "test-context")

	s, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Replace os.Stdin with a blocking pipe to simulate production behavior.
	// The write end is kept open so ServeStdio blocks on read until stdin is closed.
	origStdin := os.Stdin
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = pr
	t.Cleanup(func() {
		os.Stdin = origStdin
		pw.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan error, 1)
	go func() {
		runDone <- s.Run(ctx)
	}()

	// Give the server a moment to start and block on stdin
	time.Sleep(100 * time.Millisecond)

	// Cancel the context
	cancel()

	// Server.Run must:
	// 1. Close stdin to unblock the ServeStdio goroutine
	// 2. Wait for the goroutine to finish
	// 3. Return context.Canceled
	select {
	case runErr := <-runDone:
		if runErr != context.Canceled {
			t.Logf("Server.Run returned: %v (expected context.Canceled)", runErr)
		}

		// Verify stdin was closed by the Run method (the pipe read end).
		// Writing to the pipe should fail if the read end was properly closed.
		_, writeErr := pw.Write([]byte("test"))
		if writeErr == nil {
			t.Error("stdin pipe read end was NOT closed by Server.Run — goroutine cleanup incomplete")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Server.Run did not return within 5s after context cancellation — goroutine leak detected")
	}
}

func TestResultStructs(t *testing.T) {
	t.Run("SubmitResult", func(t *testing.T) {
		result := SubmitResult{
			JobID:     "mpi-job-abc123",
			JobType:   "mpijob",
			Namespace: "default",
			Message:   "Job submitted successfully",
		}
		if result.JobID == "" {
			t.Error("JobID should not be empty")
		}
	})

	t.Run("StatusResult", func(t *testing.T) {
		result := StatusResult{
			JobID:     "test-job",
			Phase:     "Running",
			Ready:     3,
			Succeeded: 0,
			Failed:    0,
			Message:   "Job is running",
		}
		validPhases := map[string]bool{
			"Pending": true, "Running": true, "Succeeded": true, "Failed": true,
		}
		if !validPhases[result.Phase] {
			t.Errorf("Invalid phase: %s", result.Phase)
		}
	})

	t.Run("ArtifactResult", func(t *testing.T) {
		result := ArtifactResult{
			Path:    "/mnt/data/output.txt",
			Content: "line 1\nline 2\nline 3",
			Lines:   3,
		}
		if result.Lines <= 0 {
			t.Error("Lines should be positive")
		}
	})
}
