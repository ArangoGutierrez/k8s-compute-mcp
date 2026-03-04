// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"flag"
	"os"
	"strings"
	"testing"

	"k8s.io/klog/v2"
)

func TestNewServer_KlogStructuredOutput(t *testing.T) {
	kubeconfig := createTestKubeconfig(t)
	t.Setenv("KUBECONFIG", kubeconfig)
	t.Setenv("KUBE_CONTEXT", "test-context")
	t.Setenv("PVC_NAME", "test-pvc")
	t.Setenv("PVC_MOUNT_PATH", "/mnt/test")
	t.Setenv("KUEUE_QUEUE", "test-queue")

	// Capture klog to a temp file
	dir := t.TempDir()
	logFile := dir + "/klog.log"
	fs := flag.NewFlagSet("klog-test", flag.ContinueOnError)
	klog.InitFlags(fs)
	_ = fs.Set("log_file", logFile)
	_ = fs.Set("logtostderr", "false")
	_ = fs.Set("alsologtostderr", "false")

	t.Cleanup(func() {
		fs2 := flag.NewFlagSet("klog-reset", flag.ContinueOnError)
		klog.InitFlags(fs2)
		_ = fs2.Set("logtostderr", "true")
		_ = fs2.Set("log_file", "")
	})

	_, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	klog.Flush()
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	output := string(data)

	// Verify structured logging format: klog.InfoS uses key=value pairs
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
	if !strings.Contains(output, "Registered MCP tools") {
		t.Errorf("Expected 'Registered MCP tools' in klog output, got:\n%s", output)
	}
	if !strings.Contains(output, "count") {
		t.Errorf("Expected structured key 'count' in klog output, got:\n%s", output)
	}
}

func TestKlogNoStdlibLogImport(t *testing.T) {
	// Verify that server.go doesn't import "log" stdlib package.
	serverContent, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("Failed to read server.go: %v", err)
	}

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
			t.Error("server.go still imports stdlib \"log\" package -- should use klog/v2")
		}
	}

	if !strings.Contains(string(serverContent), "k8s.io/klog/v2") {
		t.Error("server.go should import k8s.io/klog/v2")
	}
}
