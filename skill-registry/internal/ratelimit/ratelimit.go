package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int
	window  time.Duration
}

type bucket struct {
	count     int
	windowEnd time.Time
}

func New(rate int, window time.Duration) *Limiter {
	l := &Limiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		window:  window,
	}
	go l.cleanup()
	return l
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.buckets[key]
	if !ok || now.After(b.windowEnd) {
		l.buckets[key] = &bucket{count: 1, windowEnd: now.Add(l.window)}
		return true
	}
	if b.count >= l.rate {
		return false
	}
	b.count++
	return true
}

func (l *Limiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for key, b := range l.buckets {
			if now.After(b.windowEnd) {
				delete(l.buckets, key)
			}
		}
		l.mu.Unlock()
	}
}

// Middleware applies per-key rate limiting. Key is derived from token suffix or
// client IP, preferring the token so authenticated clients get their own bucket.
func Middleware(l *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(clientKey(r)) {
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"rate limit exceeded","code":"RATE_LIMITED"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); len(auth) > 12 {
		return "tok:" + auth[len(auth)-12:]
	}
	return "ip:" + clientIP(r)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
