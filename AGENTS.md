# AGENTS.md - AI Agent Context for k8s-compute-mcp

> **Purpose**: This file provides persistent context for AI coding agents working on this project.
> It helps maintain continuity across sessions and ensures consistent understanding of the codebase.

## Project Identity

**Name**: k8s-compute-mcp  
**Type**: MCP (Model Context Protocol) Server  
**Language**: Go 1.25+  
**License**: Apache-2.0  
**Repository**: https://github.com/ArangoGutierrez/k8s-compute-mcp

## Project Overview

`k8s-compute-mcp` is a local MCP Server (written in Go) that acts as a bridge between an LLM client (Gemini CLI, Claude Desktop, Cursor) and a Kubernetes cluster. It enables LLMs to offload heavy mathematical and financial computations to the cluster using specialized operators.

### Target Architecture

- **Interface**: MCP Server over Stdio/SSE (JSON-RPC 2.0)
- **Runtime**: Go with `k8s.io/client-go`
- **Target Cluster**: Kubernetes with Kueue, MPI-Operator (Kubeflow), JobSet controllers
- **Storage**: Shared PVC (`ReadWriteMany`) mounted at `/mnt/data`

## Core Tools (MCP Endpoints)

| Tool | Description | K8s Resource |
|------|-------------|--------------|
| `submit_mpi_job` | Submit MPI workloads (Python/C++) | `kubeflow.org/v2beta1 MPIJob` |
| `submit_monte_carlo_batch` | Submit parallel Monte Carlo simulations | `jobset.x-k8s.io/v1alpha2 JobSet` |
| `submit_reducer` | Submit result aggregation jobs | `batch/v1 Job` |
| `check_status` | Query job/workload status | Various |
| `read_artifact_head` | Read first N lines of result files | PVC access |

### Critical Constraints

1. **Non-blocking submissions**: All `submit_*` tools MUST return immediately with Job ID
2. **Data race prevention**: Monte Carlo jobs MUST inject `JOB_COMPLETION_INDEX` into output paths
3. **Out-of-cluster config**: Use `~/.kube/config` (not in-cluster ServiceAccount)
4. **Typed manifests**: Use Go structs or `unstructured.Unstructured`, NOT raw YAML strings

## Directory Structure

```
k8s-compute-mcp/
├── cmd/
│   └── server/           # Main entry point
│       └── main.go
├── internal/
│   ├── k8s/              # Kubernetes client and manifest generation
│   │   ├── client.go     # client-go setup (out-of-cluster)
│   │   ├── mpijob.go     # MPIJob manifest builder
│   │   ├── jobset.go     # JobSet manifest builder
│   │   └── job.go        # Batch Job manifest builder
│   └── mcp/              # MCP protocol handling
│       ├── server.go     # MCP server setup
│       ├── tools.go      # Tool registration
│       └── handlers.go   # Tool handlers
├── pkg/
│   └── prompts/          # MCP prompt templates
├── deployment/
│   └── helm/             # Helm chart (only Helm, no Kustomize)
├── manifests/            # Example CRDs for reference
├── examples/             # Example MCP requests (JSON-RPC)
├── docs/                 # Documentation
├── test/
│   └── e2e/              # End-to-end tests
├── .github/
│   ├── ISSUE_TEMPLATE/   # Issue templates
│   ├── workflows/        # CI/CD
│   └── PULL_REQUEST_TEMPLATE.md
├── AGENTS.md             # This file
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
├── LICENSE
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

## Go Patterns & Conventions

### Error Handling
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to submit MPI job %s: %w", jobName, err)
}
```

### Context Propagation
```go
// All K8s operations must accept context
func (c *Client) SubmitMPIJob(ctx context.Context, job *MPIJob) (string, error)
```

### Interface-First Design
```go
// Define interfaces for testability
type JobSubmitter interface {
    SubmitMPIJob(ctx context.Context, job *MPIJob) (string, error)
    SubmitJobSet(ctx context.Context, js *JobSet) (string, error)
}
```

### Typed Kubernetes Resources
```go
// Use typed structs, not raw YAML
import mpiv2beta1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"

func BuildMPIJob(name string, replicas int, code string) *mpiv2beta1.MPIJob {
    return &mpiv2beta1.MPIJob{
        // ...
    }
}
```

## Kubernetes CRD References

### MPIJob (kubeflow.org/v2beta1)
- API Docs: https://github.com/kubeflow/mpi-operator
- Requires: `mpi-operator` installed in cluster
- Key fields: `spec.mpiReplicaSpecs`, `spec.slotsPerWorker`

### JobSet (jobset.x-k8s.io/v1alpha2)
- API Docs: https://github.com/kubernetes-sigs/jobset
- Requires: `jobset-controller` installed in cluster
- Key fields: `spec.replicatedJobs`, `spec.successPolicy`

### Kueue Integration
- API Docs: https://kueue.sigs.k8s.io/
- LocalQueue annotation: `kueue.x-k8s.io/queue-name`

## Development Milestones

