// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
)

// MPIJobGVR is the GroupVersionResource for Kubeflow MPIJob.
var MPIJobGVR = schema.GroupVersionResource{
	Group:    "kubeflow.org",
	Version:  "v2beta1",
	Resource: "mpijobs",
}

// MPIJobConfig contains configuration for building an MPIJob manifest.
type MPIJobConfig struct {
	// Name is the job name.
	Name string

	// Namespace is the target namespace.
	Namespace string

	// Replicas is the number of worker replicas.
	Replicas int

	// Code is the source code to execute.
	Code string

	// Language is the programming language (python or cpp).
	Language string

	// Image is the container image to use.
	Image string

	// KueueQueue is the Kueue queue name for scheduling.
	KueueQueue string

	// PVCName is the name of the shared PVC.
	PVCName string

	// MountPath is the mount path for the PVC.
	MountPath string
}

// BuildMPIJob creates an unstructured MPIJob manifest.
//
// The generated MPIJob has:
//   - Launcher pod (1 replica): Runs mpirun to coordinate the job
//   - Worker pods (cfg.Replicas): Execute the actual MPI computation
//
// Note: We use unstructured here to avoid importing the full mpi-operator
// dependency. For production, consider importing the typed structs from
// github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1.
func BuildMPIJob(cfg MPIJobConfig) (*unstructured.Unstructured, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("MPIJob name is required")
	}
	if cfg.Replicas < 1 {
		return nil, fmt.Errorf("MPIJob replicas must be >= 1")
	}
	if cfg.Code == "" {
		return nil, fmt.Errorf("MPIJob code is required")
	}

	// Set defaults
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	if cfg.Image == "" {
		cfg.Image = "mpioperator/openmpi-builder:latest"
	}
	if cfg.MountPath == "" {
		cfg.MountPath = "/mnt/data"
	}
	if cfg.Language == "" {
		cfg.Language = "python"
	}

	// Build command based on language
	command, args := buildMPICommand(cfg)

	// Build container spec (reused for launcher and worker with slight modifications)
	workerContainer := buildMPIContainer("worker", cfg.Image, nil, nil, cfg.PVCName, cfg.MountPath)
	launcherContainer := buildMPIContainer("launcher", cfg.Image, command, args, cfg.PVCName, cfg.MountPath)

	// Build volume specs if PVC is configured
	var volumes []interface{}
	if cfg.PVCName != "" {
		volumes = []interface{}{
			map[string]interface{}{
				"name": "data",
				"persistentVolumeClaim": map[string]interface{}{
					"claimName": cfg.PVCName,
				},
			},
		}
	}

	// Build Launcher pod template
	launcherPodSpec := map[string]interface{}{
		"containers":    []interface{}{launcherContainer},
		"restartPolicy": "Never",
	}
	if len(volumes) > 0 {
		launcherPodSpec["volumes"] = volumes
	}

	// Build Worker pod template
	workerPodSpec := map[string]interface{}{
		"containers":    []interface{}{workerContainer},
		"restartPolicy": "Never",
	}
	if len(volumes) > 0 {
		workerPodSpec["volumes"] = volumes
	}

	// Build MPIJob spec
	spec := map[string]interface{}{
		"slotsPerWorker": int64(1),
		"runPolicy": map[string]interface{}{
			"cleanPodPolicy": "Running",
		},
		"mpiReplicaSpecs": map[string]interface{}{
			"Launcher": map[string]interface{}{
				"replicas": int64(1),
				"template": map[string]interface{}{
					"spec": launcherPodSpec,
				},
			},
			"Worker": map[string]interface{}{
				"replicas": int64(cfg.Replicas),
				"template": map[string]interface{}{
					"spec": workerPodSpec,
				},
			},
		},
	}

	// Build metadata
	metadata := map[string]interface{}{
		"name":      cfg.Name,
		"namespace": cfg.Namespace,
	}

	// Add Kueue annotation if specified
	if cfg.KueueQueue != "" {
		metadata["annotations"] = map[string]interface{}{
			"kueue.x-k8s.io/queue-name": cfg.KueueQueue,
		}
	}

	mpijob := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubeflow.org/v2beta1",
			"kind":       "MPIJob",
			"metadata":   metadata,
			"spec":       spec,
		},
	}

	return mpijob, nil
}

