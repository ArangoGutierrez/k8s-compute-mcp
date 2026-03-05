//go:build e2e

// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/onsi/gomega"
)

var requestIDCounter int64

// MCPClient wraps MCP server interaction over stdio.
type MCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	mu     sync.Mutex
}

// MCPRequest is a JSON-RPC 2.0 request.
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int64                  `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

// MCPResponse is a JSON-RPC 2.0 response.
type MCPResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int64                  `json:"id"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   *MCPError              `json:"error,omitempty"`
}

// GetStructuredContent extracts structuredContent from MCP tool call response.
// mcp-go wraps tool results in result.structuredContent.
func (r *MCPResponse) GetStructuredContent() map[string]interface{} {
	if r.Result == nil {
		return nil
	}

	// mcp-go puts tool results in structuredContent
	if structured, ok := r.Result["structuredContent"].(map[string]interface{}); ok {
		return structured
	}

	return nil
}

// IsError checks if the response indicates an error (either via Error or isError flag).
func (r *MCPResponse) IsError() bool {
	if r.Error != nil {
		return true
	}
	if r.Result != nil {
		if isErr, ok := r.Result["isError"].(bool); ok && isErr {
			return true
		}
	}
	return false
}

// GetErrorMessage returns the error message from either Error or content[0].text.
func (r *MCPResponse) GetErrorMessage() string {
	if r.Error != nil {
		return r.Error.Message
	}
	if r.Result != nil {
		// Check for error in content array
		if contentArr, ok := r.Result["content"].([]interface{}); ok && len(contentArr) > 0 {
			if contentItem, ok := contentArr[0].(map[string]interface{}); ok {
				if text, ok := contentItem["text"].(string); ok {
					return text
				}
			}
		}
	}
	return ""
}

// MCPError is a JSON-RPC 2.0 error object.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewMCPClient creates and starts an MCP client connected to the server.
func NewMCPClient(ctx context.Context) (*MCPClient, error) {
	// Build server binary first - use ../../ to get to project root from test/e2e/
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "../../bin/server", "../../cmd/server")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to build server: %w\nOutput: %s", err, output)
	}

	// Get absolute path to server binary
	serverPath, err := filepath.Abs("../../bin/server")
	if err != nil {
		return nil, fmt.Errorf("failed to get server path: %w", err)
	}

	// Start server process
	cmd := exec.CommandContext(ctx, serverPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Capture stderr for debugging
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintf(os.Stderr, "[MCP Server] %s\n", scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	fmt.Println("MCP server started successfully")

	client := &MCPClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	// Perform MCP protocol initialization
	if err := client.initialize(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	fmt.Println("MCP connection initialized successfully")

	return client, nil
}

// initialize performs the MCP protocol handshake.
func (c *MCPClient) initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	reqID := atomic.AddInt64(&requestIDCounter, 1)

	// Send initialize request
	initReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"clientInfo": map[string]interface{}{
				"name":    "k8s-compute-mcp-e2e-test",
				"version": "1.0.0",
			},
		},
	}

	if err := json.NewEncoder(c.stdin).Encode(initReq); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Read initialize response
	var initResp MCPResponse
	decoder := json.NewDecoder(c.stdout)
	if err := decoder.Decode(&initResp); err != nil {
		return fmt.Errorf("failed to decode initialize response: %w", err)
	}

	if initResp.Error != nil {
		return fmt.Errorf("initialize failed: %s", initResp.Error.Message)
	}

	// Note: mcp-go doesn't require notifications/initialized, skip it
	return nil
}

// CallTool calls an MCP tool and returns the response.
func (c *MCPClient) CallTool(name string, args map[string]interface{}) *MCPResponse {
	c.mu.Lock()
	defer c.mu.Unlock()

	reqID := atomic.AddInt64(&requestIDCounter, 1)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}

	// Send request
	if err := json.NewEncoder(c.stdin).Encode(req); err != nil {
		return &MCPResponse{
			Error: &MCPError{
				Code:    -1,
				Message: fmt.Sprintf("failed to send request: %v", err),
			},
		}
	}

	// Read response
	var resp MCPResponse
	decoder := json.NewDecoder(c.stdout)
	if err := decoder.Decode(&resp); err != nil {
		return &MCPResponse{
			Error: &MCPError{
				Code:    -1,
				Message: fmt.Sprintf("failed to decode response: %v", err),
			},
		}
	}

	return &resp
}

// Close stops the MCP server.
func (c *MCPClient) Close() error {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			return err
		}
		_ = c.cmd.Wait()
	}

	fmt.Println("MCP server stopped")
	return nil
}

// ReadFixture reads a test fixture file.
func ReadFixture(name string) string {
	path := filepath.Join("fixtures", name)
	content, err := os.ReadFile(path)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "failed to read fixture %s", name)
	return string(content)
}
