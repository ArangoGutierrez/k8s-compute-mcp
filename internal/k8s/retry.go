// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
)

const (
	// retryMaxAttempts is the maximum number of retry attempts (not counting the initial call).
	retryMaxAttempts = 3

	// retryInitialBackoff is the initial backoff duration.
	retryInitialBackoff = 100 * time.Millisecond

	// retryBackoffFactor is the multiplier applied to backoff each retry.
	retryBackoffFactor = 2.0

	// retryMaxBackoff is the maximum backoff duration.
	retryMaxBackoff = 2 * time.Second

	// retryJitterFraction is the fraction of the backoff to add as jitter.
	retryJitterFraction = 0.1
)

// RetryOnTransient retries the given function on transient K8s API errors.
// It uses exponential backoff: 100ms, 200ms, 400ms (capped at 2s), with 10% jitter.
// Max 3 retries (4 total attempts).
//
// Retries on: 429 TooManyRequests, 5xx Server Errors, net.Error (temporary).
// Does NOT retry: 400, 401, 403, 404, 409 (client errors), or non-transient errors.
func RetryOnTransient(ctx context.Context, name string, fn func() error) error {
	var lastErr error
	backoff := retryInitialBackoff

	for attempt := 0; attempt <= retryMaxAttempts; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Don't retry if this was the last attempt
		if attempt == retryMaxAttempts {
			break
		}

		if !isTransient(lastErr) {
			return lastErr
		}

		klog.InfoS("Retrying K8s API call", "operation", name, "attempt", attempt+1, "error", lastErr)

		// Apply jitter: backoff +/- jitterFraction
		jitter := time.Duration(float64(backoff) * retryJitterFraction * (2*rand.Float64() - 1)) //nolint:gosec // G404: math/rand is appropriate for non-security retry jitter
		sleep := backoff + jitter

		select {
		case <-ctx.Done():
			return lastErr
		case <-time.After(sleep):
		}

		// Increase backoff for next iteration
		backoff = time.Duration(float64(backoff) * retryBackoffFactor)
		if backoff > retryMaxBackoff {
			backoff = retryMaxBackoff
		}
	}

	return lastErr
}

// isTransient returns true if the error is a transient K8s API error worth retrying.
func isTransient(err error) bool {
	// Check for K8s API status errors
	if apierrors.IsServerTimeout(err) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err) {
		return true
	}

	// Check for 5xx via status code (errors.As unwraps through fmt.Errorf %w)
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		code := statusErr.Status().Code
		if code >= 500 {
			return true
		}
	}

	// Check for temporary network errors (errors.As unwraps through fmt.Errorf %w)
	var netErr net.Error
	if errors.As(err, &netErr) {
		//nolint:staticcheck // Temporary() is deprecated but still relevant for K8s client errors
		return netErr.Temporary()
	}

	return false
}
