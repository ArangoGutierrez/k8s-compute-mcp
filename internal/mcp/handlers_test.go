// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/k8s"
)

// mockK8sClient is a mock implementation of k8s.K8sClientInterface for testing.
type mockK8sClient struct {
	namespace string
	context   string

	// Control mock behavior
	submitMPIJobErr    error
	submitJobSetErr    error
	submitJobErr       error
	getMPIJobStatus    string
	getMPIJobStatusErr error
	getJobSetStatus    string
	getJobSetStatusErr error
	getJobStatus       *batchv1.JobStatus
	getJobStatusErr    error
	readArtifactResult *k8s.ReadArtifactResult
	readArtifactErr    error

	// Capture calls for verification
	submittedMPIJobs []*unstructured.Unstructured
	submittedJobSets []*unstructured.Unstructured
	submittedJobs    []*batchv1.Job
}

func (m *mockK8sClient) Namespace() string            { return m.namespace }
func (m *mockK8sClient) Context() string              { return m.context }
func (m *mockK8sClient) Ping(_ context.Context) error { return nil }

func (m *mockK8sClient) SubmitMPIJob(ctx context.Context, mpijob *unstructured.Unstructured) error {
	m.submittedMPIJobs = append(m.submittedMPIJobs, mpijob)
	return m.submitMPIJobErr
}

func (m *mockK8sClient) SubmitJobSet(ctx context.Context, jobset *unstructured.Unstructured) error {
	m.submittedJobSets = append(m.submittedJobSets, jobset)
	return m.submitJobSetErr
}

func (m *mockK8sClient) SubmitJob(ctx context.Context, job *batchv1.Job) error {
	m.submittedJobs = append(m.submittedJobs, job)
	return m.submitJobErr
}

func (m *mockK8sClient) GetMPIJobStatus(ctx context.Context, namespace, name string) (string, error) {
	return m.getMPIJobStatus, m.getMPIJobStatusErr
}

func (m *mockK8sClient) GetJobSetStatus(ctx context.Context, namespace, name string) (string, error) {
	return m.getJobSetStatus, m.getJobSetStatusErr
}

func (m *mockK8sClient) GetJobStatus(ctx context.Context, namespace, name string) (*batchv1.JobStatus, error) {
	return m.getJobStatus, m.getJobStatusErr
}

func (m *mockK8sClient) ReadArtifactHead(ctx context.Context, cfg k8s.ReadArtifactConfig) (*k8s.ReadArtifactResult, error) {
	return m.readArtifactResult, m.readArtifactErr
}

// newMockK8sClient creates a mock client with default success behavior.
func newMockK8sClient() *mockK8sClient {
	return &mockK8sClient{
		namespace:       "default",
		context:         "test-context",
		getMPIJobStatus: "Running",
		getJobSetStatus: "Running",
		getJobStatus:    &batchv1.JobStatus{Active: 1},
		readArtifactResult: &k8s.ReadArtifactResult{
			Content: "test content\n",
			Lines:   1,
		},
	}
}

// testServerWithMock creates a Server with the provided mock K8s client.
// Also initializes metrics with a fresh registry to avoid nil pointer panics.
func testServerWithMock(mock *mockK8sClient) *Server {
	InitMetricsWithRegistry(prometheus.NewRegistry())
	return &Server{
		mcpServer: nil, // Not needed for handler tests
		k8sClient: mock,
		config: ServerConfig{
			PVCName:      "test-pvc",
			PVCMountPath: "/mnt/data",
			KueueQueue:   "test-queue",
			DefaultImage: "python:3.11-slim",
		},
	}
}

