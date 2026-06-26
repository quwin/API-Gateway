package limiter_test

import (
	"context"
	"testing"
	"time"

	"quwin/api-gateway/internal/limiter"
)

func TestSlidingWindowMemoryAllowsRequestsUnderPolicyLimit(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowLogLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := fixedPolicy(2, time.Minute)

	first, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, first, true, 1, 2, 0)

	second, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, second, true, 0, 2, 0)
}

func TestSlidingWindowMemoryRejectsWhenPolicyLimitReached(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowLogLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := fixedPolicy(2, time.Minute)

	_, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	currentTime = currentTime.Add(10 * time.Second)

	_, err = l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	currentTime = currentTime.Add(10 * time.Second)

	decision, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	assertDecision(t, decision, false, 0, 2, 40*time.Second)
}

func TestSlidingWindowMemoryAllowsAfterOldestRequestExpires(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowLogLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := fixedPolicy(2, time.Minute)

	_, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	currentTime = currentTime.Add(10 * time.Second)

	_, err = l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	currentTime = currentTime.Add(10 * time.Second)

	rejected, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Allowed {
		t.Fatalf("expected request to be rejected, got %+v", rejected)
	}

	currentTime = time.Date(2026, 1, 1, 12, 1, 0, 1, time.UTC)

	allowed, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, allowed, true, 0, 2, 0)
}

func TestSlidingWindowMemoryDoesNotResetAtFixedBoundary(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 30, 0, time.UTC)

	l := limiter.NewSlidingWindowLogLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := fixedPolicy(2, time.Minute)

	_, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	currentTime = time.Date(2026, 1, 1, 12, 0, 50, 0, time.UTC)

	_, err = l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	currentTime = time.Date(2026, 1, 1, 12, 1, 5, 0, time.UTC)

	decision, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	assertDecision(t, decision, false, 0, 2, 25*time.Second)
}

func TestSlidingWindowMemoryTracksKeysIndependently(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowLogLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := fixedPolicy(1, time.Minute)

	userOneFirst, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, userOneFirst, true, 0, 1, 0)

	userOneSecond, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	if userOneSecond.Allowed {
		t.Fatalf("expected user-1 second request to be rejected, got %+v", userOneSecond)
	}

	userTwoFirst, err := l.Allow(context.Background(), "user-2", policy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, userTwoFirst, true, 0, 1, 0)
}

func TestSlidingWindowMemoryPrunesExpiredRequests(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowLogLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := fixedPolicy(3, time.Minute)

	for i := 0; i < 3; i++ {
		_, err := l.Allow(context.Background(), "user-1", policy)
		if err != nil {
			t.Fatal(err)
		}
		currentTime = currentTime.Add(10 * time.Second)
	}

	if got := len(l.Requests["user-1"]); got != 3 {
		t.Fatalf("expected 3 stored timestamps, got %d", got)
	}

	currentTime = currentTime.Add(2 * time.Minute)

	decision, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	if !decision.Allowed {
		t.Fatalf("expected request to be allowed after old requests expired, got %+v", decision)
	}

	if got := len(l.Requests["user-1"]); got != 1 {
		t.Fatalf("expected only 1 stored timestamp after pruning, got %d", got)
	}
}

func TestSlidingWindowMemoryReturnsContextError(t *testing.T) {
	l := limiter.NewSlidingWindowLogLimiter()
	policy := fixedPolicy(2, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	decision, err := l.Allow(ctx, "user-1", policy)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if decision != (limiter.Decision{}) {
		t.Fatalf("expected zero-value decision, got %+v", decision)
	}
}