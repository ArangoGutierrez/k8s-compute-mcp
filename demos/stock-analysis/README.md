# Stock Analysis with k8s-compute-mcp

Run quantitative stock analysis on Kubernetes — driven entirely by your LLM.

This demo walks you through deploying **k8s-compute-mcp** to any Kubernetes cluster and using it to run Monte Carlo simulations, Fibonacci technical analysis, and aggregated reports for NVIDIA (NVDA) stock. Works with Claude Desktop, Claude Code, Cursor, Gemini CLI, or any MCP-compatible client.

## Prerequisites

- **Kubernetes cluster** — any provider (AWS, GKE, EKS, on-prem, or [Holodeck](#provision-a-cluster-with-holodeck))
- **kubectl** configured with cluster access
- **helm** v3+
- **Go** 1.23+ (to build the MCP server)
- An **MCP-compatible LLM client** (Claude Desktop, Claude Code, Cursor, Gemini CLI)

## Quick Start

```bash
# 1. Install operators and storage
./demos/stock-analysis/setup-cluster.sh

# 2. Install k8s-compute-mcp via Helm
helm install k8s-compute-mcp ./deployment/helm \
  --set pvcName=compute-data \
  --set kueueQueue=default-queue

# 3. Verify
kubectl get pods -l app=k8s-compute-mcp
```

## Deploy to Your Cluster

### Step 1: Install Operators

The setup script installs Kueue, MPI Operator, and JobSet controller:

```bash
./demos/stock-analysis/setup-cluster.sh
```

Pass `--skip-mpi` if you don't need MPI workloads:

```bash
./demos/stock-analysis/setup-cluster.sh --skip-mpi
```

### Step 2: Install k8s-compute-mcp

**Option A: Helm chart**

```bash
helm install k8s-compute-mcp ./deployment/helm \
  --set pvcName=compute-data \
  --set kueueQueue=default-queue
```

**Option B: Build from source**

```bash
make build
./bin/k8s-compute-mcp
```

### Step 3: Verify

```bash
# Check the MCP server pod is running
kubectl get pods -l app=k8s-compute-mcp

# Test the health endpoint (port-forward if needed)
kubectl port-forward svc/k8s-compute-mcp 8081:8081 &
curl http://localhost:8081/healthz
```

## Connect Your LLM

### Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "k8s-compute": {
      "command": "k8s-compute-mcp",
      "args": [],
      "env": {
        "KUBECONFIG": "/path/to/kubeconfig",
        "PVC_NAME": "compute-data"
      }
    }
  }
}
```

### Claude Code

```bash
claude mcp add k8s-compute -- k8s-compute-mcp
```

### Cursor

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "k8s-compute": {
      "command": "k8s-compute-mcp",
      "env": { "KUBECONFIG": "/path/to/kubeconfig" }
    }
  }
}
```

### Gemini CLI / Generic stdio

```bash
k8s-compute-mcp  # speaks JSON-RPC 2.0 over stdin/stdout
```

## Example: NVIDIA Stock Analysis

Paste this prompt into your LLM:

> Analyze NVIDIA (NVDA) stock using the Kubernetes compute cluster.
>
> 1. Run a Monte Carlo simulation of NVDA stock price using Geometric Brownian Motion.
>    Current price: $130, annual drift: 15%, volatility: 45%, 252 trading days, 100 parallel replicas.
>    Use `submit_monte_carlo_batch` with the simulation script at `demos/stock-analysis/scripts/monte_carlo_gbm.py`.
>
> 2. Calculate Fibonacci retracement levels between the 52-week high ($150) and low ($100).
>    Submit as a single batch job using `submit_reducer` with the script at `demos/stock-analysis/scripts/fibonacci_levels.py`.
>
> 3. Aggregate all results into a final analysis report using `submit_reducer` with `demos/stock-analysis/scripts/aggregate_report.py`.
>
> 4. Read the final report with `read_artifact_head`.
>
> After each submission, poll `check_status` until the job completes before proceeding.

### What the LLM Does

The LLM reads the Python scripts and submits them to your cluster via MCP tool calls:

**Step 1 — Monte Carlo simulation** (`submit_monte_carlo_batch`):
- Submits 100 parallel replicas as a Kubernetes JobSet
- Each replica runs 10,000 GBM simulation paths
- Results written to `/mnt/data/monte-carlo-{0..99}/result.json` on the shared PVC