// TestGenerateJobName verifies job name generation with unique timestamps.
func TestGenerateJobName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string // prefix pattern
	}{
		{
			name:   "mpi prefix",
			prefix: "mpi",
			want:   "mpi-",
		},
		{
			name:   "mc prefix",
			prefix: "mc",
			want:   "mc-",
		},
		{
			name:   "reduce prefix",
			prefix: "reduce",
			want:   "reduce-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateJobName(tt.prefix)

			// Verify prefix
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("generateJobName(%q) = %q, want prefix %q", tt.prefix, got, tt.want)
			}

			// Verify uniqueness (generate two and compare)
			got2 := generateJobName(tt.prefix)
			if got == got2 {
				t.Errorf("generateJobName() should generate unique names, got same name twice: %q", got)
			}

			// Verify format: prefix-YYYYMMDD-HHMMSS-xxxxx
			parts := strings.Split(got, "-")
			if len(parts) < 4 {
				t.Errorf("generateJobName(%q) = %q, expected format prefix-date-time-random", tt.prefix, got)
			}
		})
	}
}

// TestValidateCode verifies code validation logic.
func TestValidateCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{
			name:    "valid python code",
			code:    "print('hello world')",
			wantErr: false,
		},
		{
			name:    "valid multiline code",
			code:    "import os\nprint(os.getcwd())",
			wantErr: false,
		},
		{
			name:    "empty code",
			code:    "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			code:    "   \n\t  ",
			wantErr: true,
		},
		{
			name:    "code exceeds max size",
			code:    strings.Repeat("x", maxCodeBytes+1),
			wantErr: true,
		},
		{
			name:    "code at exact max size is valid",
			code:    strings.Repeat("x", maxCodeBytes),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCode(%q) error = %v, wantErr %v", tt.code, err, tt.wantErr)
			}
		})
	}
}

// TestValidateArtifactPath verifies path security validation.
func TestValidateArtifactPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		mountPath string
		wantErr   bool
	}{
		{
			name:      "valid path",
			path:      "/mnt/data/results/output.json",
			mountPath: "/mnt/data",
			wantErr:   false,
		},
		{
			name:      "valid nested path",
			path:      "/mnt/data/job-123/replica-0/result.csv",
			mountPath: "/mnt/data",
			wantErr:   false,
		},
		{
			name:      "empty path",
			path:      "",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "path outside mount",
			path:      "/etc/passwd",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "path traversal attempt",
			path:      "/mnt/data/../etc/passwd",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "double dot in path",
			path:      "/mnt/data/results/../../secret",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "empty mount path",
			path:      "/mnt/data/results/output.json",
			mountPath: "",
			wantErr:   true,
		},
		{
			name:      "mount path with trailing slash",
			path:      "/mnt/data/results/output.json",
			mountPath: "/mnt/data/",
			wantErr:   false,
		},
		{
			name:      "mount path exact match with trailing slash",
			path:      "/mnt/data",
			mountPath: "/mnt/data/",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArtifactPath(tt.path, tt.mountPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateArtifactPath(%q, %q) error = %v, wantErr %v",
					tt.path, tt.mountPath, err, tt.wantErr)
			}
		})
	}
}

// TestHandleSubmitMPIJob verifies MPI job submission handler.
func TestHandleSubmitMPIJob(t *testing.T) {
	tests := []struct {
		name        string
		input       SubmitMPIJobInput
		mockSetup   func(*mockK8sClient)
		wantErr     bool
		wantJobType string
	}{
		{
			name: "successful submission",
			input: SubmitMPIJobInput{
				Code:     "print('hello mpi')",
				Language: "python",
				Nodes:    4,
			},
			wantErr:     false,
			wantJobType: "mpijob",
		},
		{
			name: "with namespace",
			input: SubmitMPIJobInput{
				Code:      "print('hello')",
				Language:  "python",
				Nodes:     2,
				Namespace: "custom-ns",
			},
			wantErr:     false,
			wantJobType: "mpijob",
		},
		{
			name: "empty code",
			input: SubmitMPIJobInput{
				Code:     "",
				Language: "python",
				Nodes:    2,
			},
			wantErr: true,
		},
		{
			name: "zero nodes",
			input: SubmitMPIJobInput{
				Code:     "print('test')",
				Language: "python",
				Nodes:    0,
			},
			wantErr: true,
		},
		{
			name: "negative nodes",
			input: SubmitMPIJobInput{
				Code:     "print('test')",
				Language: "python",
				Nodes:    -1,
			},
			wantErr: true,
		},
		{
			name: "k8s submission error",
			input: SubmitMPIJobInput{
				Code:     "print('test')",
				Language: "python",
				Nodes:    2,
			},
			mockSetup: func(m *mockK8sClient) {
				m.submitMPIJobErr = fmt.Errorf("cluster unavailable")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			if tt.mockSetup != nil {
				tt.mockSetup(mock)
			}

			s := testServerWithMock(mock)
			result, err := s.handleSubmitMPIJob(context.Background(), tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleSubmitMPIJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Fatal("handleSubmitMPIJob() returned nil result")
				}
				if result.JobType != tt.wantJobType {
					t.Errorf("JobType = %q, want %q", result.JobType, tt.wantJobType)
				}
				if result.JobID == "" {
					t.Error("JobID should not be empty")
				}
				if !strings.HasPrefix(result.JobID, "mpi-") {
					t.Errorf("JobID = %q, want prefix 'mpi-'", result.JobID)
				}
			}
		})
	}
}

