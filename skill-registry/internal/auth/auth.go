package auth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

type contextKey string

const actorKey contextKey = "actor"
const userKey contextKey = "user"
const scopesKey contextKey = "scopes"

// Authenticator handles token-based authentication
type Authenticator struct {
	enabled  bool
	userRepo *UserRepository
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(enabled bool, db *sql.DB) (*Authenticator, error) {
	userRepo, err := NewUserRepository(db)
	if err != nil {
		return nil, err
	}

	return &Authenticator{
		enabled:  enabled,
		userRepo: userRepo,
	}, nil
}

// GetUserRepo returns the user repository
func (a *Authenticator) GetUserRepo() *UserRepository {
	return a.userRepo
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
				// No auth required for this endpoint (public read access)
				ctx := context.WithValue(r.Context(), actorKey, "anonymous")
				ctx = context.WithValue(ctx, scopesKey, []string{"read"})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error": {"code": "UNAUTHORIZED", "message": "Invalid authorization header"}}`, http.StatusUnauthorized)
				return
			}

			token := parts[1]
			user, scopes, err := a.userRepo.ValidateToken(r.Context(), token)
			if err != nil {
				http.Error(w, `{"error": {"code": "UNAUTHORIZED", "message": "Invalid or expired token"}}`, http.StatusUnauthorized)
				return
			}

			// Check if token has required scopes
			if len(requiredScopes) > 0 && !hasScopes(scopes, requiredScopes) {
				http.Error(w, `{"error": {"code": "FORBIDDEN", "message": "Insufficient permissions"}}`, http.StatusForbidden)
				return
			}

			// Add user, actor and scopes to context
			ctx := context.WithValue(r.Context(), actorKey, user.Username)
			ctx = context.WithValue(ctx, userKey, user)
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

// UserFromContext retrieves the user from context
func UserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(userKey).(*User); ok {
		return user
	}
	return nil
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
