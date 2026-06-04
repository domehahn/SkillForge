package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const actorKey contextKey = "actor"
const scopesKey contextKey = "scopes"

// Authenticator handles token-based authentication
type Authenticator struct {
	enabled bool
	tokens  map[string][]string // token -> scopes
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(enabled bool, tokens map[string][]string) *Authenticator {
	return &Authenticator{
		enabled: enabled,
		tokens:  tokens,
	}
}

// Middleware returns an authentication middleware
func (a *Authenticator) Middleware(requiredScopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If auth is disabled, allow all requests
			if !a.enabled {
				ctx := context.WithValue(r.Context(), actorKey, "anonymous")
				ctx = context.WithValue(ctx, scopesKey, []string{"read", "write", "delete"})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Extract token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				if len(requiredScopes) > 0 {
					http.Error(w, `{"error": {"code": "UNAUTHORIZED", "message": "Missing authorization header"}}`, http.StatusUnauthorized)
					return
				}
				// No auth required for this endpoint
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error": {"code": "UNAUTHORIZED", "message": "Invalid authorization header"}}`, http.StatusUnauthorized)
				return
			}

			token := parts[1]
			scopes, ok := a.tokens[token]
			if !ok {
				http.Error(w, `{"error": {"code": "UNAUTHORIZED", "message": "Invalid token"}}`, http.StatusUnauthorized)
				return
			}

			// Check if token has required scopes
			if len(requiredScopes) > 0 && !hasScopes(scopes, requiredScopes) {
				http.Error(w, `{"error": {"code": "FORBIDDEN", "message": "Insufficient permissions"}}`, http.StatusForbidden)
				return
			}

			// Add actor and scopes to context
			ctx := context.WithValue(r.Context(), actorKey, "authenticated")
			ctx = context.WithValue(ctx, scopesKey, scopes)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ActorFromContext retrieves the actor from context
func ActorFromContext(ctx context.Context) string {
	if actor, ok := ctx.Value(actorKey).(string); ok {
		return actor
	}
	return "anonymous"
}

// ScopesFromContext retrieves the scopes from context
func ScopesFromContext(ctx context.Context) []string {
	if scopes, ok := ctx.Value(scopesKey).([]string); ok {
		return scopes
	}
	return []string{}
}

func hasScopes(userScopes, requiredScopes []string) bool {
	scopeMap := make(map[string]bool)
	for _, s := range userScopes {
		scopeMap[s] = true
	}

	for _, required := range requiredScopes {
		if !scopeMap[required] {
			return false
		}
	}

	return true
}
