package limiter

import (
	"context"
	"time"
	"errors"
)

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	Remaining  int64
	Limit      int64
}

type Policy struct {
	Limit      int64
	Window     time.Duration
	Capacity   int64
	RefillRate float64
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, policy Policy) (Decision, error)
}

var errInvalidRedisScriptResult = errors.New("invalid redis script result")