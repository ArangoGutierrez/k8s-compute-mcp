# k8s-compute-mcp

**Computational Mathematics MCP Server for Kubernetes**

Go Version License

---

## Overview

`k8s-compute-mcp` is a local MCP (Model Context Protocol) Server written in Go that acts as a bridge between LLM clients (Gemini CLI, Claude Desktop, Cursor) and a Kubernetes cluster. It enables LLMs to offload heavy mathematical and financial computations to the cluster using specialized operators.

### Key Features

- **MPI Job Submission** - Submit distributed MPI workloads (Python/C++) via Kubeflow MPI Operator
- **Monte Carlo Simulations** - Run parallel Monte Carlo batches using JobSet with automatic output isolation
- **Result Aggregation** - Submit reducer jobs to aggregate computation results
- **Status Monitoring** - Query job/workload status in real-time
- **Artifact Access** - Read computation results directly from shared storage
- **Observability** - Prometheus metrics, structured logging, health/readiness probes
- **Reliability** - Retry with exponential backoff, input validation, code size limits
- **E2E Testing** - Ginkgo/Gomega framework with KIND cluster lifecycle
- **Security** - gosec scanning, Trivy container analysis, SBOM generation
- **Releases** - GoReleaser with multi-platform binaries and container images

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    MCP Client (Gemini/Claude/Cursor)                 │
└────────────────────────────┬────────────────────────────────────────┘
                             │ stdio / SSE
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    k8s-compute-mcp Server                            │
│       MCP Protocol → Tool Handlers → K8s Client                      │
└────────────────────────────┬────────────────────────────────────────┘
                             │ client-go
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
│  │   Kueue     │  │ MPI Operator│  │   JobSet    │  │  Shared PVC │ │
│  │  (Queuing)  │  │ (Kubeflow)  │  │ Controller  │  │  /mnt/data  │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

### Target Cluster Requirements

