package observability

import (
	"net/http"
	"strconv"

	"github.com/skillforge/skill-registry/internal/config"
)

// SecurityHeadersMiddleware applies baseline browser hardening headers.
func SecurityHeadersMiddleware(cfg config.SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			}
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if cfg.HSTSEnabled && isHTTPS(r) && cfg.HSTSMaxAgeSeconds > 0 {
				w.Header().Set("Strict-Transport-Security", "max-age="+strconv.Itoa(cfg.HSTSMaxAgeSeconds)+"; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