### Phase 1: Foundation
- [x] Repository setup and Go module initialization (Issue #1)
- [x] Directory structure scaffolding (Issue #1)
- [ ] K8s client configuration (out-of-cluster) (Issue #3)
- [ ] Basic MCP server skeleton (Issue #4)

### Phase 2: Core Tools
- [ ] `submit_mpi_job` implementation
- [ ] `submit_monte_carlo_batch` implementation
- [ ] `submit_reducer` implementation
- [ ] `check_status` implementation
- [ ] `read_artifact_head` implementation

### Phase 3: Deployment
- [ ] Dockerfile (multi-stage build)
- [ ] Helm chart
- [ ] Example manifests
- [ ] CI/CD workflows

### Future Enhancements
- [ ] E2E testing with KIND/minikube (Issue #22)
- [ ] Configurable resource requests (CPU/memory/GPU) (Issue #23)
- [ ] GPU workload support (CUDA, NCCL) (Issue #24)

## Testing Strategy

### Unit Tests
- Mock K8s client using `fake.NewSimpleClientset()`
- Table-driven tests for manifest generation
- Test error paths explicitly

### Integration Tests
- Use Kind cluster with mock operators
- Test actual CRD submission (dry-run)

### E2E Tests
- Full workflow: submit → status → read results

## Dependencies (Core)

```go
require (
    github.com/mark3labs/mcp-go v0.43.2      // MCP protocol
    k8s.io/api v0.35.0                        // K8s core types
    k8s.io/apimachinery v0.35.0               // K8s utilities
    k8s.io/client-go v0.35.0                  // K8s client
    github.com/kubeflow/mpi-operator          // MPIJob types
    sigs.k8s.io/jobset                        // JobSet types
)
```

## Security Considerations

1. **No secrets in code**: Use K8s secrets or environment variables
2. **RBAC**: Document minimum required permissions
3. **Input validation**: Validate all user-provided code before submission
4. **Resource limits**: Enforce resource quotas in generated manifests

## GPU Support (Future)

### Resource Requests
Jobs should support configurable resources including GPU:
```go
type ResourceSpec struct {
    CPU     string `json:"cpu,omitempty"`      // "100m", "2"
    Memory  string `json:"memory,omitempty"`   // "256Mi", "4Gi"
    GPU     int    `json:"gpu,omitempty"`      // Number of GPUs
    GPUType string `json:"gpu_type,omitempty"` // "nvidia.com/gpu"
}
```

### RuntimeClass
GPU workloads require RuntimeClass configuration:
- `nvidia` RuntimeClass for NVIDIA GPU Operator
- Tolerations for GPU node taints
- Node selectors for GPU nodes

### Related Issues
- #22: E2E testing framework with KIND/minikube
- #23: Configurable resource requests
- #24: GPU workload support (CUDA, NCCL)

## Common Agent Tasks

### Adding a New Tool
1. Define input struct in `internal/mcp/tools.go`
2. Implement handler in `internal/mcp/handlers.go`
3. Register tool in `internal/mcp/server.go`
4. Add manifest builder in `internal/k8s/`
5. Write unit tests
6. Add example request in `examples/`

### Debugging K8s Issues
1. Check kubeconfig: `kubectl config current-context`
2. Verify CRDs installed: `kubectl get crd | grep -E 'mpijob|jobset'`
3. Check operator status: `kubectl get pods -n kubeflow`

### Running Locally
```bash
# Build
make build

# Run with stdio
./bin/server

# Test with example
cat examples/submit_mpi.json | ./bin/server
```

## Git Workflow

- **Branch naming**: `feat/`, `fix/`, `chore/`, `docs/`
- **Commit format**: `type(scope): description`
- **Signing**: DCO (`-s`) and GPG (`-S`) required
- **PR template**: Fill all checkboxes before requesting review

## Related Projects

- [k8s-gpu-mcp-server](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server) - Sister project for GPU diagnostics
- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP Go implementation
- [mpi-operator](https://github.com/kubeflow/mpi-operator) - Kubeflow MPI Operator
- [jobset](https://github.com/kubernetes-sigs/jobset) - Kubernetes JobSet controller

---

## Current Status

### Completed Issues
- **#1**: Go module initialized, directory structure created (commit `34851b7`)
- **#2**: Makefile with standard Go targets (commit `e6f4d4d`)

### Next Steps (P0 Priority)
1. **Issue #3**: Configure K8s client with out-of-cluster kubeconfig
2. **Issue #4**: Set up basic MCP server skeleton (stdio transport)

### Open Issues Summary
| Issue | Title | Priority |
|-------|-------|----------|
| #3 | K8s client (out-of-cluster) | P0 |
| #4 | MCP server skeleton | P0 |
| #5 | CI workflow | P1 |
| #6-8 | Manifest builders | P0 |
| #9-14 | MCP tool implementations | P0 |
| #15-21 | Deployment/Docs | P1-P2 |
| #22 | E2E testing with KIND | P1 |
| #23 | Configurable resources | P1 |
| #24 | GPU workload support | P1 |

---

**Last Updated**: 2026-01-23  
**Maintainer**: [@ArangoGutierrez](https://github.com/ArangoGutierrez)
