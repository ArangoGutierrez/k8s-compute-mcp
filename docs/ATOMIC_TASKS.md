# Atomic Task Breakdown - k8s-compute-mcp

This document defines all atomic tasks for the k8s-compute-mcp project, organized by phase and priority.

## GitHub Labels Configuration

### Priority Labels
| Label | Color | Description |
|-------|-------|-------------|
| `P0` | `#d73a4a` | Critical - Blocker for release |
| `P1` | `#e99695` | High - Should have |
| `P2` | `#fbca04` | Medium - Nice to have |
| `P3` | `#c5def5` | Low - Future consideration |

### Kind Labels
| Label | Color | Description |
|-------|-------|-------------|
| `kind/bug` | `#d73a4a` | Bug report |
| `kind/feature` | `#a2eeef` | New feature |
| `kind/tech-debt` | `#d4c5f9` | Technical debt |
| `kind/docs` | `#0075ca` | Documentation |
| `kind/enhancement` | `#84b6eb` | Enhancement to existing feature |

### Area Labels
| Label | Color | Description |
|-------|-------|-------------|
| `area/mcp` | `#1d76db` | MCP protocol/transport |
| `area/k8s` | `#326ce5` | Kubernetes integration |
| `area/tools` | `#5319e7` | MCP tool implementations |
| `area/build` | `#bfd4f2` | Build system/CI |
| `area/helm` | `#0e8a16` | Helm chart |
| `area/docs` | `#c5def5` | Documentation |

### Status Labels
| Label | Color | Description |
|-------|-------|-------------|
| `status/blocked` | `#b60205` | Blocked by dependency |
| `status/needs-design` | `#e4e669` | Needs design discussion |
| `status/ready` | `#0e8a16` | Ready to implement |

### Operations Labels
| Label | Color | Description |
|-------|-------|-------------|
| `ops/ci-cd` | `#fef2c0` | CI/CD related |
| `ops/security` | `#ee0701` | Security related |
| `dependencies` | `#0366d6` | Dependency updates |
| `good first issue` | `#7057ff` | Good for newcomers |

---

## Phase 1: Foundation & Repository Setup

### Issue #1: Initialize Go module and project scaffolding
**Priority**: P0 | **Area**: area/build | **Kind**: kind/feature

**Description**:
Initialize the Go module and create the base project structure.

**Acceptance Criteria**:
- [ ] `go mod init github.com/ArangoGutierrez/k8s-compute-mcp`
- [ ] Create directory structure: `cmd/server/`, `internal/k8s/`, `internal/mcp/`, `pkg/prompts/`
- [ ] Create empty placeholder files with package declarations
- [ ] Verify `go build ./...` succeeds

**Files to create**:
```
cmd/server/main.go
internal/k8s/client.go
internal/k8s/mpijob.go
internal/k8s/jobset.go
internal/k8s/job.go
internal/mcp/server.go
internal/mcp/tools.go
internal/mcp/handlers.go
pkg/prompts/library.go
```

---

### Issue #2: Add Apache 2.0 LICENSE file
**Priority**: P0 | **Area**: area/docs | **Kind**: kind/docs

**Description**:
Add the Apache 2.0 license file to the repository root.

**Acceptance Criteria**:
- [ ] LICENSE file present at repository root
- [ ] Copyright year and owner set correctly

---

### Issue #3: Create .gitignore for Go projects
**Priority**: P0 | **Area**: area/build | **Kind**: kind/feature

**Description**:
Create a comprehensive .gitignore file for Go projects.

**Acceptance Criteria**:
- [ ] Ignore build artifacts (`/bin/`, `/dist/`)
- [ ] Ignore IDE files (`.vscode/`, `.idea/`, `.cursor/`)
- [ ] Ignore coverage files (`coverage.out`, `coverage.html`)
- [ ] Ignore OS-specific files (`.DS_Store`)

---

### Issue #4: Create Makefile with standard Go targets
**Priority**: P1 | **Area**: area/build | **Kind**: kind/feature

**Description**:
Create a Makefile following patterns from k8s-gpu-mcp-server.

**Acceptance Criteria**:
- [ ] `make build` - Build all packages
- [ ] `make test` - Run unit tests with race detector
- [ ] `make lint` - Run golangci-lint
- [ ] `make fmt` - Format code
- [ ] `make clean` - Clean build artifacts
- [ ] `make help` - Display available targets

---

### Issue #5: Create CONTRIBUTING.md
**Priority**: P1 | **Area**: area/docs | **Kind**: kind/docs