**Step 2 — Fibonacci levels** (`submit_reducer`):
- Submits a single Job computing retracement and extension levels
- Results written to `/mnt/data/fibonacci/levels.json`

**Step 3 — Aggregate** (`submit_reducer`):
- Reads all Monte Carlo results + Fibonacci levels
- Produces combined report at `/mnt/data/analysis-report.json`

**Step 4 — Read results** (`read_artifact_head`):
- Returns the JSON report directly to the LLM for interpretation

### Understanding the Results

The final report includes:

| Section | Key Metrics |
|---------|------------|
| **Monte Carlo** | Mean/median price, 5th-95th percentile range, probability of reaching $200, average max drawdown |
| **Fibonacci** | Retracement levels (23.6%, 38.2%, 50%, 61.8%, 78.6%), extension targets (161.8%, 261.8%) |
| **Combined** | Which Fibonacci levels fall within the Monte Carlo price range — potential support/resistance zones |

<details>
<summary>Example output (click to expand)</summary>

```json
{
  "stock": "NVDA",
  "analysis_type": "quantitative",
  "monte_carlo": {
    "replicas": 100,
    "total_paths": 1000000,
    "mean_final_price": 151.23,
    "confidence_interval_95": [149.85, 152.61],
    "percentile_5_avg": 64.18,
    "percentile_95_avg": 287.42,
    "prob_above_200": 0.1952,
    "avg_max_drawdown": 0.3871
  },
  "fibonacci": {
    "retracement_levels": {
      "23.6%": 138.20,
      "38.2%": 130.90,
      "50.0%": 125.00,
      "61.8%": 119.10
    },
    "key_support": [119.10, 125.00, 130.90],
    "key_resistance": [180.90, 230.90]
  },
  "combined_analysis": {
    "fibonacci_levels_in_mc_range": {
      "23.6%": 138.20,
      "38.2%": 130.90,
      "50.0%": 125.00,
      "61.8%": 119.10,
      "161.8%": 180.90,
      "261.8%": 230.90
    },
    "interpretation": "Monte Carlo simulations suggest NVDA price between $64.18 and $287.42 (5th-95th percentile) over 1 year. 6 Fibonacci levels fall within this range, suggesting potential support/resistance zones."
  }
}
```

</details>

## Customize

Modify the Python scripts for different stocks:

| Parameter | File | Default | Description |
|-----------|------|---------|-------------|
| `S0` | `monte_carlo_gbm.py` | 130.0 | Current stock price |
| `MU` | `monte_carlo_gbm.py` | 0.15 | Annual drift (expected return) |
| `SIGMA` | `monte_carlo_gbm.py` | 0.45 | Annual volatility |
| `HIGH` | `fibonacci_levels.py` | 150.0 | 52-week high |
| `LOW` | `fibonacci_levels.py` | 100.0 | 52-week low |

For more complex models (jump diffusion, Heston stochastic volatility), replace the simulation logic in `monte_carlo_gbm.py` — the MCP pipeline stays the same.

## Provision a Cluster with Holodeck

If you don't have an existing Kubernetes cluster, use [NVIDIA Holodeck](https://github.com/NVIDIA/holodeck) to provision one on AWS:

```bash
# Set your AWS credentials
export AWS_KEY_NAME=your-key-pair
export AWS_PRIVATE_KEY_PATH=~/.ssh/your-key.pem

# Provision a CPU-only cluster (1 control plane + 3 workers)
holodeck create -f demos/stock-analysis/holodeck-env.yaml --provision -k ./kubeconfig.yaml

# Point kubectl at the new cluster
export KUBECONFIG=./kubeconfig.yaml

# Install operators
./demos/stock-analysis/setup-cluster.sh
```

## Teardown

```bash
# Delete compute jobs
kubectl delete jobsets,jobs -l app=k8s-compute-mcp -n default

# Remove the MCP server
helm uninstall k8s-compute-mcp

# Delete operators (optional)
kubectl delete -f "https://github.com/kubernetes-sigs/jobset/releases/download/v0.6.0/manifests.yaml"
kubectl delete -f "https://raw.githubusercontent.com/kubeflow/mpi-operator/v0.7.0/deploy/v2beta1/mpi-operator.yaml"
helm uninstall kueue -n kueue-system

# Delete PVC (destroys all computation results)
kubectl delete pvc compute-data

# Destroy Holodeck cluster (if applicable)
holodeck list
holodeck delete <instance-id>
```