// TestHandleSubmitMonteCarlo verifies Monte Carlo job submission handler.
func TestHandleSubmitMonteCarlo(t *testing.T) {
	tests := []struct {
		name        string
		input       SubmitMonteCarloInput
		mockSetup   func(*mockK8sClient)
		wantErr     bool
		wantJobType string
	}{
		{
			name: "successful submission",
			input: SubmitMonteCarloInput{
				Script:   "import random; print(random.random())",
				Replicas: 10,
			},
			wantErr:     false,
			wantJobType: "jobset",
		},
		{
			name: "with namespace",
			input: SubmitMonteCarloInput{
				Script:    "print('monte carlo')",
				Replicas:  5,
				Namespace: "simulations",
			},
			wantErr:     false,
			wantJobType: "jobset",
		},
		{
			name: "high replica count",
			input: SubmitMonteCarloInput{
				Script:   "print('test')",
				Replicas: 100,
			},
			wantErr:     false,
			wantJobType: "jobset",
		},
		{
			name: "empty script",
			input: SubmitMonteCarloInput{
				Script:   "",
				Replicas: 10,
			},
			wantErr: true,
		},
		{
			name: "zero replicas",
			input: SubmitMonteCarloInput{
				Script:   "print('test')",
				Replicas: 0,
			},
			wantErr: true,
		},
		{
			name: "k8s submission error",
			input: SubmitMonteCarloInput{
				Script:   "print('test')",
				Replicas: 5,
			},
			mockSetup: func(m *mockK8sClient) {
				m.submitJobSetErr = fmt.Errorf("quota exceeded")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			if tt.mockSetup != nil {
				tt.mockSetup(mock)
			}

			s := testServerWithMock(mock)
			result, err := s.handleSubmitMonteCarlo(context.Background(), tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleSubmitMonteCarlo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Fatal("handleSubmitMonteCarlo() returned nil result")
				}
				if result.JobType != tt.wantJobType {
					t.Errorf("JobType = %q, want %q", result.JobType, tt.wantJobType)
				}
				if !strings.HasPrefix(result.JobID, "mc-") {
					t.Errorf("JobID = %q, want prefix 'mc-'", result.JobID)
				}
			}
		})
	}
}

