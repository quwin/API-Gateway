package limiter_test

import (
	"context"
	"testing"
	"time"

	"quwin/api-gateway/internal/limiter"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	s := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	t.Cleanup(func() {
		_ = client.Close()
		s.Close()
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping miniredis: %v", err)
	}

	return s, client
}

func TestFixedWindowRedisAllowsUntilPolicyLimitThenRejects(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := limiter.NewFixedWindowRedisLimiter(client)
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
	assertDecision(t, third, false, 0, 2, time.Minute)
}

func TestFixedWindowRedisAllowsAfterPolicyWindowExpires(t *testing.T) {
	s, client := newTestRedisClient(t)

	l := limiter.NewFixedWindowRedisLimiter(client)
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

	s.FastForward(time.Minute)

	allowed, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, allowed, true, 0, 1, 0)
}

func TestFixedWindowRedisTracksKeysIndependently(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := limiter.NewFixedWindowRedisLimiter(client)
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

func TestFixedWindowRedisReturnsContextError(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := limiter.NewFixedWindowRedisLimiter(client)
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

func TestTokenBucketRedisAllowsBurstThenRejects(t *testing.T) {
	_, client := newTestRedisClient(t)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewTokenBucketRedisLimiter(client)
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

func TestTokenBucketRedisRefillsOverTime(t *testing.T) {
	_, client := newTestRedisClient(t)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewTokenBucketRedisLimiter(client)
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

func TestTokenBucketRedisClampsRefillToPolicyCapacity(t *testing.T) {
	_, client := newTestRedisClient(t)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewTokenBucketRedisLimiter(client)
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

func TestTokenBucketRedisTracksKeysIndependently(t *testing.T) {
	_, client := newTestRedisClient(t)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewTokenBucketRedisLimiter(client)
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

func TestTokenBucketRedisReturnsContextError(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := limiter.NewTokenBucketRedisLimiter(client)
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

func TestTokenBucketRedisPanicsWithNilClient(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()

	limiter.NewTokenBucketRedisLimiter(nil)
}

func TestSlidingWindowRedisAllowsRequestsUnderPolicyLimit(t *testing.T) {
	_, client := newTestRedisClient(t)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowRedisLimiter(client, "test-instance")
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

func TestSlidingWindowRedisRejectsWhenPolicyLimitReached(t *testing.T) {
	_, client := newTestRedisClient(t)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowRedisLimiter(client, "test-instance")
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

func TestSlidingWindowRedisAllowsAfterOldestRequestExpires(t *testing.T) {
	_, client := newTestRedisClient(t)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowRedisLimiter(client, "test-instance")
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
		t.Fatalf("expected request to be rejected before oldest request expires, got %+v", rejected)
	}

	currentTime = time.Date(2026, 1, 1, 12, 1, 0, 1, time.UTC)

	allowed, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}

	assertDecision(t, allowed, true, 0, 2, 0)
}

func TestSlidingWindowRedisTracksKeysIndependently(t *testing.T) {
	_, client := newTestRedisClient(t)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowRedisLimiter(client, "test-instance")
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

func TestSlidingWindowRedisStoresDistinctRequestsAtSameTimestamp(t *testing.T) {
	_, client := newTestRedisClient(t)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	l := limiter.NewSlidingWindowRedisLimiter(client, "test-instance")
	l.Now = func() time.Time {
		return currentTime
	}

	policy := fixedPolicy(3, time.Minute)

	for i := 0; i < 3; i++ {
		d, err := l.Allow(context.Background(), "user-1", policy)
		if err != nil {
			t.Fatal(err)
		}
		if !d.Allowed {
			t.Fatalf("request %d: expected allowed, got %+v", i+1, d)
		}
	}

	rejected, err := l.Allow(context.Background(), "user-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Allowed {
		t.Fatalf("expected fourth request at same timestamp to be rejected, got %+v", rejected)
	}
}

func TestSlidingWindowRedisReturnsContextError(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := limiter.NewSlidingWindowRedisLimiter(client, "test-instance")
	policy := fixedPolicy(2, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d, err := l.Allow(ctx, "user-1", policy)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if d != (limiter.Decision{}) {
		t.Fatalf("expected zero-value decision, got %+v", d)
	}
}

func TestSlidingWindowRedisPanicsWithNilClient(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()

	limiter.NewSlidingWindowRedisLimiter(nil, "test-instance")
}