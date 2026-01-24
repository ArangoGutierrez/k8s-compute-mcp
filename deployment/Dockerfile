# Copyright 2026 k8s-compute-mcp contributors
# SPDX-License-Identifier: Apache-2.0

# =============================================================================
# Build Stage
# =============================================================================
FROM golang:1.25-alpine AS builder

# Install git for version info (optional, graceful degradation if missing)
# RUN apk add --no-cache git

WORKDIR /app

# Download dependencies first (layer caching optimization)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version injection
ARG VERSION=dev
ARG GIT_COMMIT=unknown

# Build static binary
# Note: Uses PascalCase variable names to match internal/info/info.go
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false \
    -ldflags="-s -w \
        -X github.com/ArangoGutierrez/k8s-compute-mcp/internal/info.Version=${VERSION} \
        -X github.com/ArangoGutierrez/k8s-compute-mcp/internal/info.GitCommit=${GIT_COMMIT}" \
    -o /server \
    ./cmd/server

# =============================================================================
# Runtime Stage
# =============================================================================
FROM gcr.io/distroless/static:nonroot

# OCI Image Labels
LABEL org.opencontainers.image.source="https://github.com/ArangoGutierrez/k8s-compute-mcp"
LABEL org.opencontainers.image.description="Kubernetes Computational Mathematics MCP Server"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.title="k8s-compute-mcp"
LABEL org.opencontainers.image.vendor="ArangoGutierrez"

# Copy binary from builder
COPY --from=builder /server /server

# Kubeconfig mount point
# Note: In distroless, the nonroot user has $HOME=/home/nonroot
# Mount kubeconfig to /home/nonroot/.kube/config at runtime
VOLUME ["/home/nonroot/.kube"]

# Run as non-root user (distroless nonroot UID=65532)
USER nonroot:nonroot

# MCP server uses stdio transport, no ports exposed
ENTRYPOINT ["/server"]
