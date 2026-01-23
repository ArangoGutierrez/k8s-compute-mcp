// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

// Package prompts provides MCP prompt templates for common computational workflows.
//
// Prompts are pre-built templates that guide LLMs through complex multi-tool
// workflows such as Monte Carlo simulations, MPI computations, and result
// aggregation.
package prompts

// MonteCarloSimulation is a prompt template for running parallel Monte Carlo simulations.
const MonteCarloSimulation = `Run a Monte Carlo simulation with the following parameters:
- Simulation type: {{.Type}}
- Number of iterations per replica: {{.IterationsPerReplica}}
- Parallel replicas: {{.Replicas}}

The workflow should:
1. Submit parallel jobs using submit_monte_carlo_batch
2. Monitor progress with check_status until all replicas complete
3. Aggregate results with submit_reducer
4. Return final results with read_artifact_head

Important: Each replica writes to /mnt/data/results/$JOB_COMPLETION_INDEX/ to prevent conflicts.
`

// MPIMatrixMultiply is a prompt template for distributed matrix operations.
const MPIMatrixMultiply = `Perform distributed matrix multiplication using MPI:
- Matrix A dimensions: {{.RowsA}}x{{.ColsA}}
- Matrix B dimensions: {{.RowsB}}x{{.ColsB}}
- Number of MPI nodes: {{.Nodes}}

The workflow should:
1. Submit MPI job with matrix multiplication code using submit_mpi_job
2. Monitor completion with check_status
3. Read result matrix with read_artifact_head

The MPI code should:
- Distribute matrix blocks across workers
- Use collective operations for efficient communication
- Write final result to /mnt/data/results/matrix_result.npy
`

// AggregateResults is a prompt template for combining results from parallel jobs.
const AggregateResults = `Aggregate results from a completed parallel computation:
- Input pattern: {{.InputPattern}}
- Aggregation method: {{.Method}} (sum, mean, concatenate, custom)
- Output path: {{.OutputPath}}

The workflow should:
1. Submit reducer job using submit_reducer
2. Monitor completion with check_status
3. Read aggregated results with read_artifact_head

The reducer script should:
- Glob all files matching the input pattern
- Apply the specified aggregation method
- Write combined results to the output path
`

// PromptTemplate represents a registered prompt with its arguments.
type PromptTemplate struct {
	// Name is the prompt identifier.
	Name string

	// Description explains what the prompt does.
	Description string

	// Template is the prompt template string.
	Template string

	// Arguments defines the required and optional arguments.
	Arguments []PromptArgument
}

// PromptArgument describes a prompt template argument.
type PromptArgument struct {
	// Name is the argument name (used in template as {{.Name}}).
	Name string

	// Description explains the argument.
	Description string

	// Required indicates if the argument must be provided.
	Required bool
}

// GetPrompts returns all registered prompt templates.
func GetPrompts() []PromptTemplate {
	return []PromptTemplate{
		{
			Name:        "monte-carlo-simulation",
			Description: "Run parallel Monte Carlo simulation with result aggregation",
			Template:    MonteCarloSimulation,
			Arguments: []PromptArgument{
				{Name: "Type", Description: "Type of simulation (e.g., option-pricing, pi-estimation)", Required: true},
				{Name: "IterationsPerReplica", Description: "Number of iterations per replica", Required: true},
				{Name: "Replicas", Description: "Number of parallel replicas", Required: true},
			},
		},
		{
			Name:        "mpi-matrix-multiply",
			Description: "Perform distributed matrix multiplication using MPI",
			Template:    MPIMatrixMultiply,
			Arguments: []PromptArgument{
				{Name: "RowsA", Description: "Number of rows in matrix A", Required: true},
				{Name: "ColsA", Description: "Number of columns in matrix A", Required: true},
				{Name: "RowsB", Description: "Number of rows in matrix B", Required: true},
				{Name: "ColsB", Description: "Number of columns in matrix B", Required: true},
				{Name: "Nodes", Description: "Number of MPI nodes", Required: true},
			},
		},
		{
			Name:        "aggregate-results",
			Description: "Aggregate results from parallel computation",
			Template:    AggregateResults,
			Arguments: []PromptArgument{
				{Name: "InputPattern", Description: "Glob pattern for input files", Required: true},
				{Name: "Method", Description: "Aggregation method", Required: true},
				{Name: "OutputPath", Description: "Path for aggregated output", Required: true},
			},
		},
	}
}
