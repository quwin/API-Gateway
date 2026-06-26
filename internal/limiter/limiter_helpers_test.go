package limiter_test

import (
	"testing"
	"time"

	"quwin/api-gateway/internal/limiter"
)

func fixedPolicy(limit int64, window time.Duration) limiter.Policy {
	return limiter.Policy{
		Limit:      limit,
		Window:     window,
		Capacity:   limit,
		RefillRate: float64(limit) / window.Seconds(),
	}
}

func tokenBucketPolicy(capacity int64, refillRate float64) limiter.Policy {
	return limiter.Policy{
		Limit:      capacity,
		Window:     time.Duration(float64(time.Second) * float64(capacity) / refillRate),
		Capacity:   capacity,
		RefillRate: refillRate,
	}
}

func assertDecision(
	t *testing.T,
	got limiter.Decision,
	allowed bool,
	remaining int64,
	limit int64,
	retryAfter time.Duration,
) {
	t.Helper()

	if got.Allowed != allowed {
		t.Fatalf("Allowed: expected %v, got %v; decision=%+v", allowed, got.Allowed, got)
	}

	if got.Remaining != remaining {
		t.Fatalf("Remaining: expected %d, got %d; decision=%+v", remaining, got.Remaining, got)
	}

	if got.Limit != limit {
		t.Fatalf("Limit: expected %d, got %d; decision=%+v", limit, got.Limit, got)
	}

	if got.RetryAfter != retryAfter {
		t.Fatalf("RetryAfter: expected %s, got %s; decision=%+v", retryAfter, got.RetryAfter, got)
	}
}