// TestHandleSubmitReducer verifies reducer job submission handler.
func TestHandleSubmitReducer(t *testing.T) {
	tests := []struct {
		name        string
		input       SubmitReducerInput
		mockSetup   func(*mockK8sClient)
		wantErr     bool
		wantJobType string
	}{
		{
			name: "successful submission",
			input: SubmitReducerInput{
				Script: "import glob; files = glob.glob('/mnt/data/*.json')",
			},
			wantErr:     false,
			wantJobType: "job",
		},
		{
			name: "with input pattern",
			input: SubmitReducerInput{
				Script:       "print('aggregating')",
				InputPattern: "/mnt/data/results/*.json",
			},
			wantErr:     false,
			wantJobType: "job",
		},
		{
			name: "with namespace",
			input: SubmitReducerInput{
				Script:    "print('reduce')",
				Namespace: "analytics",
			},
			wantErr:     false,
			wantJobType: "job",
		},
		{
			name: "empty script",
			input: SubmitReducerInput{
				Script: "",
			},
			wantErr: true,
		},
		{
			name: "whitespace script",
			input: SubmitReducerInput{
				Script: "   \n\t",
			},
			wantErr: true,
		},
		{
			name: "k8s submission error",
			input: SubmitReducerInput{
				Script: "print('test')",
			},
			mockSetup: func(m *mockK8sClient) {
				m.submitJobErr = fmt.Errorf("namespace not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			if tt.mockSetup != nil {
				tt.mockSetup(mock)
			}

			s := testServerWithMock(mock)
			result, err := s.handleSubmitReducer(context.Background(), tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleSubmitReducer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Fatal("handleSubmitReducer() returned nil result")
				}
				if result.JobType != tt.wantJobType {
					t.Errorf("JobType = %q, want %q", result.JobType, tt.wantJobType)
				}
				if !strings.HasPrefix(result.JobID, "reduce-") {
					t.Errorf("JobID = %q, want prefix 'reduce-'", result.JobID)
				}
			}
		})
	}
}

// TestHandleCheckStatus verifies job status check handler.
func TestHandleCheckStatus(t *testing.T) {
	tests := []struct {
		name      string
		input     CheckStatusInput
		mockSetup func(*mockK8sClient)
		wantErr   bool
		wantPhase string
	}{
		{
			name: "mpijob running",
			input: CheckStatusInput{
				JobID:   "mpi-test-123",
				JobType: "mpijob",
			},
			mockSetup: func(m *mockK8sClient) {
				m.getMPIJobStatus = "Running"
			},
			wantErr:   false,
			wantPhase: "Running",
		},
		{
			name: "jobset succeeded",
			input: CheckStatusInput{
				JobID:   "mc-test-456",
				JobType: "jobset",
			},
			mockSetup: func(m *mockK8sClient) {
				m.getJobSetStatus = "Succeeded"
			},
			wantErr:   false,
			wantPhase: "Succeeded",
		},
		{
			name: "batch job with counts",
			input: CheckStatusInput{
				JobID:   "reduce-789",
				JobType: "job",
			},
			mockSetup: func(m *mockK8sClient) {
				m.getJobStatus = &batchv1.JobStatus{
					Active:    0,
					Succeeded: 1,
					Failed:    0,
				}
			},
			wantErr:   false,
			wantPhase: "Succeeded",
		},
		{
			name: "batch job failed",
			input: CheckStatusInput{
				JobID:   "reduce-failed",
				JobType: "job",
			},
			mockSetup: func(m *mockK8sClient) {
				m.getJobStatus = &batchv1.JobStatus{
					Active:    0,
					Succeeded: 0,
					Failed:    1,
				}
			},
			wantErr:   false,
			wantPhase: "Failed",
		},
		{
			name: "batch job pending",
			input: CheckStatusInput{
				JobID:   "reduce-pending",
				JobType: "job",
			},
			mockSetup: func(m *mockK8sClient) {
				m.getJobStatus = &batchv1.JobStatus{
					Active:    0,
					Succeeded: 0,
					Failed:    0,
				}
			},
			wantErr:   false,
			wantPhase: "Pending",
		},
		{
			name: "empty job_id",
			input: CheckStatusInput{
				JobID:   "",
				JobType: "mpijob",
			},
			wantErr: true,
		},
		{
			name: "invalid job_type",
			input: CheckStatusInput{
				JobID:   "test-123",
				JobType: "invalid",
			},
			wantErr: true,
		},
		{
			name: "mpijob not found",
			input: CheckStatusInput{
				JobID:   "nonexistent",
				JobType: "mpijob",
			},
			mockSetup: func(m *mockK8sClient) {
				m.getMPIJobStatusErr = fmt.Errorf("mpijob not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			if tt.mockSetup != nil {
				tt.mockSetup(mock)
			}

			s := testServerWithMock(mock)
			result, err := s.handleCheckStatus(context.Background(), tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleCheckStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Fatal("handleCheckStatus() returned nil result")
				}
				if result.Phase != tt.wantPhase {
					t.Errorf("Phase = %q, want %q", result.Phase, tt.wantPhase)
				}
				if result.JobID != tt.input.JobID {
					t.Errorf("JobID = %q, want %q", result.JobID, tt.input.JobID)
				}
			}
		})
	}
}

