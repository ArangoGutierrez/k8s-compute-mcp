//go:build e2e

// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	ctx       context.Context
	cancel    context.CancelFunc
	mcpClient *MCPClient
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Test Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Minute)

	By("Creating KIND cluster")
	Expect(CreateKindCluster(ctx)).To(Succeed())

	By("Waiting for cluster to be ready")
	Expect(WaitForClusterReady(ctx)).To(Succeed())

	By("Loading custom test images")
	Expect(LoadTestImages(ctx)).To(Succeed())

	By("Setting up storage (PV/PVC)")
	Expect(SetupStorage(ctx)).To(Succeed())

	By("Installing operators (Kueue, MPI Operator, JobSet)")
	Expect(InstallOperators(ctx)).To(Succeed())

	By("Starting MCP server")
	// Configure MCP server to use test PVC
	os.Setenv("PVC_NAME", "shared-data")

	var err error
	mcpClient, err = NewMCPClient(ctx)
	Expect(err).NotTo(HaveOccurred())

	// Wait for server initialization
	time.Sleep(2 * time.Second)
})

var _ = AfterSuite(func() {
	defer cancel()

	if mcpClient != nil {
		By("Stopping MCP server")
		Expect(mcpClient.Close()).To(Succeed())
	}

	if os.Getenv("E2E_KEEP_CLUSTER") != "true" {
		By("Deleting KIND cluster")
		Expect(DeleteKindCluster(ctx)).To(Succeed())
	} else {
		GinkgoWriter.Println("Keeping cluster (E2E_KEEP_CLUSTER=true)")
	}
})
