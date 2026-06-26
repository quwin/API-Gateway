package limiter

import (
	"context"
	"sync"
	"time"
)

type fixedWindowCounter struct {
	count   int64
	resetAt time.Time
}

type FixedWindowMemoryLimiter struct {
	mu       sync.Mutex
	counters map[string]fixedWindowCounter
	Now      func() time.Time
}

func NewFixedWindowMemoryLimiter() *FixedWindowMemoryLimiter {
	return &FixedWindowMemoryLimiter{
		counters: make(map[string]fixedWindowCounter),
		Now:      time.Now,
	}
}

func (limiter *FixedWindowMemoryLimiter) Allow(ctx context.Context, key string, policy Policy) (Decision, error) {
	if err := ctx.Err(); err != nil {
		return Decision{}, err
	}
	Now := limiter.Now()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	counter, exists := limiter.counters[key]

	if !exists || Now.After(counter.resetAt) {
		counter = fixedWindowCounter{
			count:   0,
			resetAt: Now.Add(policy.Window),
		}
	}

	if counter.count >= policy.Limit {
		limiter.counters[key] = counter

		return Decision{
			Allowed:    false,
			RetryAfter: counter.resetAt.Sub(Now),
			Remaining:  0,
			Limit:      policy.Limit,
		}, nil
	}

	counter.count++
	limiter.counters[key] = counter

	return Decision{
		Allowed:    true,
		RetryAfter: 0,
		Remaining:  policy.Limit - counter.count,
		Limit:      policy.Limit,
	}, nil
}
