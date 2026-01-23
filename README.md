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
```

### Project Structure

```
k8s-compute-mcp/
├── cmd/server/         # Main entry point
├── internal/
│   ├── k8s/            # Kubernetes client & manifest generation
│   └── mcp/            # MCP protocol handling
├── pkg/prompts/        # MCP prompt templates
├── deployment/helm/    # Helm chart
├── manifests/          # Example CRDs
├── examples/           # Example MCP requests
└── docs/               # Documentation
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

## Project Status

**Current Phase:** Phase 1 - Foundation

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1 | In Progress | Repository setup, K8s client, MCP skeleton |
| Phase 2 | Planned | Core tool implementations |
| Phase 3 | Planned | Deployment, Helm chart, CI/CD |

See [Issues](https://github.com/ArangoGutierrez/k8s-compute-mcp/issues) for detailed roadmap.

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
