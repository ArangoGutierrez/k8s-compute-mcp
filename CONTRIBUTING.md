# Contributing to k8s-compute-mcp

Thank you for your interest in contributing to k8s-compute-mcp! This document
provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Documentation](#documentation)

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).
By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- Go 1.25 or later
- Access to a Kubernetes cluster with:
  - [Kueue](https://kueue.sigs.k8s.io/) installed
  - [MPI Operator](https://github.com/kubeflow/mpi-operator) installed
  - [JobSet Controller](https://github.com/kubernetes-sigs/jobset) installed
- Docker or Podman (for container builds)
- Helm 3.x (for chart development)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/k8s-compute-mcp.git
cd k8s-compute-mcp
```

3. Add the upstream remote:

```bash
git remote add upstream https://github.com/ArangoGutierrez/k8s-compute-mcp.git
```

## Development Setup

### Build the Project

```bash
# Build the binary
make build

# Run tests
make test

# Run linter
make lint

# Build container image
make image
```

### Project Structure

```
k8s-compute-mcp/
├── cmd/server/         # Main application entry point
├── internal/
│   ├── k8s/            # Kubernetes client and manifest generation
│   └── mcp/            # MCP protocol handling
├── pkg/
│   └── prompts/        # MCP prompt templates
├── deployment/
│   └── helm/           # Helm chart
├── manifests/          # Example CRD manifests
├── examples/           # Example MCP requests
├── docs/               # Documentation
└── test/e2e/           # End-to-end tests
```

## Making Changes

### Branch Naming

Use descriptive branch names:

- `feat/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation changes
- `refactor/description` - Code refactoring
- `chore/description` - Maintenance tasks

### Commit Messages

Follow conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `test`: Adding or updating tests
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `chore`: Changes to build process or auxiliary tools

Examples:

```
feat(tools): add submit_mpi_job tool

Implements MPI job submission with support for Python and C++
source code injection. Uses typed Go structs for manifest generation.

Closes #16
```

```
fix(k8s): handle context cancellation in job submission

Previously ignored context cancellation during API calls.
Now properly propagates cancellation to avoid orphaned resources.

Fixes #42
```

## Pull Request Process

### Before Submitting

1. **Update your branch** with the latest upstream changes:

```bash
git fetch upstream
git rebase upstream/main
```

2. **Run all checks**:

```bash
make lint
make test
go vet ./...
```

3. **Ensure tests pass** with the race detector:

```bash
go test -race ./...
```

### PR Requirements

- [ ] Tests pass locally
- [ ] New code has appropriate test coverage
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventions
- [ ] No unrelated changes included
- [ ] Commits are signed with DCO (`-s`) and GPG (`-S`)

### Review Process

1. Submit your PR with a clear description
2. Address reviewer feedback
3. Maintainer will merge once approved

## Coding Standards

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Run `golangci-lint` before committing
- Keep functions focused and small
- Use meaningful variable names
- Documentation comments limited to 80 characters per line

### Error Handling

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to submit MPI job %s: %w", jobName, err)
}

// Bad: Bare error returns
if err != nil {
    return err
}
```

### Context Propagation

```go
// All K8s operations must accept and respect context
func (c *Client) SubmitMPIJob(ctx context.Context, job *MPIJob) (string, error) {
    // Check context before long operations
    select {
    case <-ctx.Done():
        return "", ctx.Err()
    default:
    }
    // ... implementation
}
```

### Comments

- Export comments for all public types and functions
- Keep comments up to date with code changes
- Use complete sentences

```go
// MPIJob represents a Kubeflow MPI job submission request.
// It contains the source code, language, and resource requirements
// for distributed MPI workloads.
type MPIJob struct {
    // ...
}
```

### MCP Tool Implementation

When adding new MCP tools:

1. Define input struct in `internal/mcp/tools.go`
2. Implement handler in `internal/mcp/handlers.go`
3. Register tool in `internal/mcp/server.go`
4. Add manifest builder in `internal/k8s/` (if needed)
5. Write unit tests
6. Add example request in `examples/`

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run specific package tests
go test ./internal/k8s/...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Integration Tests

```bash
# Requires a Kubernetes cluster with operators installed
go test -tags=integration ./...
```

### E2E Tests

```bash
# Full workflow tests
go test -tags=e2e ./test/e2e/...
```

## Documentation

### When to Update Docs

- Adding new features or tools
- Changing configuration options
- Modifying deployment procedures
- Fixing incorrect documentation

### Documentation Locations

- `README.md` - Project overview and quick start
- `AGENTS.md` - AI agent context (for AI-assisted development)
- `docs/` - Detailed documentation
- `docs/prompts/` - Development task prompts
- Code comments - API documentation

## Getting Help

- Open an issue for bugs or feature requests
- Check existing issues before creating new ones
- Join discussions in GitHub Discussions

## Recognition

Contributors are recognized in release notes. Thank you for helping improve
k8s-compute-mcp!
