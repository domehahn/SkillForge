package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisBackend is a fixed-window rate limiter whose counters live in
// Redis, shared across every registry replica pointed at the same
// instance — see Backend's doc comment for why that matters.
type RedisBackend struct {
	client *redis.Client
	rate   int
	window time.Duration
	prefix string
}

// incrementAndExpire atomically increments the counter at KEYS[1] and, only
// the first time it's created (count == 1), sets its expiry to ARGV[1]
// milliseconds. Doing this as one script avoids the race between a plain
// INCR and a separate PEXPIRE, where a crash or reordering between the two
// calls could leave a key with no expiry (a slow permanent leak) or reset
// its expiry on every request (a window that never closes).
var incrementAndExpire = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`)

// NewRedisBackend builds a Redis-backed rate limiter allowing up to rate
// requests per window, per key, using client. keyPrefix namespaces this
// limiter's keys in Redis (useful if the same Redis instance is shared
// with other data) — pass "" for no prefix.
func NewRedisBackend(client *redis.Client, rate int, window time.Duration, keyPrefix string) *RedisBackend {
	return &RedisBackend{client: client, rate: rate, window: window, prefix: keyPrefix}
}

func (b *RedisBackend) Allow(ctx context.Context, key string) (bool, error) {
	redisKey := b.prefix + key
	count, err := incrementAndExpire.Run(ctx, b.client, []string{redisKey}, b.window.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("ratelimit: redis: %w", err)
	}
	return count <= int64(b.rate), nil
}
