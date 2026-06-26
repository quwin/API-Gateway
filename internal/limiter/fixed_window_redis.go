package limiter

import (
	"context"
	_ "embed"
	"time"

	"github.com/redis/go-redis/v9"
)

type FixedWindowRedisLimiter struct {
	client *redis.Client
	Now    func() time.Time
}

func NewFixedWindowRedisLimiter(client *redis.Client) *FixedWindowRedisLimiter {
	return &FixedWindowRedisLimiter{
		client: client,
		Now:    time.Now,
	}
}

var fixedWindowScript = redis.NewScript(`
	local key = KEYS[1]
	local limit = tonumber(ARGV[1])
	local window_seconds = tonumber(ARGV[2])

	local current = redis.call("INCR", key)

	if current == 1 then
		redis.call("EXPIRE", key, window_seconds)
	end

	local ttl = redis.call("TTL", key)

	if current > limit then
		return {0, limit - current, ttl}
	end

	return {1, limit - current, ttl}
`)

func (l *FixedWindowRedisLimiter) Allow(ctx context.Context, key string, policy Policy) (Decision, error) {
	if err := ctx.Err(); err != nil {
		return Decision{}, err
	}

	redisKey := "ratelimit:fixed:" + key

	result, err := fixedWindowScript.Run(
		ctx,
		l.client,
		[]string{redisKey},
		policy.Limit,
		int64(policy.Window.Seconds()),
	).Result()
	if err != nil {
		return Decision{}, err
	}

	values, ok := result.([]any)
	if !ok || len(values) != 3 {
		return Decision{}, errInvalidRedisScriptResult
	}

	allowedInt, ok := values[0].(int64)
	if !ok {
		return Decision{}, errInvalidRedisScriptResult
	}

	remaining, ok := values[1].(int64)
	if !ok {
		return Decision{}, errInvalidRedisScriptResult
	}

	ttlSeconds, ok := values[2].(int64)
	if !ok {
		return Decision{}, errInvalidRedisScriptResult
	}

	if remaining < 0 {
		remaining = 0
	}

	retryAfter := time.Duration(0)
	if allowedInt == 0 {
		retryAfter = max(time.Duration(ttlSeconds) * time.Second, time.Second)
	}

	return Decision{
		Allowed:    allowedInt == 1,
		RetryAfter: retryAfter,
		Remaining:  remaining,
		Limit:      policy.Limit,
	}, nil
}