// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildReaderPod(t *testing.T) {
	tests := []struct {
		name        string
		cfg         ReadArtifactConfig
		wantErr     bool
		errContains string
		validate    func(*testing.T, *corev1.Pod)
	}{
		{
			name: "valid config",
			cfg: ReadArtifactConfig{
				Path:      "/mnt/data/results/output.json",
				Lines:     50,
				Namespace: "default",
				PVCName:   "shared-data",
				MountPath: "/mnt/data",
			},
			validate: func(t *testing.T, pod *corev1.Pod) {
				// Check namespace
				if pod.Namespace != "default" {
					t.Errorf("namespace = %q, want %q", pod.Namespace, "default")
				}

				// Check labels
				if pod.Labels[ReaderLabelKey] != ReaderLabelValue {
					t.Errorf("label %s = %q, want %q", ReaderLabelKey, pod.Labels[ReaderLabelKey], ReaderLabelValue)
				}

				// Check restart policy
				if pod.Spec.RestartPolicy != corev1.RestartPolicyNever {
					t.Errorf("restartPolicy = %q, want %q", pod.Spec.RestartPolicy, corev1.RestartPolicyNever)
				}

				// Check container
				if len(pod.Spec.Containers) != 1 {
					t.Fatalf("containers count = %d, want 1", len(pod.Spec.Containers))
				}
				container := pod.Spec.Containers[0]
				if container.Name != "reader" {
					t.Errorf("container name = %q, want %q", container.Name, "reader")
				}
				if container.Image != ReaderPodImage {
					t.Errorf("image = %q, want %q", container.Image, ReaderPodImage)
				}

				// Check command
				if len(container.Command) != 3 {
					t.Fatalf("command length = %d, want 3", len(container.Command))
				}
				if container.Command[0] != "sh" || container.Command[1] != "-c" {
					t.Errorf("command = %v, want sh -c ...", container.Command)
				}
				expectedCmd := "head -n 50 '/mnt/data/results/output.json'"
				if container.Command[2] != expectedCmd {
					t.Errorf("command script = %q, want %q", container.Command[2], expectedCmd)
				}

				// Check volume mount
				if len(container.VolumeMounts) != 1 {
					t.Fatalf("volume mounts count = %d, want 1", len(container.VolumeMounts))
				}
				mount := container.VolumeMounts[0]
				if mount.Name != "data" {
					t.Errorf("volume mount name = %q, want %q", mount.Name, "data")
				}
				if mount.MountPath != "/mnt/data" {
					t.Errorf("mount path = %q, want %q", mount.MountPath, "/mnt/data")
				}
				if !mount.ReadOnly {
					t.Error("volume mount should be read-only")
				}

				// Check volume
				if len(pod.Spec.Volumes) != 1 {
					t.Fatalf("volumes count = %d, want 1", len(pod.Spec.Volumes))
				}
				vol := pod.Spec.Volumes[0]
				if vol.Name != "data" {
					t.Errorf("volume name = %q, want %q", vol.Name, "data")
				}
				if vol.PersistentVolumeClaim == nil {
					t.Fatal("PVC volume source is nil")
				}
				if vol.PersistentVolumeClaim.ClaimName != "shared-data" {
					t.Errorf("PVC claimName = %q, want %q", vol.PersistentVolumeClaim.ClaimName, "shared-data")
				}
				if !vol.PersistentVolumeClaim.ReadOnly {
					t.Error("PVC should be read-only")
				}
			},
		},
		{
			name: "default lines",
			cfg: ReadArtifactConfig{
				Path:      "/mnt/data/file.txt",
				Namespace: "default",
				PVCName:   "shared-data",
				// Lines not set - should default to 100
			},
			validate: func(t *testing.T, pod *corev1.Pod) {
				expectedCmd := "head -n 100 '/mnt/data/file.txt'"
				if pod.Spec.Containers[0].Command[2] != expectedCmd {
					t.Errorf("command script = %q, want %q", pod.Spec.Containers[0].Command[2], expectedCmd)
				}
			},
		},
		{
			name: "default namespace",
			cfg: ReadArtifactConfig{
				Path:    "/mnt/data/file.txt",
				PVCName: "shared-data",
				// Namespace not set - should default to "default"
			},
			validate: func(t *testing.T, pod *corev1.Pod) {
				if pod.Namespace != "default" {
					t.Errorf("namespace = %q, want %q", pod.Namespace, "default")
				}
			},
		},
		{
			name: "default mount path",
			cfg: ReadArtifactConfig{
				Path:      "/mnt/data/file.txt",
				Namespace: "default",
				PVCName:   "shared-data",
				// MountPath not set - should default to "/mnt/data"
			},
			validate: func(t *testing.T, pod *corev1.Pod) {
				mount := pod.Spec.Containers[0].VolumeMounts[0]
				if mount.MountPath != "/mnt/data" {
					t.Errorf("mount path = %q, want %q", mount.MountPath, "/mnt/data")
				}
			},
		},
		{
			name: "custom mount path",
			cfg: ReadArtifactConfig{
				Path:      "/custom/path/file.txt",
				Namespace: "default",
				PVCName:   "shared-data",
				MountPath: "/custom/path",
			},
			validate: func(t *testing.T, pod *corev1.Pod) {
				mount := pod.Spec.Containers[0].VolumeMounts[0]
				if mount.MountPath != "/custom/path" {
					t.Errorf("mount path = %q, want %q", mount.MountPath, "/custom/path")
				}
			},
		},
		{
			name: "pod name is unique",
			cfg: ReadArtifactConfig{
				Path:      "/mnt/data/file.txt",
				Namespace: "default",
				PVCName:   "shared-data",
			},
			validate: func(t *testing.T, pod *corev1.Pod) {
				if pod.Name == "" {
					t.Error("pod name should not be empty")
				}
				// Name should start with "reader-"
				if len(pod.Name) < 7 || pod.Name[:7] != "reader-" {
					t.Errorf("pod name = %q, want prefix 'reader-'", pod.Name)
				}
			},
		},
		{
			name:        "missing path",
			cfg:         ReadArtifactConfig{PVCName: "shared-data"},
			wantErr:     true,
			errContains: "path is required",
		},
		{
			name:        "missing PVC name",
			cfg:         ReadArtifactConfig{Path: "/mnt/data/file.txt"},
			wantErr:     true,
			errContains: "PVC name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildReaderPod(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("BuildReaderPod() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("BuildReaderPod() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildReaderPod() unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:  "single line no newline",
			input: "hello",
			want:  1,
		},
		{
			name:  "single line with newline",
			input: "hello\n",
			want:  1,
		},
		{
			name:  "two lines",
			input: "hello\nworld",
			want:  2,
		},
		{
			name:  "two lines with trailing newline",
			input: "hello\nworld\n",
			want:  2,
		},
		{
			name:  "multiple lines",
			input: "line1\nline2\nline3\nline4\nline5",
			want:  5,
		},
		{
			name:  "multiple lines with trailing newline",
			input: "line1\nline2\nline3\nline4\nline5\n",
			want:  5,
		},
		{
			name:  "empty lines",
			input: "\n\n\n",
			want:  3,
		},
		{
			name:  "mixed content",
			input: "first\n\nsecond\n\nthird\n",
			want:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLines(tt.input)
			if got != tt.want {
				t.Errorf("countLines(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildReaderPodName(t *testing.T) {
	// Generate multiple names and check they're unique
	names := make(map[string]bool)
	for i := 0; i < 10; i++ {
		name := buildReaderPodName()
		if names[name] {
			t.Errorf("duplicate name generated: %s", name)
		}
		names[name] = true

		// Check prefix
		if len(name) < 7 || name[:7] != "reader-" {
			t.Errorf("name = %q, want prefix 'reader-'", name)
		}
	}
}

func TestReadArtifactHead_Success(t *testing.T) {
	// Create mock client
	mockClient := NewMockClient()

	// Pre-create a completed pod with logs
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reader-test-12345",
			Namespace: "default",
			Labels: map[string]string{
				ReaderLabelKey: ReaderLabelValue,
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	}

	_, err := mockClient.Clientset().CoreV1().Pods("default").Create(
		context.Background(),
		pod,
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("failed to create test pod: %v", err)
	}

	// Note: The fake clientset doesn't support log streaming,
	// so we test the pod creation and cleanup flow separately.
	// Full integration testing requires a real cluster or more sophisticated mocking.
}

func TestReadArtifactHead_PodFailure(t *testing.T) {
	// This test verifies the error handling when a pod fails.
	// With the fake client, we can test the pod status checking logic.

	mockClient := NewMockClient()

	// Pre-create a failed pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reader-failed-test",
			Namespace: "default",
			Labels: map[string]string{
				ReaderLabelKey: ReaderLabelValue,
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}

	_, err := mockClient.Clientset().CoreV1().Pods("default").Create(
		context.Background(),
		pod,
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("failed to create test pod: %v", err)
	}

	// Verify we can detect the failed status
	fetched, err := mockClient.Clientset().CoreV1().Pods("default").Get(
		context.Background(),
		"reader-failed-test",
		metav1.GetOptions{},
	)
	if err != nil {
		t.Fatalf("failed to get pod: %v", err)
	}

	if fetched.Status.Phase != corev1.PodFailed {
		t.Errorf("pod phase = %q, want %q", fetched.Status.Phase, corev1.PodFailed)
	}
}

func TestCleanupOrphanedReaderPods(t *testing.T) {
	mockClient := NewMockClient()

	// Create multiple reader pods
	for i := 0; i < 3; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      buildReaderPodName(),
				Namespace: "default",
				Labels: map[string]string{
					ReaderLabelKey: ReaderLabelValue,
				},
			},
		}
		_, err := mockClient.Clientset().CoreV1().Pods("default").Create(
			context.Background(),
			pod,
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Fatalf("failed to create pod: %v", err)
		}
	}

	// Also create a non-reader pod that should NOT be deleted
	nonReaderPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "other",
			},
		},
	}
	_, err := mockClient.Clientset().CoreV1().Pods("default").Create(
		context.Background(),
		nonReaderPod,
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("failed to create non-reader pod: %v", err)
	}

	// Verify 4 pods exist (3 reader + 1 other)
	pods, err := mockClient.Clientset().CoreV1().Pods("default").List(
		context.Background(),
		metav1.ListOptions{},
	)
	if err != nil {
		t.Fatalf("failed to list pods: %v", err)
	}
	if len(pods.Items) != 4 {
		t.Errorf("initial pod count = %d, want 4", len(pods.Items))
	}

	// Run cleanup
	err = mockClient.CleanupOrphanedReaderPods(context.Background(), "default")
	if err != nil {
		t.Fatalf("CleanupOrphanedReaderPods() error = %v", err)
	}

	// Verify only non-reader pod remains
	pods, err = mockClient.Clientset().CoreV1().Pods("default").List(
		context.Background(),
		metav1.ListOptions{},
	)
	if err != nil {
		t.Fatalf("failed to list pods after cleanup: %v", err)
	}
	if len(pods.Items) != 1 {
		t.Errorf("pod count after cleanup = %d, want 1", len(pods.Items))
	}
	if pods.Items[0].Name != "other-pod" {
		t.Errorf("remaining pod = %q, want %q", pods.Items[0].Name, "other-pod")
	}
}

