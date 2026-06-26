package limiter_test

import (
	"context"
	"testing"
	"time"

	"quwin/api-gateway/internal/limiter"
)

func TestFixedWindowMemoryAllowsUntilPolicyLimitThenRejects(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewFixedWindowMemoryLimiter()
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

	third, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	if third.Allowed {
		t.Fatalf("expected third request to be rejected, got %+v", third)
	}
	if third.Remaining != 0 {
		t.Fatalf("expected 0 remaining, got %d", third.Remaining)
	}
	if third.Limit != 2 {
		t.Fatalf("expected limit 2, got %d", third.Limit)
	}
	if third.RetryAfter <= 0 || third.RetryAfter > time.Minute {
		t.Fatalf("expected RetryAfter between 0 and 1m, got %s", third.RetryAfter)
	}
}

func TestFixedWindowMemoryAllowsAfterPolicyWindowExpires(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewFixedWindowMemoryLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := fixedPolicy(1, time.Minute)

	first, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, first, true, 0, 1, 0)

	rejected, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Allowed {
		t.Fatalf("expected second request to be rejected, got %+v", rejected)
	}

	currentTime = currentTime.Add(time.Minute + time.Nanosecond)

	allowed, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, allowed, true, 0, 1, 0)
}

func TestFixedWindowMemoryTracksKeysIndependently(t *testing.T) {
	l := limiter.NewFixedWindowMemoryLimiter()
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

func TestFixedWindowMemoryUsesDifferentPoliciesPerRequest(t *testing.T) {
	l := limiter.NewFixedWindowMemoryLimiter()

	freePolicy := fixedPolicy(1, time.Minute)
	proPolicy := fixedPolicy(3, time.Minute)

	first, err := l.Allow(context.Background(), "user-1", freePolicy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, first, true, 0, 1, 0)

	rejectedByFreePolicy, err := l.Allow(context.Background(), "user-1", freePolicy)
	if err != nil {
		t.Fatal(err)
	}
	if rejectedByFreePolicy.Allowed {
		t.Fatalf("expected request to be rejected by free policy, got %+v", rejectedByFreePolicy)
	}

	// Same stored counter, but a larger policy should allow additional requests.
	allowedByProPolicy, err := l.Allow(context.Background(), "user-1", proPolicy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, allowedByProPolicy, true, 1, 3, 0)
}

func TestFixedWindowMemoryReturnsContextError(t *testing.T) {
	l := limiter.NewFixedWindowMemoryLimiter()
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