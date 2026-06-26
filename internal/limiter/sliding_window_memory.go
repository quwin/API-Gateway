package limiter

import (
	"context"
	"sync"
	"time"
)

type SlidingWindowLogLimiter struct {
	mu       sync.Mutex
	Requests map[string][]time.Time
	Now      func() time.Time
}

func NewSlidingWindowLogLimiter() *SlidingWindowLogLimiter {
	return &SlidingWindowLogLimiter{
		Requests: make(map[string][]time.Time),
		Now:      time.Now,
	}
}

func (l *SlidingWindowLogLimiter) Allow(ctx context.Context, key string, policy Policy) (Decision, error) {
	if err := ctx.Err(); err != nil {
		return Decision{}, err
	}

	Now := l.Now()
	windowStart := Now.Add(-policy.Window)

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamps := l.Requests[key]

	// Drop Requests that are outside the current sliding window.
	firstValidIndex := 0
	for firstValidIndex < len(timestamps) && !timestamps[firstValidIndex].After(windowStart) {
		firstValidIndex++
	}

	timestamps = timestamps[firstValidIndex:]

	if int64(len(timestamps)) >= policy.Limit {
		oldestRequest := timestamps[0]
		retryAfter := max(oldestRequest.Add(policy.Window).Sub(Now), 0)

		l.Requests[key] = timestamps

		return Decision{
			Allowed:    false,
			RetryAfter: retryAfter,
			Remaining:  0,
			Limit:      policy.Limit,
		}, nil
	}

	timestamps = append(timestamps, Now)
	l.Requests[key] = timestamps

	return Decision{
		Allowed:    true,
		RetryAfter: 0,
		Remaining:  policy.Limit - int64(len(timestamps)),
		Limit:      policy.Limit,
	}, nil
}