// TestHandleReadArtifactHead verifies artifact reading handler.
func TestHandleReadArtifactHead(t *testing.T) {
	tests := []struct {
		name        string
		input       ReadArtifactInput
		mockSetup   func(*mockK8sClient)
		wantErr     bool
		wantContent string
		wantLines   int
	}{
		{
			name: "successful read",
			input: ReadArtifactInput{
				Path:  "/mnt/data/results/output.json",
				Lines: 10,
			},
			mockSetup: func(m *mockK8sClient) {
				m.readArtifactResult = &k8s.ReadArtifactResult{
					Content: `{"result": 3.14159}`,
					Lines:   1,
				}
			},
			wantErr:     false,
			wantContent: `{"result": 3.14159}`,
			wantLines:   1,
		},
		{
			name: "default lines",
			input: ReadArtifactInput{
				Path:  "/mnt/data/output.txt",
				Lines: 0, // Should default to 100
			},
			mockSetup: func(m *mockK8sClient) {
				m.readArtifactResult = &k8s.ReadArtifactResult{
					Content: "line1\nline2\nline3",
					Lines:   3,
				}
			},
			wantErr:   false,
			wantLines: 3,
		},
		{
			name: "empty path",
			input: ReadArtifactInput{
				Path:  "",
				Lines: 10,
			},
			wantErr: true,
		},
		{
			name: "path outside mount",
			input: ReadArtifactInput{
				Path:  "/etc/passwd",
				Lines: 10,
			},
			wantErr: true,
		},
		{
			name: "path traversal",
			input: ReadArtifactInput{
				Path:  "/mnt/data/../../../etc/shadow",
				Lines: 10,
			},
			wantErr: true,
		},
		{
			name: "file not found",
			input: ReadArtifactInput{
				Path:  "/mnt/data/nonexistent.txt",
				Lines: 10,
			},
			mockSetup: func(m *mockK8sClient) {
				m.readArtifactErr = fmt.Errorf("file not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			if tt.mockSetup != nil {
				tt.mockSetup(mock)
			}

			s := testServerWithMock(mock)
			result, err := s.handleReadArtifactHead(context.Background(), tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleReadArtifactHead() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Fatal("handleReadArtifactHead() returned nil result")
				}
				if tt.wantContent != "" && result.Content != tt.wantContent {
					t.Errorf("Content = %q, want %q", result.Content, tt.wantContent)
				}
				if tt.wantLines > 0 && result.Lines != tt.wantLines {
					t.Errorf("Lines = %d, want %d", result.Lines, tt.wantLines)
				}
				if result.Path != tt.input.Path {
					t.Errorf("Path = %q, want %q", result.Path, tt.input.Path)
				}
			}
		})
	}
}

