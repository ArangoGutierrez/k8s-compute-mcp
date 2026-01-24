// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// testKubeconfig creates a temporary kubeconfig file for testing.
func testKubeconfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test kubeconfig: %v", err)
	}
	return path
}

// validKubeconfig returns a minimal valid kubeconfig content.
func validKubeconfig(server string) string {
	return `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: ` + server + `
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
    namespace: test-namespace
  name: test-context
- context:
    cluster: test-cluster
    user: test-user
    namespace: other-namespace
  name: other-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
}

func TestNewClient_WithKubeconfig(t *testing.T) {
	kubeconfigPath := testKubeconfig(t, validKubeconfig("https://127.0.0.1:6443"))

	client, err := NewClient(WithKubeconfig(kubeconfigPath))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.Namespace() != "test-namespace" {
		t.Errorf("Namespace() = %q, want %q", client.Namespace(), "test-namespace")
	}

	if client.Context() != "test-context" {
		t.Errorf("Context() = %q, want %q", client.Context(), "test-context")
	}

	if client.Clientset() == nil {
		t.Error("Clientset() returned nil")
	}

	if client.DynamicClient() == nil {
		t.Error("DynamicClient() returned nil")
	}
}

func TestNewClient_WithContext(t *testing.T) {
	kubeconfigPath := testKubeconfig(t, validKubeconfig("https://127.0.0.1:6443"))

	client, err := NewClient(
		WithKubeconfig(kubeconfigPath),
		WithContext("other-context"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.Namespace() != "other-namespace" {
		t.Errorf("Namespace() = %q, want %q", client.Namespace(), "other-namespace")
	}

	if client.Context() != "other-context" {
		t.Errorf("Context() = %q, want %q", client.Context(), "other-context")
	}
}

func TestNewClient_EnvironmentVariables(t *testing.T) {
	kubeconfigPath := testKubeconfig(t, validKubeconfig("https://127.0.0.1:6443"))

	// Set environment variables
	t.Setenv("KUBECONFIG", kubeconfigPath)
	t.Setenv("KUBE_CONTEXT", "other-context")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.Namespace() != "other-namespace" {
		t.Errorf("Namespace() = %q, want %q", client.Namespace(), "other-namespace")
	}

	if client.Context() != "other-context" {
		t.Errorf("Context() = %q, want %q", client.Context(), "other-context")
	}
}

func TestNewClient_OptionsPrecedence(t *testing.T) {
	kubeconfigPath := testKubeconfig(t, validKubeconfig("https://127.0.0.1:6443"))

	// Set environment variables to wrong values
	t.Setenv("KUBE_CONTEXT", "wrong-context")

	// Options should take precedence
	client, err := NewClient(
		WithKubeconfig(kubeconfigPath),
		WithContext("other-context"),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.Context() != "other-context" {
		t.Errorf("Context() = %q, want %q (options should take precedence)", client.Context(), "other-context")
	}
}

func TestNewClient_InvalidKubeconfig(t *testing.T) {
	kubeconfigPath := testKubeconfig(t, "invalid: yaml: content")

	_, err := NewClient(WithKubeconfig(kubeconfigPath))
	if err == nil {
		t.Error("NewClient() expected error for invalid kubeconfig, got nil")
	}
}

func TestNewClient_MissingKubeconfig(t *testing.T) {
	_, err := NewClient(WithKubeconfig("/nonexistent/path/kubeconfig"))
	if err == nil {
		t.Error("NewClient() expected error for missing kubeconfig, got nil")
	}
}

func TestNewClient_InvalidContext(t *testing.T) {
	kubeconfigPath := testKubeconfig(t, validKubeconfig("https://127.0.0.1:6443"))

	_, err := NewClient(
		WithKubeconfig(kubeconfigPath),
		WithContext("nonexistent-context"),
	)
	if err == nil {
		t.Error("NewClient() expected error for invalid context, got nil")
	}
}

// MockClient creates a Client with mock Kubernetes clients for testing.
// This is useful for testing handlers that use the Client interface.
type MockClient struct {
	*Client
	FakeClientset     *kubefake.Clientset
	FakeDynamicClient *fake.FakeDynamicClient
}

// NewMockClient creates a Client with fake clientsets for unit testing.
// Pass initial objects to pre-populate the fake clients.
func NewMockClient(objects ...runtime.Object) *MockClient {
	//nolint:staticcheck // SA1019: NewSimpleClientset is deprecated but NewClientset requires applyconfig generation
	fakeClientset := kubefake.NewSimpleClientset(objects...)
	fakeDynamic := fake.NewSimpleDynamicClient(runtime.NewScheme())

	return &MockClient{
		Client: &Client{
			clientset:     fakeClientset,
			dynamicClient: fakeDynamic,
			namespace:     "default",
			context:       "mock-context",
		},
		FakeClientset:     fakeClientset,
		FakeDynamicClient: fakeDynamic,
	}
}

func TestMockClient_ListNamespaces(t *testing.T) {
	// Create mock client with a pre-existing namespace
	mockClient := NewMockClient(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ns",
			},
		},
	)

	// Use the fake clientset to list namespaces
	nsList, err := mockClient.Clientset().CoreV1().Namespaces().List(
		context.Background(),
		metav1.ListOptions{},
	)
	if err != nil {
		t.Fatalf("failed to list namespaces: %v", err)
	}

	if len(nsList.Items) != 1 {
		t.Errorf("expected 1 namespace, got %d", len(nsList.Items))
	}

	if nsList.Items[0].Name != "test-ns" {
		t.Errorf("expected namespace name 'test-ns', got %q", nsList.Items[0].Name)
	}
}

func TestMockClient_CreatePod(t *testing.T) {
	mockClient := NewMockClient()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "busybox",
				},
			},
		},
	}

	// Create a pod
	created, err := mockClient.Clientset().CoreV1().Pods("default").Create(
		context.Background(),
		pod,
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("failed to create pod: %v", err)
	}

	if created.Name != "test-pod" {
		t.Errorf("expected pod name 'test-pod', got %q", created.Name)
	}

	// Verify the pod exists
	fetched, err := mockClient.Clientset().CoreV1().Pods("default").Get(
		context.Background(),
		"test-pod",
		metav1.GetOptions{},
	)
	if err != nil {
		t.Fatalf("failed to get pod: %v", err)
	}

	if fetched.Spec.Containers[0].Image != "busybox" {
		t.Errorf("expected image 'busybox', got %q", fetched.Spec.Containers[0].Image)
	}
}

func TestMockClient_Properties(t *testing.T) {
	mockClient := NewMockClient()

	if mockClient.Namespace() != "default" {
		t.Errorf("Namespace() = %q, want %q", mockClient.Namespace(), "default")
	}

	if mockClient.Context() != "mock-context" {
		t.Errorf("Context() = %q, want %q", mockClient.Context(), "mock-context")
	}
}

// Interface compliance test
var _ kubernetes.Interface = (*kubefake.Clientset)(nil)
