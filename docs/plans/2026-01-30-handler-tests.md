# Handler Unit Tests Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add comprehensive unit tests for all MCP handler functions in `internal/mcp/handlers.go` to bring coverage from 0% to >80%.

**Architecture:** Create a testable `Server` by injecting a mock K8s client that implements the required interface. Use table-driven tests with dependency injection pattern. Test both success paths and error conditions for all 5 MCP tool handlers plus utility functions.

**Tech Stack:** Go 1.25+, `testing` package, `k8s.io/client-go/kubernetes/fake`, `k8s.io/client-go/dynamic/fake`

---

## Background

The `internal/mcp/handlers.go` file contains:
- 3 utility functions: `generateJobName`, `resolveNamespace`, `validateCode`
- 5 MCP tool handlers: `handleSubmitMPIJob`, `handleSubmitMonteCarlo`, `handleSubmitReducer`, `handleCheckStatus`, `handleReadArtifactHead`
- 3 status helpers: `getMPIJobStatus`, `getJobSetStatus`, `getBatchJobStatus`
- 1 validation helper: `validateArtifactPath`

Current coverage: **0%**. Target: **>80%**.

## Testing Strategy

1. Create `internal/mcp/handlers_test.go` with a testable server factory
2. Mock the K8s client using existing `NewMockClient` from `internal/k8s/client_test.go`
3. Use table-driven tests for comprehensive coverage
4. Test success paths, validation errors, and K8s API errors

---

## Task 1: Create Test Infrastructure

**Files:**
- Create: `internal/mcp/handlers_test.go`

**Step 1: Write the test helper and mock server factory**

```go
// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/ArangoGutierrez/k8s-compute-mcp/internal/k8s"
)

// testServer creates a Server with mock K8s clients for testing handlers.
// This allows testing handler logic without a real K8s cluster.
func testServer(t *testing.T, objects ...runtime.Object) *Server {
	t.Helper()

	// Create fake clientset with any pre-existing objects
	//nolint:staticcheck // SA1019: NewSimpleClientset is deprecated but works for tests
	fakeClientset := kubefake.NewSimpleClientset(objects...)

	// Create fake dynamic client with CRD scheme
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create mock K8s client with injected fakes
	mockClient := &k8s.Client{}
	// Use reflection or exported setters to inject fakes
	// For now, we'll create a wrapper that exposes what we need

	// Build server with test config
	return &Server{
		mcpServer: nil, // Not needed for handler tests
		k8sClient: &k8s.Client{},
		config: ServerConfig{
			PVCName:      "test-pvc",
			PVCMountPath: "/mnt/data",
			KueueQueue:   "test-queue",
			DefaultImage: "python:3.11-slim",
		},
	}
}
```

**Step 2: Run test to verify it compiles**

