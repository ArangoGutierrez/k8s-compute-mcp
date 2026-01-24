// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildMPIJob(t *testing.T) {
	tests := []struct {
		name        string
		cfg         MPIJobConfig
		wantErr     bool
		errContains string
		validate    func(*testing.T, *unstructured.Unstructured)
	}{
		{
			name: "valid python job",
			cfg: MPIJobConfig{
				Name:      "test-mpi",
				Namespace: "default",
				Replicas:  4,
				Code:      "print('hello from MPI')",
				Language:  "python",
				Image:     "mpioperator/openmpi:latest",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				// Check basic metadata
				if got := u.GetName(); got != "test-mpi" {
					t.Errorf("name = %q, want %q", got, "test-mpi")
				}
				if got := u.GetNamespace(); got != "default" {
					t.Errorf("namespace = %q, want %q", got, "default")
				}
				if got := u.GetKind(); got != "MPIJob" {
					t.Errorf("kind = %q, want %q", got, "MPIJob")
				}
				if got := u.GetAPIVersion(); got != "kubeflow.org/v2beta1" {
					t.Errorf("apiVersion = %q, want %q", got, "kubeflow.org/v2beta1")
				}

				// Check launcher spec exists
				launcher, found, err := unstructured.NestedMap(u.Object, "spec", "mpiReplicaSpecs", "Launcher")
				if err != nil || !found {
					t.Fatalf("Launcher spec not found: %v", err)
				}
				launcherReplicas, _, _ := unstructured.NestedInt64(launcher, "replicas")
				if launcherReplicas != 1 {
					t.Errorf("launcher replicas = %d, want 1", launcherReplicas)
				}

				// Check worker spec exists
				worker, found, err := unstructured.NestedMap(u.Object, "spec", "mpiReplicaSpecs", "Worker")
				if err != nil || !found {
					t.Fatalf("Worker spec not found: %v", err)
				}
				workerReplicas, _, _ := unstructured.NestedInt64(worker, "replicas")
				if workerReplicas != 4 {
					t.Errorf("worker replicas = %d, want 4", workerReplicas)
				}
			},
		},
		{
			name: "valid cpp job",
			cfg: MPIJobConfig{
				Name:      "test-mpi-cpp",
				Namespace: "compute",
				Replicas:  2,
				Code:      "#include <mpi.h>\nint main() { MPI_Init(NULL,NULL); MPI_Finalize(); }",
				Language:  "cpp",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				if got := u.GetName(); got != "test-mpi-cpp" {
					t.Errorf("name = %q, want %q", got, "test-mpi-cpp")
				}

				// Verify launcher has shell command for C++ compilation
				containers, _, _ := unstructured.NestedSlice(u.Object, "spec", "mpiReplicaSpecs", "Launcher", "template", "spec", "containers")
				if len(containers) == 0 {
					t.Fatal("no launcher containers found")
				}
				container := containers[0].(map[string]interface{})
				command, _, _ := unstructured.NestedStringSlice(container, "command")
				if len(command) == 0 || command[0] != "/bin/sh" {
					t.Errorf("cpp command = %v, want /bin/sh wrapper", command)
				}
			},
		},
		{
			name: "with PVC",
			cfg: MPIJobConfig{
				Name:      "test-mpi-pvc",
				Namespace: "default",
				Replicas:  2,
				Code:      "print('hello')",
				Language:  "python",
				PVCName:   "shared-data",
				MountPath: "/mnt/data",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				// Check volumes in launcher
				volumes, found, _ := unstructured.NestedSlice(u.Object, "spec", "mpiReplicaSpecs", "Launcher", "template", "spec", "volumes")
				if !found || len(volumes) == 0 {
					t.Error("launcher volumes not found")
					return
				}
				vol := volumes[0].(map[string]interface{})
				pvcClaim, _, _ := unstructured.NestedString(vol, "persistentVolumeClaim", "claimName")
				if pvcClaim != "shared-data" {
					t.Errorf("pvc claimName = %q, want %q", pvcClaim, "shared-data")
				}

				// Check volume mounts in launcher container
				containers, _, _ := unstructured.NestedSlice(u.Object, "spec", "mpiReplicaSpecs", "Launcher", "template", "spec", "containers")
				if len(containers) == 0 {
					t.Fatal("no containers found")
				}
				container := containers[0].(map[string]interface{})
				mounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
				if len(mounts) == 0 {
					t.Error("volume mounts not found")
					return
				}
				mount := mounts[0].(map[string]interface{})
				mountPath, _, _ := unstructured.NestedString(mount, "mountPath")
				if mountPath != "/mnt/data" {
					t.Errorf("mountPath = %q, want %q", mountPath, "/mnt/data")
				}
			},
		},
		{
			name: "with Kueue queue",
			cfg: MPIJobConfig{
				Name:       "test-mpi-kueue",
				Namespace:  "default",
				Replicas:   2,
				Code:       "print('hello')",
				Language:   "python",
				KueueQueue: "default-queue",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				annotations := u.GetAnnotations()
				if annotations == nil {
					t.Fatal("annotations are nil")
				}
				queue, ok := annotations["kueue.x-k8s.io/queue-name"]
				if !ok {
					t.Error("kueue annotation not found")
					return
				}
				if queue != "default-queue" {
					t.Errorf("kueue queue = %q, want %q", queue, "default-queue")
				}
			},
		},
		{
			name: "defaults applied",
			cfg: MPIJobConfig{
				Name:     "test-defaults",
				Replicas: 2,
				Code:     "print('hello')",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				// Check default namespace
				if got := u.GetNamespace(); got != "default" {
					t.Errorf("namespace = %q, want %q", got, "default")
				}
				// Check default image
				containers, _, _ := unstructured.NestedSlice(u.Object, "spec", "mpiReplicaSpecs", "Launcher", "template", "spec", "containers")
				if len(containers) == 0 {
					t.Fatal("no containers found")
				}
				container := containers[0].(map[string]interface{})
				image, _, _ := unstructured.NestedString(container, "image")
				if image != "mpioperator/openmpi-builder:latest" {
					t.Errorf("image = %q, want default image", image)
				}
			},
		},
		{
			name:        "missing name",
			cfg:         MPIJobConfig{Replicas: 2, Code: "print('hello')"},
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name:        "missing code",
			cfg:         MPIJobConfig{Name: "test", Replicas: 2},
			wantErr:     true,
			errContains: "code is required",
		},
		{
			name:        "zero replicas",
			cfg:         MPIJobConfig{Name: "test", Replicas: 0, Code: "print('hello')"},
			wantErr:     true,
			errContains: "replicas must be >= 1",
		},
		{
			name:        "negative replicas",
			cfg:         MPIJobConfig{Name: "test", Replicas: -1, Code: "print('hello')"},
			wantErr:     true,
			errContains: "replicas must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildMPIJob(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("BuildMPIJob() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("BuildMPIJob() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildMPIJob() unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}

func TestBuildJobSet(t *testing.T) {
	tests := []struct {
		name        string
		cfg         JobSetConfig
		wantErr     bool
		errContains string
		validate    func(*testing.T, *unstructured.Unstructured)
	}{
		{
			name: "valid monte carlo job",
			cfg: JobSetConfig{
				Name:      "test-mc",
				Namespace: "default",
				Replicas:  10,
				Script:    "import os; print(f'Worker {os.environ[\"JOB_COMPLETION_INDEX\"]}')",
				Image:     "python:3.11-slim",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				// Check basic metadata
				if got := u.GetName(); got != "test-mc" {
					t.Errorf("name = %q, want %q", got, "test-mc")
				}
				if got := u.GetNamespace(); got != "default" {
					t.Errorf("namespace = %q, want %q", got, "default")
				}
				if got := u.GetKind(); got != "JobSet" {
					t.Errorf("kind = %q, want %q", got, "JobSet")
				}
				if got := u.GetAPIVersion(); got != "jobset.x-k8s.io/v1alpha2" {
					t.Errorf("apiVersion = %q, want %q", got, "jobset.x-k8s.io/v1alpha2")
				}

				// Check replicatedJobs
				replicatedJobs, found, _ := unstructured.NestedSlice(u.Object, "spec", "replicatedJobs")
				if !found || len(replicatedJobs) == 0 {
					t.Fatal("replicatedJobs not found")
				}

				// Check completionMode is Indexed
				job := replicatedJobs[0].(map[string]interface{})
				completionMode, _, _ := unstructured.NestedString(job, "template", "spec", "completionMode")
				if completionMode != "Indexed" {
					t.Errorf("completionMode = %q, want %q", completionMode, "Indexed")
				}

				// Check completions matches replicas
				completions, _, _ := unstructured.NestedInt64(job, "template", "spec", "completions")
				if completions != 10 {
					t.Errorf("completions = %d, want 10", completions)
				}

				// Check parallelism matches replicas
				parallelism, _, _ := unstructured.NestedInt64(job, "template", "spec", "parallelism")
				if parallelism != 10 {
					t.Errorf("parallelism = %d, want 10", parallelism)
				}
			},
		},
		{
			name: "with PVC",
			cfg: JobSetConfig{
				Name:      "test-mc-pvc",
				Namespace: "default",
				Replicas:  5,
				Script:    "print('hello')",
				PVCName:   "shared-data",
				MountPath: "/mnt/data",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				// Navigate to pod spec volumes
				replicatedJobs, _, _ := unstructured.NestedSlice(u.Object, "spec", "replicatedJobs")
				job := replicatedJobs[0].(map[string]interface{})
				volumes, found, _ := unstructured.NestedSlice(job, "template", "spec", "template", "spec", "volumes")
				if !found || len(volumes) == 0 {
					t.Error("volumes not found")
					return
				}
				vol := volumes[0].(map[string]interface{})
				pvcClaim, _, _ := unstructured.NestedString(vol, "persistentVolumeClaim", "claimName")
				if pvcClaim != "shared-data" {
					t.Errorf("pvc claimName = %q, want %q", pvcClaim, "shared-data")
				}

				// Check volume mounts
				containers, _, _ := unstructured.NestedSlice(job, "template", "spec", "template", "spec", "containers")
				if len(containers) == 0 {
					t.Fatal("no containers found")
				}
				container := containers[0].(map[string]interface{})
				mounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
				if len(mounts) == 0 {
					t.Error("volume mounts not found")
					return
				}
				mount := mounts[0].(map[string]interface{})
				mountPath, _, _ := unstructured.NestedString(mount, "mountPath")
				if mountPath != "/mnt/data" {
					t.Errorf("mountPath = %q, want %q", mountPath, "/mnt/data")
				}
			},
		},
		{
			name: "with Kueue queue",
			cfg: JobSetConfig{
				Name:       "test-mc-kueue",
				Namespace:  "default",
				Replicas:   5,
				Script:     "print('hello')",
				KueueQueue: "ml-queue",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				annotations := u.GetAnnotations()
				if annotations == nil {
					t.Fatal("annotations are nil")
				}
				queue, ok := annotations["kueue.x-k8s.io/queue-name"]
				if !ok {
					t.Error("kueue annotation not found")
					return
				}
				if queue != "ml-queue" {
					t.Errorf("kueue queue = %q, want %q", queue, "ml-queue")
				}
			},
		},
		{
			name: "defaults applied",
			cfg: JobSetConfig{
				Name:     "test-defaults",
				Replicas: 3,
				Script:   "print('hello')",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				// Check default namespace
				if got := u.GetNamespace(); got != "default" {
					t.Errorf("namespace = %q, want %q", got, "default")
				}
				// Check default image
				replicatedJobs, _, _ := unstructured.NestedSlice(u.Object, "spec", "replicatedJobs")
				job := replicatedJobs[0].(map[string]interface{})
				containers, _, _ := unstructured.NestedSlice(job, "template", "spec", "template", "spec", "containers")
				if len(containers) == 0 {
					t.Fatal("no containers found")
				}
				container := containers[0].(map[string]interface{})
				image, _, _ := unstructured.NestedString(container, "image")
				if image != "python:3.11-slim" {
					t.Errorf("image = %q, want %q", image, "python:3.11-slim")
				}
			},
		},
		{
			name: "MOUNT_PATH env var present",
			cfg: JobSetConfig{
				Name:      "test-env",
				Replicas:  2,
				Script:    "print('hello')",
				MountPath: "/custom/path",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				replicatedJobs, _, _ := unstructured.NestedSlice(u.Object, "spec", "replicatedJobs")
				job := replicatedJobs[0].(map[string]interface{})
				containers, _, _ := unstructured.NestedSlice(job, "template", "spec", "template", "spec", "containers")
				if len(containers) == 0 {
					t.Fatal("no containers found")
				}
				container := containers[0].(map[string]interface{})
				env, _, _ := unstructured.NestedSlice(container, "env")
				if len(env) == 0 {
					t.Fatal("no env vars found")
				}

				found := false
				for _, e := range env {
					envVar := e.(map[string]interface{})
					name, _, _ := unstructured.NestedString(envVar, "name")
					value, _, _ := unstructured.NestedString(envVar, "value")
					if name == "MOUNT_PATH" && value == "/custom/path" {
						found = true
						break
					}
				}
				if !found {
					t.Error("MOUNT_PATH env var not found or incorrect value")
				}
			},
		},
		{
			name: "successPolicy configured",
			cfg: JobSetConfig{
				Name:     "test-success",
				Replicas: 2,
				Script:   "print('hello')",
			},
			validate: func(t *testing.T, u *unstructured.Unstructured) {
				operator, found, _ := unstructured.NestedString(u.Object, "spec", "successPolicy", "operator")
				if !found {
					t.Error("successPolicy.operator not found")
					return
				}
				if operator != "All" {
					t.Errorf("successPolicy.operator = %q, want %q", operator, "All")
				}
			},
		},
		{
			name:        "missing name",
			cfg:         JobSetConfig{Replicas: 2, Script: "print('hello')"},
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name:        "missing script",
			cfg:         JobSetConfig{Name: "test", Replicas: 2},
			wantErr:     true,
			errContains: "script is required",
		},
		{
			name:        "zero replicas",
			cfg:         JobSetConfig{Name: "test", Replicas: 0, Script: "print('hello')"},
			wantErr:     true,
			errContains: "replicas must be >= 1",
		},
		{
			name:        "negative replicas",
			cfg:         JobSetConfig{Name: "test", Replicas: -1, Script: "print('hello')"},
			wantErr:     true,
			errContains: "replicas must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildJobSet(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("BuildJobSet() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("BuildJobSet() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildJobSet() unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}

func TestBuildReducerJob(t *testing.T) {
	tests := []struct {
		name        string
		cfg         ReducerJobConfig
		wantErr     bool
		errContains string
		validate    func(*testing.T, *batchv1.Job)
	}{
		{
			name: "valid reducer job",
			cfg: ReducerJobConfig{
				Name:         "test-reducer",
				Namespace:    "default",
				Script:       "import glob; print(len(glob.glob('/mnt/data/*.json')))",
				InputPattern: "/mnt/data/*.json",
				Image:        "python:3.11-slim",
			},
			validate: func(t *testing.T, job *batchv1.Job) {
				if job.Name != "test-reducer" {
					t.Errorf("name = %q, want %q", job.Name, "test-reducer")
				}
				if job.Namespace != "default" {
					t.Errorf("namespace = %q, want %q", job.Namespace, "default")
				}
			},
		},
		{
			name: "with PVC",
			cfg: ReducerJobConfig{
				Name:      "test-reducer-pvc",
				Namespace: "default",
				Script:    "print('hello')",
				PVCName:   "shared-data",
				MountPath: "/mnt/data",
			},
			validate: func(t *testing.T, job *batchv1.Job) {
				if len(job.Spec.Template.Spec.Volumes) == 0 {
					t.Error("volumes not found")
					return
				}
				vol := job.Spec.Template.Spec.Volumes[0]
				if vol.PersistentVolumeClaim.ClaimName != "shared-data" {
					t.Errorf("pvc claimName = %q, want %q", vol.PersistentVolumeClaim.ClaimName, "shared-data")
				}
			},
		},
		{
			name: "with Kueue queue",
			cfg: ReducerJobConfig{
				Name:       "test-reducer-kueue",
				Namespace:  "default",
				Script:     "print('hello')",
				KueueQueue: "default-queue",
			},
			validate: func(t *testing.T, job *batchv1.Job) {
				queue, ok := job.Annotations["kueue.x-k8s.io/queue-name"]
				if !ok {
					t.Error("kueue annotation not found")
					return
				}
				if queue != "default-queue" {
					t.Errorf("kueue queue = %q, want %q", queue, "default-queue")
				}
			},
		},
		{
			name:        "missing name",
			cfg:         ReducerJobConfig{Script: "print('hello')"},
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name:        "missing script",
			cfg:         ReducerJobConfig{Name: "test"},
			wantErr:     true,
			errContains: "script is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildReducerJob(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("BuildReducerJob() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("BuildReducerJob() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildReducerJob() unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}

// containsString checks if s contains substr (case-insensitive).
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
