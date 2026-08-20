package ratelimit

import "context"

// Backend is the counting strategy a Limiter uses to decide whether a
// request identified by key is allowed. MemoryBackend counts in-process —
// fine for a single instance, but each replica behind a load balancer gets
// its own independent counters, so a client can get roughly
// (numReplicas × rate) requests per window rather than rate, defeating
// the point of the limit at any real scale. RedisBackend shares counters
// across every replica pointed at the same Redis instance, so the limit
// holds regardless of which replica a given request lands on.
type Backend interface {
	Allow(ctx context.Context, key string) (bool, error)
}
