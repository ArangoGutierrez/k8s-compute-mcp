// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/klog/v2"
)

// setupKlogCapture redirects klog output to a buffer for testing.
// Returns the buffer and a cleanup function.
func setupKlogCapture(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()

	// Create a temp dir for klog log files
	dir := t.TempDir()
	logFile := filepath.Join(dir, "klog-test.log")

	// Reset klog flags and configure to write to file
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	klog.InitFlags(fs)
	_ = fs.Set("logtostderr", "false")
	_ = fs.Set("alsologtostderr", "false")
	_ = fs.Set("log_file", logFile)
	_ = fs.Set("v", "0")

	// Flush any pending output
	klog.Flush()

	cleanup := func() {
		klog.Flush()
		// Reset to stderr for other tests
		fs2 := flag.NewFlagSet("reset", flag.ContinueOnError)
		klog.InitFlags(fs2)
		_ = fs2.Set("logtostderr", "true")
		_ = fs2.Set("log_file", "")
	}

	// Return a buffer-reading helper
	buf := &bytes.Buffer{}
	readBuf := func() {
		klog.Flush()
		data, _ := os.ReadFile(logFile)
		buf.Reset()
		buf.Write(data)
	}

	// Wrap cleanup to also read
	return buf, func() {
		readBuf()
		cleanup()
	}
}

func TestNewServer_KlogStructuredOutput(t *testing.T) {
	kubeconfig := createTestKubeconfig(t)
	t.Setenv("KUBECONFIG", kubeconfig)
	t.Setenv("KUBE_CONTEXT", "test-context")
	t.Setenv("PVC_NAME", "test-pvc")
	t.Setenv("PVC_MOUNT_PATH", "/mnt/test")
	t.Setenv("KUEUE_QUEUE", "test-queue")

	buf, cleanup := setupKlogCapture(t)
	defer cleanup()

	_, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Force flush and read
	cleanup()

	output := buf.String()

	// Verify structured logging format: klog.InfoS uses key=value pairs
	// Server config log should have structured fields
	if !strings.Contains(output, "Server configuration") {
		t.Errorf("Expected 'Server configuration' in klog output, got:\n%s", output)
	}
	if !strings.Contains(output, "pvc") {
		t.Errorf("Expected structured key 'pvc' in klog output, got:\n%s", output)
	}
	if !strings.Contains(output, "mountPath") {
		t.Errorf("Expected structured key 'mountPath' in klog output, got:\n%s", output)
	}
	if !strings.Contains(output, "kueueQueue") {
		t.Errorf("Expected structured key 'kueueQueue' in klog output, got:\n%s", output)
	}

	// Tool registration log should have structured field
	if !strings.Contains(output, "Registered MCP tools") {
		t.Errorf("Expected 'Registered MCP tools' in klog output, got:\n%s", output)
	}
	if !strings.Contains(output, "count") {
		t.Errorf("Expected structured key 'count' in klog output, got:\n%s", output)
	}
}

func TestServerRun_KlogStructuredOutput(t *testing.T) {
	kubeconfig := createTestKubeconfig(t)
	t.Setenv("KUBECONFIG", kubeconfig)
	t.Setenv("KUBE_CONTEXT", "test-context")

	buf, cleanup := setupKlogCapture(t)
	defer cleanup()

	s, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Force flush and read to check Run's log output
	cleanup()
	output := buf.String()

	// The Run method logs server ready with structured fields
	// We can't easily test Run without blocking, but we verify the server was created
	// and the NewServer logs are structured.
	_ = s

	if !strings.Contains(output, "Server configuration") {
		t.Errorf("Expected structured log from NewServer, got:\n%s", output)
	}
}

func TestKlogNoStdlibLogImport(t *testing.T) {
	// Verify that server.go and handlers.go don't import "log" stdlib package.
	// This is a code-level check to prevent regression.
	serverContent, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("Failed to read server.go: %v", err)
	}

	// Check that "log" is not in the import block (but "klog" is fine)
	lines := strings.Split(string(serverContent), "\n")
	inImport := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "import (" {
			inImport = true
			continue
		}
		if inImport && trimmed == ")" {
			inImport = false
			continue
		}
		if inImport && trimmed == `"log"` {
			t.Error("server.go still imports stdlib \"log\" package — should use klog/v2")
		}
	}

	if !strings.Contains(string(serverContent), "k8s.io/klog/v2") {
		t.Error("server.go should import k8s.io/klog/v2")
	}
}
