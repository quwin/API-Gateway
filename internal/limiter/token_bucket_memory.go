package limiter

import (
	"context"
	"math"
	"sync"
	"time"
)

type tokenBucket struct {
	tokens       float64
	lastRefillAt time.Time
}

type TokenBucketMemoryLimiter struct {
	mu         sync.Mutex
	buckets    map[string]tokenBucket
	Now        func() time.Time
}

// NewTokenBucketMemoryLimiter creates an in-memory token bucket limiter.
//
// capacity: maximum burst size.
// refillRate: tokens added per second.
//
// Example:
//   capacity = 10
//   refillRate = 2
//
// This allows bursts of up to 10 requests and refills at 2 requests/sec.
func NewTokenBucketMemoryLimiter() *TokenBucketMemoryLimiter {
	return &TokenBucketMemoryLimiter{
		buckets:    make(map[string]tokenBucket),
		Now:        time.Now,
	}
}

func (l *TokenBucketMemoryLimiter) Allow(ctx context.Context, key string, policy Policy) (Decision, error) {
	if err := ctx.Err(); err != nil {
		return Decision{}, err
	}

	Now := l.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, exists := l.buckets[key]
	if !exists {
		bucket = tokenBucket{
			tokens:       float64(policy.Capacity),
			lastRefillAt: Now,
		}
	}

	elapsed := Now.Sub(bucket.lastRefillAt).Seconds()
	tokensToAdd := elapsed * policy.RefillRate

	bucket.tokens = math.Min(float64(policy.Capacity), bucket.tokens+tokensToAdd)
	bucket.lastRefillAt = Now

	if bucket.tokens < 1 {
		tokensNeeded := 1 - bucket.tokens
		secondsUntilNextToken := tokensNeeded / policy.RefillRate
		retryAfter := time.Duration(math.Ceil(secondsUntilNextToken)) * time.Second

		l.buckets[key] = bucket

		return Decision{
			Allowed:    false,
			RetryAfter: retryAfter,
			Remaining:  int64(math.Floor(bucket.tokens)),
			Limit:      int64(policy.Capacity),
		}, nil
	}

	bucket.tokens -= 1
	l.buckets[key] = bucket

	return Decision{
		Allowed:    true,
		RetryAfter: 0,
		Remaining:  int64(math.Floor(bucket.tokens)),
		Limit:      int64(policy.Capacity),
	}, nil
}