Run: `go build ./internal/mcp/`
Expected: Build succeeds (we'll fix the mock injection next)

**Step 3: Commit initial test infrastructure**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add handler test infrastructure"
```

---

## Task 2: Test Utility Functions

**Files:**
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Write tests for `generateJobName`**

```go
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
```

**Step 2: Write tests for `validateCode`**

```go
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
```

**Step 3: Write tests for `validateArtifactPath`**

```go
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
```

**Step 4: Run tests**

Run: `go test -v ./internal/mcp/ -run "TestGenerateJobName|TestValidateCode|TestValidateArtifactPath"`
Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add utility function tests"
```

---

## Task 3: Create Testable K8s Client Interface

**Files:**
- Modify: `internal/k8s/client.go:166-175` (add interface)
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Define K8s client interface for testability**

Add to `internal/k8s/client.go` after the Client struct:

```go
// K8sClientInterface defines the methods used by MCP handlers.
// This interface enables mocking for unit tests.
type K8sClientInterface interface {
	Namespace() string
	Context() string
	SubmitMPIJob(ctx context.Context, mpijob *unstructured.Unstructured) error
	SubmitJobSet(ctx context.Context, jobset *unstructured.Unstructured) error
	SubmitJob(ctx context.Context, job *batchv1.Job) error
	GetMPIJobStatus(ctx context.Context, namespace, name string) (string, error)
	GetJobSetStatus(ctx context.Context, namespace, name string) (string, error)
	GetJobStatus(ctx context.Context, namespace, name string) (*batchv1.JobStatus, error)
	ReadArtifactHead(ctx context.Context, cfg ReadArtifactConfig) (*ReadArtifactResult, error)
}

// Ensure Client implements K8sClientInterface
var _ K8sClientInterface = (*Client)(nil)
```

**Step 2: Run build to verify interface is satisfied**

Run: `go build ./internal/k8s/`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/k8s/client.go
git commit -m "refactor(k8s): add K8sClientInterface for testability"
```

---

## Task 4: Create Mock K8s Client for Handler Tests

**Files:**
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Add mock K8s client implementation**

```go
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
	submittedMPIJobs  []*unstructured.Unstructured
	submittedJobSets  []*unstructured.Unstructured
	submittedJobs     []*batchv1.Job
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
```

**Step 2: Run build**

Run: `go build ./internal/mcp/`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add mock K8s client for handler tests"
```

---

## Task 5: Update Server to Use Interface

**Files:**
- Modify: `internal/mcp/server.go:61-65`

**Step 1: Change k8sClient type to interface**

Update the Server struct in `internal/mcp/server.go`:

```go
// Server represents the MCP server instance.
type Server struct {
	mcpServer *server.MCPServer
	k8sClient k8s.K8sClientInterface  // Changed from *k8s.Client
	config    ServerConfig
}
```

**Step 2: Run build and tests**

Run: `go build ./... && go test ./...`
Expected: Build and existing tests pass

**Step 3: Commit**

```bash
git add internal/mcp/server.go
git commit -m "refactor(mcp): use K8sClientInterface for dependency injection"
```

---

## Task 6: Add Test Helper for Creating Test Server

**Files:**
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Add testServerWithMock helper**

```go
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
```

**Step 2: Run build**

Run: `go build ./internal/mcp/`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add testServerWithMock helper"
```

---

## Task 7: Test handleSubmitMPIJob Handler

**Files:**
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Write table-driven tests for handleSubmitMPIJob**

```go
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
```

**Step 2: Run tests**

Run: `go test -v ./internal/mcp/ -run TestHandleSubmitMPIJob`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add handleSubmitMPIJob tests"
```

---

## Task 8: Test handleSubmitMonteCarlo Handler

**Files:**
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Write table-driven tests for handleSubmitMonteCarlo**

```go
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
```

**Step 2: Run tests**

Run: `go test -v ./internal/mcp/ -run TestHandleSubmitMonteCarlo`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add handleSubmitMonteCarlo tests"
```

---

## Task 9: Test handleSubmitReducer Handler

**Files:**
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Write table-driven tests for handleSubmitReducer**

```go
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
```

**Step 2: Run tests**

Run: `go test -v ./internal/mcp/ -run TestHandleSubmitReducer`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add handleSubmitReducer tests"
```

---

## Task 10: Test handleCheckStatus Handler

**Files:**
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Write table-driven tests for handleCheckStatus**

```go
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
```

**Step 2: Run tests**

Run: `go test -v ./internal/mcp/ -run TestHandleCheckStatus`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add handleCheckStatus tests"
```

---

## Task 11: Test handleReadArtifactHead Handler

**Files:**
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Write table-driven tests for handleReadArtifactHead**

```go
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
```

**Step 2: Run tests**

Run: `go test -v ./internal/mcp/ -run TestHandleReadArtifactHead`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add handleReadArtifactHead tests"
```

---

## Task 12: Test resolveNamespace Helper

**Files:**
- Modify: `internal/mcp/handlers_test.go`

**Step 1: Write tests for resolveNamespace**

```go
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
```

**Step 2: Run tests**

Run: `go test -v ./internal/mcp/ -run TestResolveNamespace`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/mcp/handlers_test.go
git commit -m "test(mcp): add resolveNamespace tests"
```

---

## Task 13: Run Full Test Suite and Measure Coverage

**Files:**
- None (verification only)

**Step 1: Run all tests with race detector**

Run: `go test -v -race ./internal/mcp/`
Expected: All tests pass

**Step 2: Generate coverage report**

Run: `go test -coverprofile=coverage.out ./internal/mcp/ && go tool cover -func=coverage.out | grep handlers`
Expected: Coverage for `handlers.go` should be >80%

**Step 3: Verify total coverage increase**

Run: `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1`
Expected: Total coverage should increase from ~44% to >55%

**Step 4: Commit if all checks pass**

```bash
git add -A
git commit -m "test(mcp): complete handler test coverage"
```

---

## Task 14: Update AGENTS.md with Coverage Status

**Files:**
- Modify: `AGENTS.md:313`

**Step 1: Update the test coverage line**

Update the Implementation Summary section:

```markdown
### Implementation Summary
- **K8s Client**: `internal/k8s/client.go` - Out-of-cluster config, context selection
- **Manifest Builders**: `internal/k8s/{mpijob,jobset,job,reader}.go`
- **MCP Server**: `internal/mcp/server.go` - 5 tools registered with type-safe schemas
- **Handlers**: `internal/mcp/handlers.go` - Non-blocking submissions, status queries
- **Test Coverage**: >55% (handlers and manifest builders well-tested)
```

**Step 2: Commit documentation update**

```bash
git add AGENTS.md
git commit -m "docs: update test coverage status in AGENTS.md"
```

---

## Completion Checklist

- [ ] Task 1: Test infrastructure created
- [ ] Task 2: Utility function tests (generateJobName, validateCode, validateArtifactPath)
- [ ] Task 3: K8sClientInterface defined
- [ ] Task 4: Mock K8s client implementation
- [ ] Task 5: Server uses interface for DI
- [ ] Task 6: testServerWithMock helper
- [ ] Task 7: handleSubmitMPIJob tests
- [ ] Task 8: handleSubmitMonteCarlo tests
- [ ] Task 9: handleSubmitReducer tests
- [ ] Task 10: handleCheckStatus tests
- [ ] Task 11: handleReadArtifactHead tests
- [ ] Task 12: resolveNamespace tests
- [ ] Task 13: Full test suite verification
- [ ] Task 14: Documentation update

## Success Criteria

1. All handler functions have unit tests
2. Coverage for `internal/mcp/handlers.go` > 80%
3. Total project coverage > 55%
4. All tests pass with race detector
5. Tests are table-driven and follow Go best practices

---

## Reference Files

- `internal/mcp/handlers.go` - Target file for testing
- `internal/mcp/server_test.go` - Existing test patterns
- `internal/k8s/client_test.go` - Mock client patterns
- `internal/k8s/manifests_test.go` - Table-driven test examples