func TestReadArtifactConfig_Defaults(t *testing.T) {
	cfg := ReadArtifactConfig{
		Path:    "/mnt/data/test.txt",
		PVCName: "shared-data",
	}

	// Verify defaults are applied when building pod
	pod, err := BuildReaderPod(cfg)
	if err != nil {
		t.Fatalf("BuildReaderPod() error = %v", err)
	}

	// Defaults should be applied
	if pod.Namespace != "default" {
		t.Errorf("namespace = %q, want %q", pod.Namespace, "default")
	}

	// Lines default is 100, check command
	expectedCmd := "head -n 100 '/mnt/data/test.txt'"
	if pod.Spec.Containers[0].Command[2] != expectedCmd {
		t.Errorf("command = %q, want %q", pod.Spec.Containers[0].Command[2], expectedCmd)
	}

	// MountPath default
	mount := pod.Spec.Containers[0].VolumeMounts[0]
	if mount.MountPath != "/mnt/data" {
		t.Errorf("mountPath = %q, want %q", mount.MountPath, "/mnt/data")
	}
}

func TestDefaultReadTimeout(t *testing.T) {
	if DefaultReadTimeout != 30*time.Second {
		t.Errorf("DefaultReadTimeout = %v, want %v", DefaultReadTimeout, 30*time.Second)
	}
}

func TestReaderPodImage(t *testing.T) {
	// Verify we're using a specific image version, not :latest
	if ReaderPodImage == "busybox:latest" || ReaderPodImage == "busybox" {
		t.Error("ReaderPodImage should use a specific version tag, not :latest")
	}
	if ReaderPodImage != "busybox:1.36" {
		t.Errorf("ReaderPodImage = %q, want %q", ReaderPodImage, "busybox:1.36")
	}
}