// P0-3: Path traversal bypass - validateArtifactPath must use filepath.Clean
// and validate path characters.
func TestValidateArtifactPath_SecurityBypass(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		mountPath string
		wantErr   bool
	}{
		{
			name:      "clean traversal that bypasses prefix check",
			path:      "/mnt/data/../etc/passwd",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "encoded traversal with double slash",
			path:      "/mnt/data//../../etc/shadow",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "path equals mount path should be allowed",
			path:      "/mnt/data",
			mountPath: "/mnt/data",
			wantErr:   false,
		},
		{
			name:      "path with trailing slash cleaned",
			path:      "/mnt/data/",
			mountPath: "/mnt/data",
			wantErr:   false,
		},
		{
			name:      "mount path prefix attack - /mnt/data-evil",
			path:      "/mnt/data-evil/secret",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "null byte injection",
			path:      "/mnt/data/file\x00.txt",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "newline injection",
			path:      "/mnt/data/file\n.txt",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "backtick injection",
			path:      "/mnt/data/`whoami`.txt",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "dollar sign injection",
			path:      "/mnt/data/$HOME/file.txt",
			mountPath: "/mnt/data",
			wantErr:   true,
		},
		{
			name:      "valid path with hyphen and dot",
			path:      "/mnt/data/job-123/result.csv",
			mountPath: "/mnt/data",
			wantErr:   false,
		},
		{
			name:      "valid path with underscore",
			path:      "/mnt/data/my_results/output_001.json",
			mountPath: "/mnt/data",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArtifactPath(tt.path, tt.mountPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateArtifactPath(%q, %q) error = %v, wantErr %v",
					tt.path, tt.mountPath, err, tt.wantErr)
			}
		})
	}
}

// P0-4: Namespace validation - must enforce RFC 1123.
func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name    string
		ns      string
		wantErr bool
	}{
		{
			name:    "valid namespace",
			ns:      "default",
			wantErr: false,
		},
		{
			name:    "valid with hyphens",
			ns:      "my-namespace-123",
			wantErr: false,
		},
		{
			name:    "valid single char",
			ns:      "a",
			wantErr: false,
		},
		{
			name:    "valid single digit",
			ns:      "0",
			wantErr: false,
		},
		{
			name:    "empty namespace",
			ns:      "",
			wantErr: true,
		},
		{
			name:    "uppercase letters",
			ns:      "MyNamespace",
			wantErr: true,
		},
		{
			name:    "spaces",
			ns:      "my namespace",
			wantErr: true,
		},
		{
			name:    "starts with hyphen",
			ns:      "-bad",
			wantErr: true,
		},
		{
			name:    "ends with hyphen",
			ns:      "bad-",
			wantErr: true,
		},
		{
			name:    "special characters",
			ns:      "ns@#$",
			wantErr: true,
		},
		{
			name:    "underscore",
			ns:      "my_namespace",
			wantErr: true,
		},
		{
			name:    "dot",
			ns:      "my.namespace",
			wantErr: true,
		},
		{
			name:    "too long (64 chars)",
			ns:      "a234567890123456789012345678901234567890123456789012345678901234",
			wantErr: true,
		},
		{
			name:    "max length (63 chars)",
			ns:      "a23456789012345678901234567890123456789012345678901234567890123",
			wantErr: false,
		},
		{
			name:    "path traversal attempt",
			ns:      "../etc",
			wantErr: true,
		},
		{
			name:    "semicolon injection",
			ns:      "ns;rm -rf /",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNamespace(tt.ns)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNamespace(%q) error = %v, wantErr %v", tt.ns, err, tt.wantErr)
			}
		})
	}
}

