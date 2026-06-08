package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a registry user
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`    // Never expose in JSON
	Role         string    `json:"role"` // 'admin', 'user'
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Token represents an API token
type Token struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	Name      string     `json:"name"`
	Token     string     `json:"token,omitempty"` // Only shown on creation
	TokenHash string     `json:"-"`
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// UserRepository manages users and tokens in the database
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB) (*UserRepository, error) {
	repo := &UserRepository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run user migrations: %w", err)
	}
	return repo, nil
}

func (r *UserRepository) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

	CREATE TABLE IF NOT EXISTS tokens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		token_hash TEXT UNIQUE NOT NULL,
		scopes TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME,
		revoked_at DATETIME,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);

	CREATE INDEX IF NOT EXISTS idx_tokens_hash ON tokens(token_hash);
	CREATE INDEX IF NOT EXISTS idx_tokens_user ON tokens(user_id);
	`

	_, err := r.db.Exec(schema)
	return err
}

// CreateUser creates a new user
func (r *UserRepository) CreateUser(ctx context.Context, username, password, role string) (*User, error) {
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, username, string(hash), role, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &User{
		ID:        id,
		Username:  username,
		Role:      role,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetUser retrieves a user by username
func (r *UserRepository) GetUser(ctx context.Context, username string) (*User, error) {
	user := &User{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users
		WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// GetUserByID retrieves a user by ID
func (r *UserRepository) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user := &User{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// AuthenticateUser validates username and password
func (r *UserRepository) AuthenticateUser(ctx context.Context, username, password string) (*User, error) {
	user, err := r.GetUser(ctx, username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}

// CreateToken creates a new API token for a user
func (r *UserRepository) CreateToken(ctx context.Context, userID int64, name string, scopes []string, expiresIn *time.Duration) (*Token, error) {
	// Generate token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := "skt_" + base64.RawURLEncoding.EncodeToString(tokenBytes)

	// Hash token for storage
	hash := sha256.Sum256([]byte(token))
	tokenHash := base64.RawURLEncoding.EncodeToString(hash[:])

	// Calculate expiration
	var expiresAt *time.Time
	if expiresIn != nil {
		exp := time.Now().Add(*expiresIn)
		expiresAt = &exp
	}

	// Store in database
	scopesStr := ""
	if len(scopes) > 0 {
		scopesStr = strings.Join(scopes, ",")
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO tokens (user_id, name, token_hash, scopes, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, name, tokenHash, scopesStr, time.Now(), expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Token{
		ID:        id,
		UserID:    userID,
		Name:      name,
		Token:     token, // Return full token only on creation
		TokenHash: tokenHash,
		Scopes:    scopes,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateToken validates a token and returns the associated user and scopes
func (r *UserRepository) ValidateToken(ctx context.Context, token string) (*User, []string, error) {
	// Hash the provided token
	hash := sha256.Sum256([]byte(token))
	tokenHash := base64.RawURLEncoding.EncodeToString(hash[:])

	// Look up token
	var userID int64
	var scopesStr string
	var expiresAt, revokedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT user_id, scopes, expires_at, revoked_at
		FROM tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&userID, &scopesStr, &expiresAt, &revokedAt)

	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("invalid token")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate token: %w", err)
	}

	// Check if revoked
	if revokedAt.Valid {
		return nil, nil, fmt.Errorf("token has been revoked")
	}

	// Check if expired
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		return nil, nil, fmt.Errorf("token has expired")
	}

	// Get user
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	// Parse scopes
	var scopes []string
	if scopesStr != "" {
		scopes = strings.Split(scopesStr, ",")
	}

	return user, scopes, nil
}

// ListTokens lists all tokens for a user
func (r *UserRepository) ListTokens(ctx context.Context, userID int64) ([]*Token, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, name, token_hash, scopes, created_at, expires_at, revoked_at
		FROM tokens
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*Token
	for rows.Next() {
		token := &Token{}
		var scopesStr string
		var expiresAt, revokedAt sql.NullTime

		if err := rows.Scan(&token.ID, &token.UserID, &token.Name, &token.TokenHash,
			&scopesStr, &token.CreatedAt, &expiresAt, &revokedAt); err != nil {
			return nil, err
		}

		if scopesStr != "" {
			token.Scopes = strings.Split(scopesStr, ",")
		}
		if expiresAt.Valid {
			token.ExpiresAt = &expiresAt.Time
		}
		if revokedAt.Valid {
			token.RevokedAt = &revokedAt.Time
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

// RevokeToken revokes a token
func (r *UserRepository) RevokeToken(ctx context.Context, tokenID int64, userID int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE tokens
		SET revoked_at = ?
		WHERE id = ? AND user_id = ? AND revoked_at IS NULL
	`, time.Now(), tokenID, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("token not found or already revoked")
	}

	return nil
}