**Description**:
Create contribution guidelines following k8s-gpu-mcp-server patterns.

**Acceptance Criteria**:
- [ ] Prerequisites section
- [ ] Development setup instructions
- [ ] Branch naming conventions
- [ ] Commit message format (conventional commits)
- [ ] PR process documentation

---

### Issue #6: Create CODE_OF_CONDUCT.md
**Priority**: P1 | **Area**: area/docs | **Kind**: kind/docs

**Description**:
Add Contributor Covenant Code of Conduct.

**Acceptance Criteria**:
- [ ] Contributor Covenant v2.1
- [ ] Contact email for enforcement

---

### Issue #7: Configure K8s client with out-of-cluster kubeconfig
**Priority**: P0 | **Area**: area/k8s | **Kind**: kind/feature

**Description**:
Implement Kubernetes client initialization using `~/.kube/config`.

**Acceptance Criteria**:
- [ ] Load kubeconfig from `~/.kube/config` or `KUBECONFIG` env
- [ ] Create `*kubernetes.Clientset`
- [ ] Create dynamic client for CRDs
- [ ] Support context selection via flag/env
- [ ] Proper error handling with wrapped errors
- [ ] Unit tests with mock client

**Technical Notes**:
```go
import (
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/dynamic"
)
```

---

### Issue #8: Set up basic MCP server skeleton (stdio transport)
**Priority**: P0 | **Area**: area/mcp | **Kind**: kind/feature

**Description**:
Create the basic MCP server using mcp-go library with stdio transport.

**Acceptance Criteria**:
- [ ] Initialize MCP server with name and version
- [ ] Configure stdio transport
- [ ] Register empty tool list (placeholder)
- [ ] Implement graceful shutdown with context
- [ ] Basic logging to stderr

**Technical Notes**:
```go
import "github.com/mark3labs/mcp-go/mcp"
```

---

### Issue #9: Add GitHub Issue Templates
**Priority**: P1 | **Area**: ops/ci-cd | **Kind**: kind/feature

**Description**:
Create GitHub issue templates following k8s-gpu-mcp-server patterns.

**Acceptance Criteria**:
- [ ] Bug report template (`bug_report.yml`)
- [ ] Feature request template (`feature_request.yml`)
- [ ] Technical debt template (`tech_debt.yml`)
- [ ] Security vulnerability template (`security.yml`)
- [ ] Config file to disable blank issues (`config.yml`)

---

### Issue #10: Add Pull Request Template
**Priority**: P1 | **Area**: ops/ci-cd | **Kind**: kind/feature

**Description**:
Create PR template with comprehensive checklist.

**Acceptance Criteria**:
- [ ] Description sections (what/why/how)
- [ ] Type of change checkboxes
- [ ] Testing checklist
- [ ] Code quality checklist
- [ ] DCO/GPG signing reminder

---

### Issue #11: Create CI workflow for lint, test, build
**Priority**: P1 | **Area**: ops/ci-cd | **Kind**: kind/feature

**Description**:
Create GitHub Actions CI workflow.

**Acceptance Criteria**:
- [ ] Trigger on push and PR
- [ ] Lint job with golangci-lint
- [ ] Test job with race detector
- [ ] Build job for linux/amd64 and linux/arm64
- [ ] Cache Go modules for speed

---

### Issue #12: Add dependabot.yml configuration
**Priority**: P2 | **Area**: ops/ci-cd | **Kind**: kind/feature

**Description**:
Configure Dependabot for automated dependency updates.

**Acceptance Criteria**:
- [ ] Weekly Go module updates
- [ ] Weekly GitHub Actions updates
- [ ] Group minor/patch updates
- [ ] Ignore major version bumps
- [ ] Assign to maintainer

---

## Phase 2: Core Tool Implementation

### Issue #13: Implement MPIJob manifest builder
**Priority**: P0 | **Area**: area/k8s | **Kind**: kind/feature

**Description**:
Implement Go struct-based MPIJob manifest generation.

**Acceptance Criteria**:
- [ ] Import kubeflow/mpi-operator types
- [ ] Create `BuildMPIJob(name, replicas, code string) *mpiv2beta1.MPIJob`
- [ ] Support launcher and worker specs
- [ ] Configure PVC mount at `/mnt/data`
- [ ] Add Kueue queue annotation
- [ ] Unit tests for manifest generation

**Constraints**:
- MUST use typed Go structs, not raw YAML strings
- MUST support Python and C++ source code injection

---

### Issue #14: Implement JobSet manifest builder
**Priority**: P0 | **Area**: area/k8s | **Kind**: kind/feature

