#!/usr/bin/env bash
# setup-cluster.sh — Install required operators and storage for k8s-compute-mcp.
#
# Prerequisites: kubectl and helm configured for your cluster.
# This script is idempotent — safe to re-run.
#
# Installs:
#   - Kueue (job queuing)
#   - MPI Operator (distributed MPI workloads)
#   - JobSet controller (parallel batch jobs)
#   - Shared PVC for computation results
#
# Usage:
#   ./demos/stock-analysis/setup-cluster.sh
#   ./demos/stock-analysis/setup-cluster.sh --skip-mpi   # skip MPI Operator

set -euo pipefail

KUEUE_NAMESPACE="kueue-system"
MPI_OPERATOR_VERSION="v0.7.0"
JOBSET_VERSION="v0.6.0"
PVC_NAME="${PVC_NAME:-compute-data}"
PVC_SIZE="${PVC_SIZE:-10Gi}"
SKIP_MPI="${1:-}"

if [ -n "$SKIP_MPI" ] && [ "$SKIP_MPI" != "--skip-mpi" ]; then
  echo "ERROR: Unknown argument '$SKIP_MPI'"
  echo "Usage: $0 [--skip-mpi]"
  exit 1
fi

echo "=== k8s-compute-mcp Cluster Setup ==="
echo ""

# --- Verify prerequisites ---
for cmd in kubectl helm; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: $cmd is required but not found in PATH"
    exit 1
  fi
done

echo "Cluster: $(kubectl config current-context)"
echo ""

# --- Install Kueue ---
if helm list -n "$KUEUE_NAMESPACE" 2>/dev/null | grep -q kueue; then
  echo "[OK] Kueue already installed"
else
  echo "Installing Kueue..."
  helm install kueue \
    oci://registry.k8s.io/kueue/charts/kueue \
    --namespace "$KUEUE_NAMESPACE" \
    --create-namespace \
    --wait \
    --timeout 5m
  echo "[OK] Kueue installed"
fi

# Create Kueue queues
echo "Configuring Kueue queues..."
cat <<'KUEUE_EOF' | kubectl apply -f -
apiVersion: kueue.x-k8s.io/v1beta1
kind: ResourceFlavor
metadata:
  name: default-flavor
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: ClusterQueue
metadata:
  name: cluster-queue
spec:
  namespaceSelector: {}
  resourceGroups:
  - coveredResources: ["cpu", "memory"]
    flavors:
    - name: default-flavor
      resources:
      - name: "cpu"
        nominalQuota: 100
      - name: "memory"
        nominalQuota: 200Gi
---
apiVersion: kueue.x-k8s.io/v1beta1
kind: LocalQueue
metadata:
  name: default-queue
  namespace: default
spec:
  clusterQueue: cluster-queue
KUEUE_EOF
echo "[OK] Kueue queues configured"

# --- Install MPI Operator (optional) ---
if [ "$SKIP_MPI" = "--skip-mpi" ]; then
  echo "[SKIP] MPI Operator (--skip-mpi flag)"
else
  if kubectl get deployment mpi-operator -n mpi-operator &>/dev/null; then
    echo "[OK] MPI Operator already installed"
  else
    echo "Installing MPI Operator ${MPI_OPERATOR_VERSION}..."
    kubectl apply --server-side -f \
      "https://raw.githubusercontent.com/kubeflow/mpi-operator/${MPI_OPERATOR_VERSION}/deploy/v2beta1/mpi-operator.yaml"
    echo "Waiting for MPI Operator..."
    kubectl rollout status deployment/mpi-operator -n mpi-operator --timeout=3m
    echo "[OK] MPI Operator installed"
  fi
fi

# --- Install JobSet ---
if kubectl get deployment jobset-controller-manager -n jobset-system &>/dev/null; then
  echo "[OK] JobSet controller already installed"
else
  echo "Installing JobSet controller ${JOBSET_VERSION}..."
  kubectl apply --server-side -f \
    "https://github.com/kubernetes-sigs/jobset/releases/download/${JOBSET_VERSION}/manifests.yaml"
  echo "Waiting for JobSet controller..."
  kubectl rollout status deployment/jobset-controller-manager -n jobset-system --timeout=3m
  echo "[OK] JobSet controller installed"
fi

# --- Create shared PVC ---
if kubectl get pvc "$PVC_NAME" -n default &>/dev/null; then
  echo "[OK] PVC '${PVC_NAME}' already exists"
else
  echo "Creating shared PVC '${PVC_NAME}' (${PVC_SIZE})..."
  cat <<PVC_EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ${PVC_NAME}
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: ${PVC_SIZE}
PVC_EOF
  echo "Waiting for PVC to bind..."
  kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc/"$PVC_NAME" -n default --timeout=60s || \
    echo "WARNING: PVC not yet bound (may need a StorageClass that supports RWX)"
  echo "[OK] PVC created"
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. Install k8s-compute-mcp:  go install github.com/ArangoGutierrez/k8s-compute-mcp/cmd/server@latest"
echo "  2. Or build from source:     make build"
echo "  3. Configure your LLM client (see README.md)"
echo ""
