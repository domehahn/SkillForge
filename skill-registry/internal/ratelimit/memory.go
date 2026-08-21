package ratelimit

import (
	"context"
	"sync"
	"time"
)

// MemoryBackend is a fixed-window rate limiter counting in-process. See
// Backend's doc comment for why this isn't sufficient once more than one
// registry replica is running.
type MemoryBackend struct {
	mu      sync.Mutex
	buckets map[string]*memoryBucket
	rate    int
	window  time.Duration
}

type memoryBucket struct {
	count     int
	windowEnd time.Time
}

// NewMemoryBackend builds an in-process rate limiter allowing up to rate
// requests per window, per key.
func NewMemoryBackend(rate int, window time.Duration) *MemoryBackend {
	b := &MemoryBackend{
		buckets: make(map[string]*memoryBucket),
		rate:    rate,
		window:  window,
	}
	go b.cleanup()
	return b
}

func (b *MemoryBackend) Allow(_ context.Context, key string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	bucket, ok := b.buckets[key]
	if !ok || now.After(bucket.windowEnd) {
		b.buckets[key] = &memoryBucket{count: 1, windowEnd: now.Add(b.window)}
		return true, nil
	}
	if bucket.count >= b.rate {
		return false, nil
	}
	bucket.count++
	return true, nil
}

func (b *MemoryBackend) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		b.mu.Lock()
		now := time.Now()
		for key, bucket := range b.buckets {
			if now.After(bucket.windowEnd) {
				delete(b.buckets, key)
			}
		}
		b.mu.Unlock()
	}
}
