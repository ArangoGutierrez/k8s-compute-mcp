// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// testServer creates a Server with mock K8s clients for testing handlers.
// This allows testing handler logic without a real K8s cluster.
func testServer(t *testing.T, objects ...runtime.Object) *Server {
	t.Helper()

	// Create fake clientset with any pre-existing objects
	//nolint:staticcheck // SA1019: NewSimpleClientset is deprecated but works for tests
	fakeClientset := kubefake.NewSimpleClientset(objects...)

	// Create fake dynamic client with CRD scheme
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)

	// Suppress unused variable warnings - these will be used after Task 3-5
	_ = fakeClientset
	_ = fakeDynamic

	// Build server with test config
	// Note: After Task 3-5, we'll inject a mock client here
	return &Server{
		mcpServer: nil, // Not needed for handler tests
		k8sClient: nil, // Will be replaced with mock after Task 3-5
		config: ServerConfig{
			PVCName:      "test-pvc",
			PVCMountPath: "/mnt/data",
			KueueQueue:   "test-queue",
			DefaultImage: "python:3.11-slim",
		},
	}
}
