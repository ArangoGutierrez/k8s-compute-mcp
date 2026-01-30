// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

// Package k8s provides Kubernetes client functionality and manifest generation.
//
// The client is configured for out-of-cluster usage, loading credentials from
// the user's kubeconfig file (~/.kube/config or KUBECONFIG environment variable).
// A specific context can be selected via the KUBE_CONTEXT environment variable.
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	// context is the kubeconfig context used for this client.
	context string
}

// ClientOption configures a Client.
type ClientOption func(*clientOptions)

type clientOptions struct {
	kubeconfig string
	context    string
}

// WithKubeconfig sets the path to the kubeconfig file.
func WithKubeconfig(path string) ClientOption {
	return func(o *clientOptions) {
		o.kubeconfig = path
	}
}

// WithContext sets the kubeconfig context to use.
func WithContext(ctx string) ClientOption {
	return func(o *clientOptions) {
		o.context = ctx
	}
}

// NewClient creates a new Kubernetes client using out-of-cluster configuration.
//
// Configuration is resolved in the following order:
//  1. Options passed via ClientOption (WithKubeconfig, WithContext)
//  2. Environment variables (KUBECONFIG, KUBE_CONTEXT)
//  3. Default paths (~/.kube/config) and current context
func NewClient(opts ...ClientOption) (*Client, error) {
	options := &clientOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Determine kubeconfig path
	kubeconfig := options.kubeconfig
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

	// Determine context
	kubeContext := options.context
	if kubeContext == "" {
		kubeContext = os.Getenv("KUBE_CONTEXT")
	}

	// Set up loading rules and overrides
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfig

	configOverrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		configOverrides.CurrentContext = kubeContext
	}

	// Build config from kubeconfig file with context override
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)

	config, err := clientConfig.ClientConfig()
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

	// Get namespace from kubeconfig context (or default)
	namespace := "default"
	if ns, _, err := clientConfig.Namespace(); err == nil && ns != "" {
		namespace = ns
	}

	// Resolve actual context name used
	rawConfig, err := clientConfig.RawConfig()
	if err == nil && kubeContext == "" {
		kubeContext = rawConfig.CurrentContext
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		config:        config,
		namespace:     namespace,
		context:       kubeContext,
	}, nil
}

// Namespace returns the default namespace for operations.
func (c *Client) Namespace() string {
	return c.namespace
}

// Context returns the kubeconfig context name used by this client.
func (c *Client) Context() string {
	return c.context
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

// K8sClientInterface defines the methods used by MCP handlers.
// This interface enables mocking for unit tests.
type K8sClientInterface interface {
	Namespace() string
	Context() string
	SubmitMPIJob(ctx context.Context, mpijob *unstructured.Unstructured) error
	SubmitJobSet(ctx context.Context, jobset *unstructured.Unstructured) error
	SubmitJob(ctx context.Context, job *batchv1.Job) error
	GetMPIJobStatus(ctx context.Context, namespace, name string) (string, error)
	GetJobSetStatus(ctx context.Context, namespace, name string) (string, error)
	GetJobStatus(ctx context.Context, namespace, name string) (*batchv1.JobStatus, error)
	ReadArtifactHead(ctx context.Context, cfg ReadArtifactConfig) (*ReadArtifactResult, error)
}

// Ensure Client implements K8sClientInterface
var _ K8sClientInterface = (*Client)(nil)