**Description**:
Implement Go struct-based JobSet manifest generation.

**Acceptance Criteria**:
- [ ] Import sigs.k8s.io/jobset types
- [ ] Create `BuildJobSet(name string, replicas int, script string) *jobsetv1alpha2.JobSet`
- [ ] Inject `JOB_COMPLETION_INDEX` into output path
- [ ] Configure PVC mount at `/mnt/data`
- [ ] Add Kueue queue annotation
- [ ] Unit tests for manifest generation

**Critical Constraint**:
Output path MUST include `$(JOB_COMPLETION_INDEX)` to prevent data races:
```python
output_path = f"/mnt/data/results/{os.environ['JOB_COMPLETION_INDEX']}/result.json"
```

---

### Issue #15: Implement Batch Job manifest builder
**Priority**: P0 | **Area**: area/k8s | **Kind**: kind/feature

**Description**:
Implement standard Kubernetes Batch Job manifest generation.

**Acceptance Criteria**:
- [ ] Use `k8s.io/api/batch/v1` types
- [ ] Create `BuildReducerJob(name, script string) *batchv1.Job`
- [ ] Configure PVC mount at `/mnt/data` (ReadWriteMany)
- [ ] Set appropriate backoff limit and TTL
- [ ] Unit tests

---

### Issue #16: Implement submit_mpi_job MCP tool
**Priority**: P0 | **Area**: area/tools | **Kind**: kind/feature

**Description**:
Implement the `submit_mpi_job` MCP tool handler.

**Acceptance Criteria**:
- [ ] Define input schema: `{code: string, language: "python"|"cpp", nodes: int}`
- [ ] Generate unique job name
- [ ] Build MPIJob manifest
- [ ] Apply to cluster with dynamic client
- [ ] Return job ID immediately (non-blocking)
- [ ] Proper error handling
- [ ] Integration test

**Non-blocking requirement**:
Tool MUST return immediately after `kubectl apply` equivalent, not wait for completion.

---

### Issue #17: Implement submit_monte_carlo_batch MCP tool
**Priority**: P0 | **Area**: area/tools | **Kind**: kind/feature

**Description**:
Implement the `submit_monte_carlo_batch` MCP tool handler.

**Acceptance Criteria**:
- [ ] Define input schema: `{script: string, replicas: int}`
- [ ] Validate script contains proper output path handling
- [ ] Generate unique JobSet name
- [ ] Build JobSet manifest with completion index injection
- [ ] Apply to cluster
- [ ] Return job ID immediately
- [ ] Unit and integration tests

---

### Issue #18: Implement submit_reducer MCP tool
**Priority**: P0 | **Area**: area/tools | **Kind**: kind/feature

**Description**:
Implement the `submit_reducer` MCP tool handler.

**Acceptance Criteria**:
- [ ] Define input schema: `{script: string, input_pattern: string}`
- [ ] Generate unique job name
- [ ] Build Batch Job manifest
- [ ] Apply to cluster
- [ ] Return job ID immediately
- [ ] Unit tests

---

### Issue #19: Implement check_status MCP tool
**Priority**: P0 | **Area**: area/tools | **Kind**: kind/feature

**Description**:
Implement the `check_status` MCP tool handler.

**Acceptance Criteria**:
- [ ] Define input schema: `{job_id: string, job_type: "mpijob"|"jobset"|"job"}`
- [ ] Query appropriate API based on job_type
- [ ] Return structured status: `{phase, ready, succeeded, failed, message}`
- [ ] Handle job not found gracefully
- [ ] Unit tests with mock client

---

### Issue #20: Implement read_artifact_head MCP tool
**Priority**: P1 | **Area**: area/tools | **Kind**: kind/feature

**Description**:
Implement the `read_artifact_head` MCP tool to read result files from PVC.

**Acceptance Criteria**:
- [ ] Define input schema: `{path: string, lines: int}`
- [ ] Implement via helper pod or kubectl exec
- [ ] Read first N lines of specified file
- [ ] Handle file not found
- [ ] Security: validate path is within `/mnt/data`
- [ ] Cleanup helper pod after read

**Implementation Options**:
1. Create ephemeral pod with PVC mount, read file, delete pod
2. Use existing pod with PVC mount and kubectl exec

---

### Issue #21: Register all tools in MCP server
**Priority**: P0 | **Area**: area/mcp | **Kind**: kind/feature

**Description**:
Wire all implemented tools into the MCP server.

