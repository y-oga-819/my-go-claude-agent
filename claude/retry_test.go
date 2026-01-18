package claude

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.InitialBackoff != 1*time.Second {
		t.Errorf("InitialBackoff = %v, want 1s", config.InitialBackoff)
	}
	if config.MaxBackoff != 30*time.Second {
		t.Errorf("MaxBackoff = %v, want 30s", config.MaxBackoff)
	}
	if config.BackoffFactor != 2.0 {
		t.Errorf("BackoffFactor = %v, want 2.0", config.BackoffFactor)
	}
	if !config.Jitter {
		t.Error("Jitter should be true")
	}
}

func TestWithRetry_Success(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
		Jitter:         false,
	}

	callCount := 0
	err := WithRetry(ctx, config, func(ctx context.Context) error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("WithRetry() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
		Jitter:         false,
	}

	callCount := 0
	err := WithRetry(ctx, config, func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return NewRetryableError("test", ErrRateLimit, 0)
		}
		return nil
	})

	if err != nil {
		t.Errorf("WithRetry() error = %v", err)
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
		Jitter:         false,
	}

	callCount := 0
	err := WithRetry(ctx, config, func(ctx context.Context) error {
		callCount++
		return NewRetryableError("test", ErrRateLimit, 0)
	})

	if err == nil {
		t.Error("WithRetry() should return error")
	}
	// MaxRetries=2 means initial + 2 retries = 3 calls
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
		Jitter:         false,
	}

	callCount := 0
	err := WithRetry(ctx, config, func(ctx context.Context) error {
		callCount++
		return ErrAuthentication // Not retryable
	})

	if !errors.Is(err, ErrAuthentication) {
		t.Errorf("WithRetry() error = %v, want ErrAuthentication", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (no retries for non-retryable error)", callCount)
	}
}

func TestWithRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := &RetryConfig{
		MaxRetries:     10,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         false,
	}

	callCount := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := WithRetry(ctx, config, func(ctx context.Context) error {
		callCount++
		return NewRetryableError("test", ErrRateLimit, 0)
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("WithRetry() error = %v, want context.Canceled", err)
	}
}

func TestWithRetry_NilConfig(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	err := WithRetry(ctx, nil, func(ctx context.Context) error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("WithRetry() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}

func TestWithRetry_RetryAfterRespected(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 1 * time.Second, // Long initial backoff
		MaxBackoff:     10 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         false,
	}

	start := time.Now()
	callCount := 0
	_ = WithRetry(ctx, config, func(ctx context.Context) error {
		callCount++
		if callCount < 2 {
			// Short RetryAfter should override long initial backoff
			return NewRetryableError("test", ErrRateLimit, 10*time.Millisecond)
		}
		return nil
	})

	elapsed := time.Since(start)
	// Should take roughly 10ms, not 1s
	if elapsed > 500*time.Millisecond {
		t.Errorf("WithRetry took %v, RetryAfter should have shortened the wait", elapsed)
	}
}

func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		attempt  int
		initial  time.Duration
		max      time.Duration
		factor   float64
		expected time.Duration
	}{
		{0, 1 * time.Second, 30 * time.Second, 2.0, 1 * time.Second},
		{1, 1 * time.Second, 30 * time.Second, 2.0, 2 * time.Second},
		{2, 1 * time.Second, 30 * time.Second, 2.0, 4 * time.Second},
		{5, 1 * time.Second, 30 * time.Second, 2.0, 30 * time.Second}, // Capped at max
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := ExponentialBackoff(tt.attempt, tt.initial, tt.max, tt.factor)
			if got != tt.expected {
				t.Errorf("ExponentialBackoff(%d) = %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}
}

func TestAddJitter(t *testing.T) {
	d := 1 * time.Second
	factor := 0.25

	// Run multiple times to ensure randomness is within bounds
	for i := 0; i < 100; i++ {
		result := AddJitter(d, factor)
		min := time.Duration(float64(d) * (1 - factor))
		max := time.Duration(float64(d) * (1 + factor))

		if result < min || result > max {
			t.Errorf("AddJitter() = %v, should be between %v and %v", result, min, max)
		}
	}
}
