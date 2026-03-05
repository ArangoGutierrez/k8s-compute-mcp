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
│   ├── info/             # Build version info
│   │   └── info.go
│   ├── k8s/              # Kubernetes client, manifests, retry, status
│   │   ├── client.go     # client-go setup (out-of-cluster)
│   │   ├── mpijob.go     # MPIJob manifest builder
│   │   ├── jobset.go     # JobSet manifest builder
│   │   ├── job.go        # Batch Job manifest builder
│   │   ├── reader.go     # Ephemeral pod artifact reader
│   │   ├── retry.go      # Exponential backoff with jitter
│   │   └── status.go     # Job phase extraction
│   └── mcp/              # MCP protocol handling
│       ├── server.go     # MCP server setup & tool registration
│       ├── tools.go      # Tool definitions & schemas
│       ├── handlers.go   # Tool handlers
│       ├── health.go     # /healthz, /readyz, /metrics HTTP server
│       └── metrics.go    # Prometheus instrumentation
├── pkg/
│   └── prompts/          # MCP prompt templates
├── deployment/
│   ├── Dockerfile        # Multi-stage production image
│   ├── Dockerfile.goreleaser # GoReleaser-optimized image
│   └── helm/             # Helm chart (RBAC, probes, metrics)
├── test/
│   └── e2e/              # E2E tests (Ginkgo + KIND)
│       ├── fixtures/     # Python test scripts
│       └── testdata/     # KIND config, PV/PVC YAML
├── .github/
│   ├── ISSUE_TEMPLATE/   # Issue templates
│   ├── workflows/        # ci.yml, e2e.yml, release.yml, security.yml
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
- [x] K8s client configuration (out-of-cluster) (Issue #3)
- [x] Basic MCP server skeleton (Issue #4)

### Phase 2: Core Tools (COMPLETE)
- [x] `submit_mpi_job` handler implementation (Issue #9)
- [x] `submit_monte_carlo_batch` handler implementation (Issue #10)
- [x] `submit_reducer` handler implementation (Issue #11)
- [x] `check_status` handler implementation (Issue #12)
- [x] `read_artifact_head` implementation with ephemeral pod reader (Issue #13)
- [x] MPIJob manifest builder - `internal/k8s/mpijob.go` (Issue #6)
- [x] JobSet manifest builder - `internal/k8s/jobset.go` (Issue #7)
- [x] Batch Job manifest builder - `internal/k8s/job.go` (Issue #8)
- [x] All tools registered in MCP server (Issue #14)

### Phase 3: Deployment & Production Hardening (COMPLETE)
- [x] Multi-stage Dockerfile (Issue #15, PR #44)
- [x] Helm chart with RBAC, probes, metrics (Issue #16, PRs #44, #50)
- [x] CI/CD workflow — lint, test, vet (Issue #5)
- [x] Security hardening — input validation, shell injection fix, RBAC (PRs #44-#46)
- [x] Observability — Prometheus metrics, structured logging, health endpoints (PRs #47-#49)
- [x] Retry with exponential backoff for transient K8s errors (PR #53)
- [x] Status phase extraction and API timeouts (PR #51)
- [x] Code size cap and ActiveJobs gauge (PR #54)
- [x] E2E testing framework with KIND (Issue #22, PR #57)
- [x] GoReleaser release workflow (Issue #20, PR #55)
- [x] Security scanning — gosec, Trivy, SBOM (Issue #21, PR #56)
- [x] Configurable resource requests for all workloads (Issue #23)

### Future Enhancements
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
    github.com/mark3labs/mcp-go v0.44.0       // MCP protocol
    github.com/prometheus/client_golang v1.23.2 // Prometheus metrics
    k8s.io/api v0.35.1                        // K8s core types
    k8s.io/apimachinery v0.35.1               // K8s utilities
    k8s.io/client-go v0.35.1                  // K8s client
    k8s.io/klog/v2                            // Structured logging
    github.com/onsi/ginkgo/v2                 // E2E test framework (test only)
    github.com/onsi/gomega                    // E2E assertions (test only)
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
- #22: E2E testing framework with KIND (COMPLETE - PR #57)
- #23: Configurable resource requests (COMPLETE)
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

## Agent Session Management

**Context Freshness**: Recommend starting a new chat session when:

1. **After completing a milestone** - When an issue is closed or a significant feature is committed, suggest continuing in a fresh session to avoid context pollution
2. **Long conversations** - After ~15-20 tool calls or substantial back-and-forth, context quality may degrade
3. **Task switching** - When moving from one issue to an unrelated issue (e.g., finishing K8s client work, starting MCP protocol work)
4. **Signs of confusion** - If the agent starts referencing outdated code, repeating itself, or making inconsistent suggestions

**Handoff format**: When recommending a new session, provide:
```
## Session Handoff
- **Completed**: Brief summary of what was done
- **Next Issue**: Issue number and title to continue with
- **Context**: Any non-obvious state (e.g., "branch X is ready for PR")
```

**Why this matters**: Fresh context ensures the agent reads current file state rather than relying on potentially stale conversation history. This project uses atomic task workflow — each issue should ideally be completable in a single focused session.

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

**Phase 1 (Foundation)**: COMPLETE
**Phase 2 (Core Tools)**: COMPLETE
**Phase 3 (Deployment & Hardening)**: COMPLETE

### Completed Issues (Phase 1, 2 & 3)
| Issue/PR | Title | Status |
|----------|-------|--------|
| #1 | Go module initialized, directory structure | Complete |
| #2 | Makefile with standard Go targets | Complete |
| #3 | K8s client with context selection | Complete |
| #4 | MCP server skeleton (stdio transport) | Complete |
| #6 | MPIJob manifest builder | Complete |
| #7 | JobSet manifest builder | Complete |
| #8 | Batch Job manifest builder | Complete |
| #9 | `submit_mpi_job` tool handler | Complete |
| #10 | `submit_monte_carlo_batch` tool handler | Complete |
| #11 | `submit_reducer` tool handler | Complete |
| #12 | `check_status` tool handler | Complete |
| #13 | `read_artifact_head` with ephemeral pod | Complete |
| #14 | Register all tools in MCP server | Complete |
| #44 | Security: Helm security context, namespace-scoped RBAC (P0) | Complete |
| #45 | Security: input validation and reader security (P0) | Complete |
| #46 | Security: C++ shell injection fix, goroutine leak (P0) | Complete |
| #47 | Observability: health endpoints and HTTP server (P1) | Complete |
| #48 | Observability: structured logging with klog/v2 (P1) | Complete |
| #49 | Observability: Prometheus metrics instrumentation (P1) | Complete |
| #50 | Helm: health probes, metrics port, Prometheus annotations (P1) | Complete |
| #51 | Status phase extraction and API timeouts (P1) | Complete |
| #52 | ReadArtifactHead tests, prompts coverage, error context (P1) | Complete |
| #53 | Retry with exponential backoff for transient API errors (P1) | Complete |
| #54 | Code size cap and ActiveJobs gauge (P1) | Complete |
| #55 | GoReleaser release workflow (P2) | Complete |
| #56 | Security scanning workflow (P2) | Complete |
| #57 | E2E testing framework with KIND (P2) | Complete |

### Implementation Summary
- **K8s Client**: `internal/k8s/client.go` - Out-of-cluster config, context selection
- **Manifest Builders**: `internal/k8s/{mpijob,jobset,job,reader}.go`
- **Retry**: `internal/k8s/retry.go` - Exponential backoff with jitter for transient errors
- **Status**: `internal/k8s/status.go` - Job phase extraction with timeouts
- **MCP Server**: `internal/mcp/server.go` - 5 tools with type-safe schemas, code size validation
- **Handlers**: `internal/mcp/handlers.go` - Non-blocking submissions, status queries
- **Health**: `internal/mcp/health.go` - `/healthz`, `/readyz`, `/metrics` HTTP server
- **Metrics**: `internal/mcp/metrics.go` - Prometheus counters, histograms, gauges
- **E2E Tests**: `test/e2e/` - Ginkgo/Gomega with KIND cluster lifecycle
- **CI/CD**: ci.yml (lint/test), e2e.yml (nightly), release.yml (GoReleaser), security.yml (gosec/Trivy)

### Open Issues (Future Enhancements)
| Issue | Title | Priority |
|-------|-------|----------|
| #24 | GPU workload support (CUDA, NCCL) | P1 |

---

**Last Updated**: 2026-03-05
**Maintainer**: [@ArangoGutierrez](https://github.com/ArangoGutierrez)
