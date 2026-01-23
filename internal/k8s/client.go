// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

// Package k8s provides Kubernetes client functionality and manifest generation.
//
// The client is configured for out-of-cluster usage, loading credentials from
// the user's kubeconfig file (~/.kube/config or KUBECONFIG environment variable).
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client provides access to the Kubernetes API.
type Client struct {
	// clientset is the typed Kubernetes client for core resources.
	clientset kubernetes.Interface

	// dynamicClient is used for CRD operations (MPIJob, JobSet).
	dynamicClient dynamic.Interface

	// config is the REST client configuration.
	config *rest.Config

	// namespace is the default namespace for operations.
	namespace string
}

// NewClient creates a new Kubernetes client using out-of-cluster configuration.
//
// If kubeconfig is empty, it attempts to load from:
// 1. KUBECONFIG environment variable
// 2. ~/.kube/config
func NewClient(kubeconfig string) (*Client, error) {
	// Determine kubeconfig path
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Build config from kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig from %s: %w", kubeconfig, err)
	}

	// Create typed clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Create dynamic client for CRDs
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Get default namespace from kubeconfig context
	namespace := "default"
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfig
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	if ns, _, err := kubeConfig.Namespace(); err == nil && ns != "" {
		namespace = ns
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		config:        config,
		namespace:     namespace,
	}, nil
}

// Namespace returns the default namespace for operations.
func (c *Client) Namespace() string {
	return c.namespace
}

// Clientset returns the typed Kubernetes client.
func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

// DynamicClient returns the dynamic client for CRD operations.
func (c *Client) DynamicClient() dynamic.Interface {
	return c.dynamicClient
}

// Ping verifies connectivity to the Kubernetes API server.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to kubernetes API: %w", err)
	}
	return nil
}
