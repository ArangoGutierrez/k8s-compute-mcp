// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/k8s"
)

// testServer creates a Server with mock K8s clients for testing handlers.
// This allows testing handler logic without a real K8s cluster.
// Deprecated: Use testServerWithMock instead.
func testServer(t *testing.T, objects ...runtime.Object) *Server {
	t.Helper()

	// Create fake clientset with any pre-existing objects
	//nolint:staticcheck // SA1019: NewSimpleClientset is deprecated but works for tests
	fakeClientset := kubefake.NewSimpleClientset(objects...)

	// Create fake dynamic client with CRD scheme
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)

	// Suppress unused variable warnings - these were used before mock client
	_ = fakeClientset
	_ = fakeDynamic

	// Build server with test config using mock client
	return &Server{
		mcpServer: nil, // Not needed for handler tests
		k8sClient: newMockK8sClient(),
		config: ServerConfig{
			PVCName:      "test-pvc",
			PVCMountPath: "/mnt/data",
			KueueQueue:   "test-queue",
			DefaultImage: "python:3.11-slim",
		},
	}
}

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

func (m *mockK8sClient) Namespace() string { return m.namespace }
func (m *mockK8sClient) Context() string   { return m.context }

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
func testServerWithMock(mock *mockK8sClient) *Server {
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