// P0-4: Handlers must reject invalid namespaces.
func TestHandlers_RejectInvalidNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		wantErr   bool
	}{
		{
			name:      "valid namespace",
			namespace: "production",
			wantErr:   false,
		},
		{
			name:      "empty uses default (valid)",
			namespace: "",
			wantErr:   false,
		},
		{
			name:      "uppercase rejected",
			namespace: "INVALID",
			wantErr:   true,
		},
		{
			name:      "special chars rejected",
			namespace: "ns;rm -rf /",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run("mpi_"+tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			s := testServerWithMock(mock)
			_, err := s.handleSubmitMPIJob(context.Background(), SubmitMPIJobInput{
				Code:      "print('hi')",
				Language:  "python",
				Nodes:     2,
				Namespace: tt.namespace,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSubmitMPIJob(ns=%q) error = %v, wantErr %v", tt.namespace, err, tt.wantErr)
			}
		})

		t.Run("mc_"+tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			s := testServerWithMock(mock)
			_, err := s.handleSubmitMonteCarlo(context.Background(), SubmitMonteCarloInput{
				Script:    "print('hi')",
				Replicas:  2,
				Namespace: tt.namespace,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSubmitMonteCarlo(ns=%q) error = %v, wantErr %v", tt.namespace, err, tt.wantErr)
			}
		})

		t.Run("reducer_"+tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			s := testServerWithMock(mock)
			_, err := s.handleSubmitReducer(context.Background(), SubmitReducerInput{
				Script:    "print('hi')",
				Namespace: tt.namespace,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSubmitReducer(ns=%q) error = %v, wantErr %v", tt.namespace, err, tt.wantErr)
			}
		})

		t.Run("status_"+tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			s := testServerWithMock(mock)
			_, err := s.handleCheckStatus(context.Background(), CheckStatusInput{
				JobID:     "test-123",
				JobType:   "mpijob",
				Namespace: tt.namespace,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("handleCheckStatus(ns=%q) error = %v, wantErr %v", tt.namespace, err, tt.wantErr)
			}
		})
	}
}

// TestResolveNamespace verifies namespace resolution logic.
func TestResolveNamespace(t *testing.T) {
	tests := []struct {
		name          string
		inputNS       string
		clientNS      string
		wantNamespace string
	}{
		{
			name:          "use provided namespace",
			inputNS:       "custom-ns",
			clientNS:      "default",
			wantNamespace: "custom-ns",
		},
		{
			name:          "fallback to client namespace",
			inputNS:       "",
			clientNS:      "client-default",
			wantNamespace: "client-default",
		},
		{
			name:          "provided takes precedence",
			inputNS:       "override",
			clientNS:      "ignored",
			wantNamespace: "override",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockK8sClient()
			mock.namespace = tt.clientNS

			s := testServerWithMock(mock)
			got := s.resolveNamespace(tt.inputNS)

			if got != tt.wantNamespace {
				t.Errorf("resolveNamespace(%q) = %q, want %q", tt.inputNS, got, tt.wantNamespace)
			}
		})
	}
}

// TestActiveJobsGauge_IncrementedOnSubmit verifies the ActiveJobs gauge is incremented
// after each successful job submission.
func TestActiveJobsGauge_IncrementedOnSubmit(t *testing.T) {
	_ = newTestRegistry()

	mock := newMockK8sClient()
	s := testServerWithMock(mock)
	ctx := context.Background()

	// Submit an MPIJob
	_, err := s.handleSubmitMPIJob(ctx, SubmitMPIJobInput{
		Code:     "print('hello')",
		Language: "python",
		Nodes:    2,
	})
	if err != nil {
		t.Fatalf("handleSubmitMPIJob error: %v", err)
	}

	mpijobCount := testutil.ToFloat64(ActiveJobs.WithLabelValues("default", "mpijob"))
	if mpijobCount != 1 {
		t.Errorf("ActiveJobs(default, mpijob) = %f, want 1", mpijobCount)
	}

	// Submit a Monte Carlo batch
	_, err = s.handleSubmitMonteCarlo(ctx, SubmitMonteCarloInput{
		Script:   "import random; print(random.random())",
		Replicas: 3,
	})
	if err != nil {
		t.Fatalf("handleSubmitMonteCarlo error: %v", err)
	}

	jobsetCount := testutil.ToFloat64(ActiveJobs.WithLabelValues("default", "jobset"))
	if jobsetCount != 1 {
		t.Errorf("ActiveJobs(default, jobset) = %f, want 1", jobsetCount)
	}

	// Submit a Reducer job
	_, err = s.handleSubmitReducer(ctx, SubmitReducerInput{
		Script: "print('reduce')",
	})
	if err != nil {
		t.Fatalf("handleSubmitReducer error: %v", err)
	}

	jobCount := testutil.ToFloat64(ActiveJobs.WithLabelValues("default", "job"))
	if jobCount != 1 {
		t.Errorf("ActiveJobs(default, job) = %f, want 1", jobCount)
	}
}
