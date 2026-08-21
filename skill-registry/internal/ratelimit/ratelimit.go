package ratelimit

import (
	"log/slog"
	"net/http"

	"github.com/skillforge/skill-registry/internal/clientip"
)

// Limiter applies per-key rate limiting to HTTP requests. The actual
// counting is delegated to a Backend (in-process or Redis-backed); Limiter
// itself only handles deriving the per-request key (preferring an
// authenticated token over client IP) and wiring that into the Backend.
type Limiter struct {
	backend   Backend
	clientIPs *clientip.Resolver
	logger    *slog.Logger
}

// New builds a Limiter around backend. trustedProxyCIDRs
// (config.Config.Security.TrustedProxies) controls whether
// X-Forwarded-For is honored when deriving the per-IP key — see
// internal/clientip's package doc for why an unconditionally-trusted
// X-Forwarded-For lets any client bypass rate limiting by sending a fresh
// header value per request. logger may be nil (a backend error is then
// silently treated as fail-open — see Allow).
func New(backend Backend, trustedProxyCIDRs []string, logger *slog.Logger) *Limiter {
	return &Limiter{
		backend:   backend,
		clientIPs: clientip.NewResolver(trustedProxyCIDRs),
		logger:    logger,
	}
}

// Allow reports whether r should proceed. A Backend error (e.g. Redis is
// unreachable) fails open — the request is allowed, and the error is
// logged if a logger was configured — rather than turning a rate-limiter
// backend outage into a full service outage. Rate limiting is a
// defense-in-depth measure, not the primary access control.
func (l *Limiter) Allow(r *http.Request) bool {
	ok, err := l.backend.Allow(r.Context(), l.clientKey(r))
	if err != nil {
		if l.logger != nil {
			l.logger.Warn("rate limit backend error; failing open", "error", err)
		}
		return true
	}
	return ok
}

// Middleware applies per-key rate limiting. Key is derived from token suffix or
// client IP, preferring the token so authenticated clients get their own bucket.
func Middleware(l *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(r) {
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"rate limit exceeded","code":"RATE_LIMITED"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *Limiter) clientKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); len(auth) > 12 {
		return "tok:" + auth[len(auth)-12:]
	}
	return "ip:" + l.clientIPs.Resolve(r)
}