// buildMPICommand constructs the mpirun command based on language.
// Returns the command and args separately for the container spec.
func buildMPICommand(cfg MPIJobConfig) ([]string, []string) {
	switch cfg.Language {
	case "python":
		// For Python: mpirun python -c "<code>"
		return []string{"mpirun"}, []string{
			"--allow-run-as-root",
			"-np", fmt.Sprintf("%d", cfg.Replicas),
			"python", "-c", cfg.Code,
		}
	case "cpp", "c++":
		// For C++: Write code to file using heredoc, compile with mpicxx, run with mpirun.
		// Single-quoted heredoc delimiter prevents ALL shell expansion,
		// making this safe against shell injection regardless of code content.
		// Generate a unique delimiter that doesn't appear on its own line in the code.
		delimiter := uniqueHeredocDelimiter(cfg.Code)
		script := fmt.Sprintf("set -e\ncat <<'%s' > /tmp/mpi_code.cpp\n%s\n%s\nmpicxx -o /tmp/mpi_program /tmp/mpi_code.cpp\nmpirun --allow-run-as-root -np %d /tmp/mpi_program\n",
			delimiter, cfg.Code, delimiter, cfg.Replicas)
		return []string{"/bin/sh"}, []string{"-c", script}
	default:
		// Default to Python
		return []string{"mpirun"}, []string{
			"--allow-run-as-root",
			"-np", fmt.Sprintf("%d", cfg.Replicas),
			"python", "-c", cfg.Code,
		}
	}
}

// uniqueHeredocDelimiter returns a heredoc delimiter that does not appear
// on its own line in the given code. This prevents early heredoc termination
// if user code contains the delimiter string.
func uniqueHeredocDelimiter(code string) string {
	delimiter := "CPPEOF"
	for code == delimiter ||
		strings.HasPrefix(code, delimiter+"\n") ||
		strings.HasSuffix(code, "\n"+delimiter) ||
		strings.Contains(code, "\n"+delimiter+"\n") {
		delimiter = fmt.Sprintf("CPPEOF_%s", rand.String(8))
	}
	return delimiter
}

// buildMPIContainer creates a container spec for MPIJob.
// For workers, command and args should be nil (mpi-operator handles this).
func buildMPIContainer(name, image string, command, args []string, pvcName, mountPath string) map[string]interface{} {
	container := map[string]interface{}{
		"name":  name,
		"image": image,
		"env": []interface{}{
			map[string]interface{}{
				"name":  "MOUNT_PATH",
				"value": mountPath,
			},
		},
		"resources": map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    "1",
				"memory": "2Gi",
			},
			"requests": map[string]interface{}{
				"cpu":    "500m",
				"memory": "1Gi",
			},
		},
	}

	// Only set command/args for launcher (workers are controlled by mpi-operator)
	if len(command) > 0 {
		container["command"] = toInterfaceSlice(command)
	}
	if len(args) > 0 {
		container["args"] = toInterfaceSlice(args)
	}

	// Add volume mount if PVC is configured
	if pvcName != "" {
		container["volumeMounts"] = []interface{}{
			map[string]interface{}{
				"name":      "data",
				"mountPath": mountPath,
			},
		}
	}

	return container
}

// toInterfaceSlice converts a string slice to an interface slice.
func toInterfaceSlice(ss []string) []interface{} {
	result := make([]interface{}, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

// SubmitMPIJob applies an MPIJob manifest to the cluster.
func (c *Client) SubmitMPIJob(ctx context.Context, mpijob *unstructured.Unstructured) error {
	namespace := mpijob.GetNamespace()
	if namespace == "" {
		namespace = c.namespace
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := c.dynamicClient.Resource(MPIJobGVR).Namespace(namespace).Create(ctx, mpijob, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create MPIJob: %w", err)
	}

	return nil
}

// GetMPIJobStatus retrieves the status of an MPIJob.
func (c *Client) GetMPIJobStatus(ctx context.Context, namespace, name string) (string, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	mpijob, err := c.dynamicClient.Resource(MPIJobGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get MPIJob %s/%s: %w", namespace, name, err)
	}

	return extractConditionPhase(mpijob.Object)
}