- Kubernetes 1.28+
- [Kueue](https://kueue.sigs.k8s.io/) - Workload queuing
- [MPI Operator](https://github.com/kubeflow/mpi-operator) (v2beta1) - Distributed MPI workloads
- [JobSet Controller](https://github.com/kubernetes-sigs/jobset) (v1alpha2) - Parallel job sets
- Shared PVC with `ReadWriteMany` access mode mounted at `/mnt/data`

---

## Available Tools

| Tool | Description | K8s Resource |
|------|-------------|--------------|
| `submit_mpi_job` | Submit MPI workloads (Python/C++) | `kubeflow.org/v2beta1 MPIJob` |
| `submit_monte_carlo_batch` | Submit parallel Monte Carlo simulations | `jobset.x-k8s.io/v1alpha2 JobSet` |
| `submit_reducer` | Submit result aggregation jobs | `batch/v1 Job` |
| `check_status` | Query job/workload status | Various |
| `read_artifact_head` | Read first N lines of result files | PVC access |

### Tool Examples

**Submit MPI Job:**
```json
{
  "tool": "submit_mpi_job",
  "arguments": {
    "code": "from mpi4py import MPI\n...",
    "language": "python",
    "nodes": 4
  }
}
```

**Submit Monte Carlo Batch:**
```json
{
  "tool": "submit_monte_carlo_batch",
  "arguments": {
    "script": "import random\n...",
    "replicas": 100
  }
}
```

---

## Quick Start

### Prerequisites

1. Go 1.25+
2. Access to a Kubernetes cluster with required operators
3. `kubectl` configured with cluster access

### Build from Source

```bash
git clone https://github.com/ArangoGutierrez/k8s-compute-mcp.git
cd k8s-compute-mcp
make build
```

### Run

```bash
# Stdio mode (for MCP clients)
./bin/server

# Test with example
cat examples/submit_mpi.json | ./bin/server
```

### Configure Claude Desktop

Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "k8s-compute": {
      "command": "/path/to/k8s-compute-mcp/bin/server"
    }
  }
}
```

---

## Installation

### Using Go

```bash
go install github.com/ArangoGutierrez/k8s-compute-mcp/cmd/server@latest
```

### Container Image

```bash
docker pull ghcr.io/arangogutierrez/k8s-compute-mcp:latest
```

### Helm Chart

```bash
helm install k8s-compute-mcp \
  oci://ghcr.io/arangogutierrez/charts/k8s-compute-mcp \
  --namespace compute-mcp --create-namespace
```

---

## Configuration

The server uses your local kubeconfig (`~/.kube/config`) for cluster access.

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `KUBECONFIG` | Path to kubeconfig file | `~/.kube/config` |
| `KUBE_CONTEXT` | Kubernetes context to use | Current context |
| `MCP_LOG_LEVEL` | Log verbosity (debug, info, warn, error) | `info` |
| `PVC_MOUNT_PATH` | Shared storage mount path | `/mnt/data` |
| `KUEUE_QUEUE` | Default Kueue queue name | `default-queue` |

---

## Development

### Build

```bash
make build      # Build binary
make test       # Run tests
make lint       # Run linter
make image      # Build container image
make test-e2e   # Run E2E tests (requires kind, helm, kubectl)
```

### Project Structure

```
k8s-compute-mcp/
├── cmd/server/              # Main entry point
├── internal/
│   ├── k8s/                 # Kubernetes client, manifests, retry, status
│   │   ├── client.go        # client-go setup (out-of-cluster)
│   │   ├── mpijob.go        # MPIJob manifest builder
│   │   ├── jobset.go        # JobSet manifest builder
│   │   ├── job.go           # Batch Job manifest builder
│   │   ├── reader.go        # Ephemeral pod artifact reader
│   │   ├── retry.go         # Exponential backoff with jitter
│   │   └── status.go        # Job phase extraction
│   ├── mcp/                 # MCP protocol handling
│   │   ├── server.go        # Server setup & tool registration
│   │   ├── handlers.go      # Tool handlers
│   │   ├── tools.go         # Tool definitions & schemas
│   │   ├── health.go        # /healthz, /readyz, /metrics endpoints
│   │   └── metrics.go       # Prometheus instrumentation
│   └── info/                # Build version info
├── pkg/prompts/             # MCP prompt templates
├── deployment/
│   ├── Dockerfile           # Multi-stage production image
│   ├── Dockerfile.goreleaser # GoReleaser-optimized image
│   └── helm/                # Helm chart
├── test/e2e/                # E2E tests (Ginkgo + KIND)
└── .github/workflows/       # CI, E2E, release, security
```

---

## Use Cases

### 1. Monte Carlo Option Pricing

```
User: "Price a European call option using Monte Carlo with 10M simulations"

LLM → k8s-compute-mcp → submit_monte_carlo_batch(replicas=100)
     → Each replica runs 100K simulations
     → submit_reducer aggregates results
     → read_artifact_head returns final price
```

### 2. Distributed Matrix Operations

```
User: "Multiply these two large matrices using MPI"

LLM → k8s-compute-mcp → submit_mpi_job(nodes=8, code="...")
     → MPI job distributes matrix blocks
     → check_status monitors progress
     → read_artifact_head returns result
```

### 3. Parameter Sweep

```
User: "Run sensitivity analysis across 500 parameter combinations"

LLM → k8s-compute-mcp → submit_monte_carlo_batch(replicas=500)
     → JOB_COMPLETION_INDEX isolates each run
     → Results collected in /mnt/data/results/{index}/
```

---

## Observability

The server exposes an HTTP endpoint (default `:8081`) for health checks and metrics:

| Endpoint | Description |
|----------|-------------|
| `/healthz` | Liveness probe (always returns 200) |
| `/readyz` | Readiness probe (checks K8s connectivity) |
| `/metrics` | Prometheus metrics |

### Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `mcp_tool_requests_total` | Counter | Total tool invocations by tool name and status |
| `mcp_tool_errors_total` | Counter | Tool errors by tool name and error type |
| `mcp_tool_duration_seconds` | Histogram | Tool call latency distribution |
| `mcp_active_jobs` | Gauge | Currently tracked active jobs |

### Reliability

- **Retry with backoff**: Transient K8s API errors (429, 5xx, timeouts) are retried with exponential backoff and jitter
- **Input validation**: Code size limits, argument validation at tool boundaries
- **Structured logging**: klog/v2 with key-value pairs to stderr (stdout reserved for MCP protocol)

---

## Project Status

**Current Phase:** All core phases complete

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1 | **Complete** | Repository setup, K8s client, MCP skeleton |
| Phase 2 | **Complete** | All 5 core tools implemented and tested |
| Phase 3 | **Complete** | Dockerfile, Helm chart, CI/CD, security, observability |

### What's Included

- **5 MCP tools**: submit_mpi_job, submit_monte_carlo_batch, submit_reducer, check_status, read_artifact_head
- **Production deployment**: Multi-stage Dockerfile, Helm chart with RBAC/probes/metrics
- **CI/CD**: Lint/test/vet on every PR, GoReleaser releases, security scanning (gosec + Trivy)
- **E2E testing**: Ginkgo framework with KIND cluster lifecycle (nightly)
- **Observability**: Prometheus metrics, structured logging, health/readiness endpoints
- **Reliability**: Retry with exponential backoff, input validation, code size caps

See [Issues](https://github.com/ArangoGutierrez/k8s-compute-mcp/issues) for future enhancements.

---

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Quick Contribution Guide

1. Fork and clone the repository
2. Create feature branch: `git checkout -b feat/my-feature`
3. Make changes, add tests
4. Run checks: `make lint && make test`
5. Commit with DCO: `git commit -s -S -m "feat(scope): description"`
6. Open PR with labels and milestone

---

## Related Projects

- [k8s-gpu-mcp-server](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server) - GPU diagnostics MCP server
- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP Go implementation
- [mpi-operator](https://github.com/kubeflow/mpi-operator) - Kubeflow MPI Operator
- [jobset](https://github.com/kubernetes-sigs/jobset) - Kubernetes JobSet controller
- [kueue](https://github.com/kubernetes-sigs/kueue) - Kubernetes-native job queuing

---

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

## Contact

**Maintainer:** [@ArangoGutierrez](https://github.com/ArangoGutierrez)  
**Issues:** [GitHub Issues](https://github.com/ArangoGutierrez/k8s-compute-mcp/issues)

---

**Star us on GitHub — it helps!**
