// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRetryOnTransient_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryOnTransient_TransientThenSuccess(t *testing.T) {
	calls := 0
	serverErr := apierrors.NewInternalError(errors.New("internal"))
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		if calls < 3 {
			return serverErr
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error after retries, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryOnTransient_TooManyRequestsThenSuccess(t *testing.T) {
	calls := 0
	tooManyErr := apierrors.NewTooManyRequests("throttled", 1)
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		if calls == 1 {
			return tooManyErr
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error after retry, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestRetryOnTransient_PermanentError_NotFound(t *testing.T) {
	calls := 0
	notFoundErr := apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "test")
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		return notFoundErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for 404), got %d", calls)
	}
}

func TestRetryOnTransient_PermanentError_Forbidden(t *testing.T) {
	calls := 0
	forbiddenErr := apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "pods"}, "test", errors.New("forbidden"))
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		return forbiddenErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for 403), got %d", calls)
	}
}

func TestRetryOnTransient_PermanentError_BadRequest(t *testing.T) {
	calls := 0
	badReqErr := apierrors.NewBadRequest("bad request")
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		return badReqErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for 400), got %d", calls)
	}
}

func TestRetryOnTransient_MaxRetriesExhausted(t *testing.T) {
	calls := 0
	serverErr := apierrors.NewInternalError(errors.New("internal"))
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		return serverErr
	})
	if err == nil {
		t.Fatal("expected error after max retries, got nil")
	}
	// 1 initial + 3 retries = 4 total calls
	if calls != 4 {
		t.Fatalf("expected 4 calls (1 initial + 3 retries), got %d", calls)
	}
}

func TestRetryOnTransient_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	serverErr := apierrors.NewInternalError(errors.New("internal"))
	err := RetryOnTransient(ctx, "test-op", func() error {
		calls++
		if calls == 2 {
			cancel()
		}
		return serverErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should stop retrying after context cancellation
	if calls > 3 {
		t.Fatalf("expected at most 3 calls before context cancel takes effect, got %d", calls)
	}
}

func TestRetryOnTransient_ContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	calls := 0
	err := RetryOnTransient(ctx, "test-op", func() error {
		calls++
		return apierrors.NewInternalError(errors.New("internal"))
	})
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
	// Should attempt at most 1 call before noticing cancellation
	if calls > 1 {
		t.Fatalf("expected at most 1 call with pre-canceled context, got %d", calls)
	}
}

// temporaryNetError implements net.Error with Temporary() = true
type temporaryNetError struct{}

func (e *temporaryNetError) Error() string   { return "temporary network error" }
func (e *temporaryNetError) Timeout() bool   { return false }
func (e *temporaryNetError) Temporary() bool { return true }

// Ensure it satisfies net.Error
var _ net.Error = (*temporaryNetError)(nil)

func TestRetryOnTransient_TemporaryNetError(t *testing.T) {
	calls := 0
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		if calls == 1 {
			return &temporaryNetError{}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error after retry, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestRetryOnTransient_NonTransientNetError(t *testing.T) {
	calls := 0
	// A plain non-temporary error should not be retried
	plainErr := errors.New("connection refused permanently")
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		return plainErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for non-transient error), got %d", calls)
	}
}

func TestRetryOnTransient_WrappedTransientError(t *testing.T) {
	// Verify that fmt.Errorf-wrapped K8s errors are still detected as transient.
	// This is critical because call sites wrap errors: fmt.Errorf("failed to ...: %w", err)
	calls := 0
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		if calls < 3 {
			rawErr := apierrors.NewInternalError(errors.New("internal"))
			return fmt.Errorf("failed to create resource: %w", rawErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error after retries through wrapped error, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls (retry through wrapped error), got %d", calls)
	}
}

func TestRetryOnTransient_WrappedPermanentError(t *testing.T) {
	// Verify that fmt.Errorf-wrapped permanent errors are NOT retried.
	calls := 0
	err := RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		rawErr := apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "test")
		return fmt.Errorf("failed to get resource: %w", rawErr)
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for wrapped 404), got %d", calls)
	}
}

func TestRetryOnTransient_BackoffTiming(t *testing.T) {
	calls := 0
	serverErr := apierrors.NewInternalError(errors.New("internal"))
	start := time.Now()
	_ = RetryOnTransient(context.Background(), "test-op", func() error {
		calls++
		return serverErr
	})
	elapsed := time.Since(start)
	// With backoff of 100ms, 200ms, 400ms (capped at 2s) total ~700ms minimum
	// Be lenient: just check it took at least 200ms (first two waits)
	if elapsed < 200*time.Millisecond {
		t.Fatalf("expected at least 200ms for backoff, got %v", elapsed)
	}
	// Should not take more than 10s (generous upper bound)
	if elapsed > 10*time.Second {
		t.Fatalf("backoff took too long: %v", elapsed)
	}
}
