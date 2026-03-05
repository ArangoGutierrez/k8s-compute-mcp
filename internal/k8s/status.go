// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// extractConditionPhase extracts the phase from a Kubernetes object's
// status.conditions array. It returns the type of the last condition
// with status "True". If no conditions are found, it returns "Pending".
//
// This handles the standard Kubernetes condition pattern used by MPIJob,
// JobSet, and similar CRDs where status.conditions is a slice of objects
// with type, status, reason, and message fields.
func extractConditionPhase(obj map[string]interface{}) (string, error) {
	conditions, found, err := unstructured.NestedSlice(obj, "status", "conditions")
	if err != nil {
		return "", err
	}
	if !found || len(conditions) == 0 {
		return "Pending", nil
	}

	// Find the last condition with status "True"
	for i := len(conditions) - 1; i >= 0; i-- {
		condition, ok := conditions[i].(map[string]interface{})
		if !ok {
			continue
		}

		status, found, err := unstructured.NestedString(condition, "status")
		if err != nil {
			return "", fmt.Errorf("malformed condition at index %d: status field: %w", i, err)
		}
		if !found || status != "True" {
			continue
		}

		condType, found, err := unstructured.NestedString(condition, "type")
		if err != nil {
			return "", fmt.Errorf("malformed condition at index %d: type field: %w", i, err)
		}
		if !found || condType == "" {
			continue
		}
		return condType, nil
	}

	// No condition with status "True" found
	return "Pending", nil
}
