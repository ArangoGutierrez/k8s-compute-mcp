// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// newFakeDynamicClientWithResources creates a fake dynamic client that
// recognizes the given GVR and pre-populates it with the given objects.
func newFakeDynamicClientWithResources(gvr schema.GroupVersionResource, objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "MPIJob"},
		&unstructured.Unstructured{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "MPIJobList"},
		&unstructured.UnstructuredList{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
		&unstructured.Unstructured{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSetList"},
		&unstructured.UnstructuredList{},
	)
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			MPIJobGVR: "MPIJobList",
			JobSetGVR: "JobSetList",
		},
		objects...)
}

func TestGetMPIJobStatus_ConditionsExtraction(t *testing.T) {
	tests := []struct {
		name       string
		conditions []interface{}
		wantPhase  string
		wantErr    bool
	}{
		{
			name: "running condition",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Running",
					"status": "True",
					"reason": "MPIJobRunning",
				},
			},
			wantPhase: "Running",
		},
		{
			name: "succeeded condition",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Running",
					"status": "False",
					"reason": "MPIJobCompleted",
				},
				map[string]interface{}{
					"type":   "Succeeded",
					"status": "True",
					"reason": "MPIJobSucceeded",
				},
			},
			wantPhase: "Succeeded",
		},
		{
			name: "failed condition",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Running",
					"status": "False",
					"reason": "MPIJobFailed",
				},
				map[string]interface{}{
					"type":   "Failed",
					"status": "True",
					"reason": "MPIJobFailed",
				},
			},
			wantPhase: "Failed",
		},
		{
			name: "multiple conditions picks last true",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Created",
					"status": "True",
					"reason": "MPIJobCreated",
				},
				map[string]interface{}{
					"type":   "Running",
					"status": "True",
					"reason": "MPIJobRunning",
				},
			},
			wantPhase: "Running",
		},
		{
			name:       "no conditions returns Pending",
			conditions: nil,
			wantPhase:  "Pending",
		},
		{
			name:       "empty conditions returns Pending",
			conditions: []interface{}{},
			wantPhase:  "Pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubeflow.org/v2beta1",
					"kind":       "MPIJob",
					"metadata": map[string]interface{}{
						"name":      "test-mpijob",
						"namespace": "default",
					},
					"status": map[string]interface{}{},
				},
			}
			if tt.conditions != nil {
				obj.Object["status"].(map[string]interface{})["conditions"] = tt.conditions
			}

			fakeDynamic := newFakeDynamicClientWithResources(MPIJobGVR, obj)
			client := &Client{
				dynamicClient: fakeDynamic,
				namespace:     "default",
			}

			phase, err := client.GetMPIJobStatus(context.Background(), "default", "test-mpijob")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q", phase, tt.wantPhase)
			}
		})
	}
}

func TestGetJobSetStatus_ConditionsExtraction(t *testing.T) {
	tests := []struct {
		name       string
		conditions []interface{}
		wantPhase  string
		wantErr    bool
	}{
		{
			name: "completed condition",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Completed",
					"status": "True",
					"reason": "AllJobsCompleted",
				},
			},
			wantPhase: "Completed",
		},
		{
			name: "failed condition",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Failed",
					"status": "True",
					"reason": "JobSetFailed",
				},
			},
			wantPhase: "Failed",
		},
		{
			name: "suspended condition",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Suspended",
					"status": "True",
					"reason": "JobSetSuspended",
				},
			},
			wantPhase: "Suspended",
		},
		{
			name:       "no conditions returns Pending",
			conditions: nil,
			wantPhase:  "Pending",
		},
		{
			name:       "empty conditions returns Pending",
			conditions: []interface{}{},
			wantPhase:  "Pending",
		},
		{
			name: "multiple conditions picks last true",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Suspended",
					"status": "False",
					"reason": "JobSetResumed",
				},
				map[string]interface{}{
					"type":   "Completed",
					"status": "True",
					"reason": "AllJobsCompleted",
				},
			},
			wantPhase: "Completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "jobset.x-k8s.io/v1alpha2",
					"kind":       "JobSet",
					"metadata": map[string]interface{}{
						"name":      "test-jobset",
						"namespace": "default",
					},
					"status": map[string]interface{}{},
				},
			}
			if tt.conditions != nil {
				obj.Object["status"].(map[string]interface{})["conditions"] = tt.conditions
			}

			fakeDynamic := newFakeDynamicClientWithResources(JobSetGVR, obj)
			client := &Client{
				dynamicClient: fakeDynamic,
				namespace:     "default",
			}

			phase, err := client.GetJobSetStatus(context.Background(), "default", "test-jobset")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q", phase, tt.wantPhase)
			}
		})
	}
}

func TestGetMPIJobStatus_NotFound(t *testing.T) {
	fakeDynamic := newFakeDynamicClientWithResources(MPIJobGVR)
	client := &Client{
		dynamicClient: fakeDynamic,
		namespace:     "default",
	}

	_, err := client.GetMPIJobStatus(context.Background(), "default", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent MPIJob, got nil")
	}
}

func TestGetJobSetStatus_NotFound(t *testing.T) {
	fakeDynamic := newFakeDynamicClientWithResources(JobSetGVR)
	client := &Client{
		dynamicClient: fakeDynamic,
		namespace:     "default",
	}

	_, err := client.GetJobSetStatus(context.Background(), "default", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent JobSet, got nil")
	}
}

func TestGetMPIJobStatus_Timeout(t *testing.T) {
	// A canceled context should cause the Get call to fail
	fakeDynamic := newFakeDynamicClientWithResources(MPIJobGVR)
	client := &Client{
		dynamicClient: fakeDynamic,
		namespace:     "default",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	// Let the timeout expire
	time.Sleep(5 * time.Millisecond)

	_, err := client.GetMPIJobStatus(ctx, "default", "test-mpijob")
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}

func TestGetJobSetStatus_Timeout(t *testing.T) {
	fakeDynamic := newFakeDynamicClientWithResources(JobSetGVR)
	client := &Client{
		dynamicClient: fakeDynamic,
		namespace:     "default",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	_, err := client.GetJobSetStatus(ctx, "default", "test-jobset")
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}