**Acceptance Criteria**:
- [ ] Register `submit_mpi_job`
- [ ] Register `submit_monte_carlo_batch`
- [ ] Register `submit_reducer`
- [ ] Register `check_status`
- [ ] Register `read_artifact_head`
- [ ] All tools have proper JSON schema definitions
- [ ] E2E test with example requests

---

## Phase 3: Deployment & Documentation

### Issue #22: Create multi-stage Dockerfile
**Priority**: P1 | **Area**: area/build | **Kind**: kind/feature

**Description**:
Create optimized multi-stage Dockerfile for the server.

**Acceptance Criteria**:
- [ ] Build stage with Go 1.25+
- [ ] Runtime stage with distroless or alpine
- [ ] Non-root user
- [ ] Proper labels (OCI annotations)
- [ ] Binary < 50MB
- [ ] Health check endpoint (if applicable)

---

### Issue #23: Create Helm chart
**Priority**: P1 | **Area**: area/helm | **Kind**: kind/feature

**Description**:
Create Helm chart for Kubernetes deployment (Helm only, no Kustomize).

**Acceptance Criteria**:
- [ ] Chart.yaml with proper metadata
- [ ] values.yaml with sensible defaults
- [ ] Deployment template
- [ ] ServiceAccount with minimal RBAC
- [ ] ConfigMap for configuration
- [ ] PVC reference configuration
- [ ] Support for Kueue queue annotation
- [ ] `helm lint` passes

---

### Issue #24: Add example CRD manifests
**Priority**: P2 | **Area**: area/docs | **Kind**: kind/docs

**Description**:
Create example manifests in `manifests/` directory.

**Acceptance Criteria**:
- [ ] Example MPIJob manifest
- [ ] Example JobSet manifest
- [ ] Example Batch Job manifest
- [ ] PVC manifest (ReadWriteMany)
- [ ] Comments explaining each field

---

### Issue #25: Add example MCP requests
**Priority**: P2 | **Area**: area/docs | **Kind**: kind/docs

**Description**:
Create example JSON-RPC requests in `examples/` directory.

**Acceptance Criteria**:
- [ ] `submit_mpi_job.json` example
- [ ] `submit_monte_carlo.json` example
- [ ] `submit_reducer.json` example
- [ ] `check_status.json` example
- [ ] `read_artifact_head.json` example
- [ ] Shell script to test examples

---

### Issue #26: Create comprehensive README.md
**Priority**: P1 | **Area**: area/docs | **Kind**: kind/docs

**Description**:
Create README following k8s-gpu-mcp-server style.

**Acceptance Criteria**:
- [ ] Project overview and purpose
- [ ] Quick start guide
- [ ] Available tools table
- [ ] Architecture diagram
- [ ] Prerequisites (Kueue, MPI-Operator, JobSet)
- [ ] Installation instructions (binary, container, Helm)
- [ ] Configuration options
- [ ] Contributing section
- [ ] License badge

---

### Issue #27: Create MCP prompt templates
**Priority**: P2 | **Area**: area/mcp | **Kind**: kind/feature

**Description**:
Create pre-built MCP prompts for common workflows.

**Acceptance Criteria**:
- [ ] `monte-carlo-simulation` prompt
- [ ] `mpi-matrix-multiply` prompt
- [ ] `aggregate-results` prompt
- [ ] Prompts documented in `pkg/prompts/`

---

### Issue #28: Add release workflow with GoReleaser
**Priority**: P2 | **Area**: ops/ci-cd | **Kind**: kind/feature

**Description**:
Create GitHub Actions workflow for releases.

**Acceptance Criteria**:
- [ ] Trigger on version tags (`v*`)
- [ ] Multi-platform builds
- [ ] Container image to ghcr.io
- [ ] GitHub release with artifacts
- [ ] Changelog generation

---

### Issue #29: Add security scanning workflow
**Priority**: P2 | **Area**: ops/security | **Kind**: kind/feature

**Description**:
Add security scanning to CI.

**Acceptance Criteria**:
- [ ] Trivy vulnerability scanning
- [ ] Upload results to GitHub Security
- [ ] Fail on high severity issues

---

## Summary

| Phase | Issues | P0 | P1 | P2 |
|-------|--------|----|----|-----|
| Phase 1: Foundation | 12 | 5 | 6 | 1 |
| Phase 2: Core Tools | 9 | 8 | 1 | 0 |
| Phase 3: Deployment | 8 | 0 | 4 | 4 |
| **Total** | **29** | **13** | **11** | **5** |

---

**Next Action**: Once `gh auth login` is complete, create the repository and all issues.
