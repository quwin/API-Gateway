package limiter_test

import (
	"context"
	"testing"
	"time"

	"quwin/api-gateway/internal/limiter"
)

func TestTokenBucketMemoryAllowsBurstThenRejects(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewTokenBucketMemoryLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := tokenBucketPolicy(2, 1)

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
	assertDecision(t, third, false, 0, 2, time.Second)
}

func TestTokenBucketMemoryRefillsOverTime(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewTokenBucketMemoryLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := tokenBucketPolicy(2, 1)

	_, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	_, err = l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	currentTime = currentTime.Add(time.Second)

	allowed, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, allowed, true, 0, 2, 0)
}

func TestTokenBucketMemoryClampsRefillToCapacity(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewTokenBucketMemoryLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := tokenBucketPolicy(2, 1)

	_, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	currentTime = currentTime.Add(10 * time.Second)

	decision, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	assertDecision(t, decision, true, 1, 2, 0)
}

func TestTokenBucketMemoryTracksKeysIndependently(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewTokenBucketMemoryLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	policy := tokenBucketPolicy(1, 1)

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

func TestTokenBucketMemoryUsesPolicyCapacityAndRefillRate(t *testing.T) {
	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewTokenBucketMemoryLimiter()
	l.Now = func() time.Time {
		return currentTime
	}

	freePolicy := tokenBucketPolicy(1, 1)
	proPolicy := tokenBucketPolicy(3, 1)

	first, err := l.Allow(context.Background(), "user-1", freePolicy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, first, true, 0, 1, 0)

	rejectedByFreePolicy, err := l.Allow(context.Background(), "user-1", freePolicy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, rejectedByFreePolicy, false, 0, 1, time.Second)

	// Because the bucket was created with free capacity 1, changing to pro
	// does not magically refill to 3. It only changes future max capacity/refill behavior.
	currentTime = currentTime.Add(time.Second)

	allowedByProPolicy, err := l.Allow(context.Background(), "user-1", proPolicy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, allowedByProPolicy, true, 0, 3, 0)
}

func TestTokenBucketMemoryReturnsContextError(t *testing.T) {
	l := limiter.NewTokenBucketMemoryLimiter()
	policy := tokenBucketPolicy(2, 1)

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