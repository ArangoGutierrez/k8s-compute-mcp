# Stock Analysis with k8s-compute-mcp

Run a comprehensive 360° stock analysis on Kubernetes — driven entirely by your LLM.

Give your LLM access to a Kubernetes compute cluster via **k8s-compute-mcp**, then ask it to analyze any stock. The LLM designs the analysis, writes the computation scripts, submits them to the cluster, and interprets the results — all autonomously.

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

## Example: NVIDIA 360° Stock Analysis

Paste this prompt into your LLM:

> Perform a comprehensive 360° analysis of NVIDIA (NVDA) stock using the Kubernetes
> compute cluster. You have full autonomy to design and execute the analysis — write
> your own Python scripts, choose your models, and decide how many parallel replicas
> to run.
>
> Your analysis should cover:
>
> **Quantitative**
> - Monte Carlo price simulation (GBM or a model of your choice) with parallel replicas
> - Technical indicators: Fibonacci retracement/extension levels, moving average crossovers,
>   RSI, or any other indicators you find relevant
> - Risk metrics: Value at Risk (VaR), max drawdown, Sharpe ratio estimates
>
> **Geopolitical & Geoeconomic**
> - Factor in US-China semiconductor export controls and their impact on NVDA revenue
> - Consider the CHIPS Act subsidies and their effect on domestic production costs
> - Account for Taiwan Strait risk and its impact on TSMC supply chain (NVDA's primary fab partner)
> - Evaluate the AI regulation landscape across US, EU, and China
>
> **Macro & Sector**
> - Interest rate sensitivity: how does Fed policy affect growth stock valuations like NVDA?
> - AI capex cycle: analyze hyperscaler (MSFT, GOOGL, AMZN, META) spending trends on GPUs
> - Competitive landscape: AMD MI300, Intel Gaudi, custom silicon (Google TPU, Amazon Trainium)
> - Data center energy constraints and their impact on GPU deployment at scale
>
> **Workflow:**
> 1. Write Python scripts for each computation you want to run
> 2. Use `submit_monte_carlo_batch` for parallel simulations (100 replicas recommended)
> 3. Use `submit_reducer` for single-job computations (Fibonacci, risk metrics, etc.)
> 4. Use `check_status` to poll each job until completion
> 5. Use `submit_reducer` to aggregate all results into a final report
> 6. Use `read_artifact_head` to retrieve the report
> 7. Provide your interpretation: bull case, bear case, and base case with price targets
>
> Current NVDA price: ~$130. 52-week range: $100–$150.
> Be creative. Use as many parallel compute jobs as you need.

### What Happens

The LLM autonomously:

1. **Designs the analysis** — decides which models and metrics to compute
2. **Writes Python scripts** — creates its own simulation and analysis code on the fly
3. **Submits to Kubernetes** — sends scripts to the cluster via MCP tools for parallel execution
4. **Aggregates results** — combines outputs from all computations into a unified report
5. **Interprets findings** — synthesizes quantitative data with geopolitical/macro context into actionable insight

### Available MCP Tools

| Tool | Purpose |
|------|---------|
| `submit_monte_carlo_batch` | Run parallel simulations as a Kubernetes JobSet (e.g., 100 replicas each running 10K paths) |
| `submit_mpi_job` | Submit distributed MPI workloads (matrix operations, large-scale simulations) |
| `submit_reducer` | Submit a single job to process/aggregate data on the shared PVC |
| `check_status` | Poll job status until completion |
| `read_artifact_head` | Read results from the shared PVC back to the LLM |

All scripts run in Python 3.11 containers with access to the standard library. Results are stored on a shared PVC at `/mnt/data/`.

## Scaling with Your Cluster

The cluster is the compute engine — a bigger cluster means faster and more precise results:

| Cluster Size | Monte Carlo Replicas | Total Paths | Wall-Clock Time | Precision |
|-------------|---------------------|-------------|-----------------|-----------|
| 3 workers (t3.medium) | 10 | 100K | ~30s | Good |
| 10 workers | 100 | 1M | ~30s | High |
| 50 workers (GPU-enabled) | 500 | 5M+ | ~30s | Very high |

**Why it scales linearly:** Each Monte Carlo replica is an independent Kubernetes Job. Adding more replicas doesn't increase wall-clock time — they all run in parallel. More paths means lower variance in VaR, Sharpe, and drawdown estimates (precision improves with √N).

**GPU acceleration:** With GPU-enabled nodes, the LLM can write CUDA/NumPy scripts that run simulations orders of magnitude faster per replica. A single GPU replica can simulate millions of paths in seconds.

**MPI for large-scale jobs:** For workloads that benefit from inter-node communication (e.g., large matrix operations, correlated simulations across assets), use `submit_mpi_job` to distribute work across multiple nodes with MPI.

## Customize

Change the stock, add more dimensions, or go deeper on any axis:

- **Different stock**: Replace NVDA references with any ticker and adjust price/range
- **Different models**: Ask for jump-diffusion, Heston stochastic volatility, or GARCH
- **More replicas**: Increase parallel jobs for higher statistical precision
- **Different focus**: Emphasize ESG factors, supply chain risk, or earnings momentum
- **Multiple stocks**: Run comparative analysis across a sector (NVDA vs AMD vs INTC)

The LLM decides what to compute — you decide what to ask.

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
