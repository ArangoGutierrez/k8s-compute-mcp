# Copyright 2026 k8s-compute-mcp contributors
# SPDX-License-Identifier: Apache-2.0

# =============================================================================
# Version Configuration
# =============================================================================

# Library version (follows semver)
LIB_VERSION := 0.1.0

# Optional tag suffix (e.g., "rc1", "beta")
LIB_TAG :=

# Go version requirement
GOLANG_VERSION := 1.25

# Tool versions
GOLANGCI_LINT_VERSION := v2.8.0

# =============================================================================
# Git Information (auto-detected)
# =============================================================================

# Get git commit hash
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Get git tag if on a tagged commit
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null || echo "")

# Check if working directory is dirty
GIT_DIRTY := $(shell test -n "$$(git status --porcelain 2>/dev/null)" && echo "-dirty")

# Full version string
VERSION := $(LIB_VERSION)$(if $(LIB_TAG),-$(LIB_TAG))$(GIT_DIRTY)
