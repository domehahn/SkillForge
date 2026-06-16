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

	"github.com/skillforge/skill-registry/internal/dbmigrate"
)

// User represents a registry user
type User struct {
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email,omitempty"`
	EmailVerified bool      `json:"email_verified"`
	PasswordHash  string    `json:"-"`    // Never expose in JSON
	Role          string    `json:"role"` // 'admin', 'user'
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Token represents an API token
type Token struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Name       string     `json:"name"`
	Token      string     `json:"token,omitempty"` // Only shown on creation
	TokenHash  string     `json:"-"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// EmailVerificationToken is returned when creating a verification challenge.
type EmailVerificationToken struct {
	Token     string
	ExpiresAt time.Time
}

// UserRepository manages users and tokens in the database
type UserRepository struct {
	db         *sql.DB
	bcryptCost int
}

// UserCreateOptions controls account creation details.
type UserCreateOptions struct {
	Username      string
	Email         string
	Password      string
	Role          string
	EmailVerified bool
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB) (*UserRepository, error) {
	return NewUserRepositoryWithBcryptCost(db, bcrypt.DefaultCost)
}

// NewUserRepositoryWithBcryptCost creates a user repository with a tuned bcrypt cost.
func NewUserRepositoryWithBcryptCost(db *sql.DB, bcryptCost int) (*UserRepository, error) {
	if bcryptCost < bcrypt.MinCost || bcryptCost > bcrypt.MaxCost {
		return nil, fmt.Errorf("bcrypt cost must be between %d and %d", bcrypt.MinCost, bcrypt.MaxCost)
	}
	repo := &UserRepository{db: db, bcryptCost: bcryptCost}
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run user migrations: %w", err)
	}
	return repo, nil
}

func (r *UserRepository) migrate() error {
	if err := dbmigrate.Run(r.db, "auth_001_initial", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
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
		`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "auth_002_token_last_used", func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE tokens ADD COLUMN last_used_at DATETIME`)
		return err
	}); err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}
	if err := dbmigrate.Run(r.db, "auth_003_user_email", func(tx *sql.Tx) error {
		if _, err := tx.Exec(`ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
		if _, err := tx.Exec(`ALTER TABLE users ADD COLUMN email_verified INTEGER NOT NULL DEFAULT 0`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
		_, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email != ''`)
		return err
	}); err != nil {
		return err
	}
	if err := dbmigrate.Run(r.db, "auth_004_email_verification_tokens", func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS email_verification_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token_hash TEXT UNIQUE NOT NULL,
			created_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL,
			used_at DATETIME,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
		CREATE INDEX IF NOT EXISTS idx_email_verification_hash ON email_verification_tokens(token_hash);
		CREATE INDEX IF NOT EXISTS idx_email_verification_user ON email_verification_tokens(user_id);`)
		return err
	}); err != nil {
		return err
	}
	return nil
}

// CreateUser creates a new user
func (r *UserRepository) CreateUser(ctx context.Context, username, password, role string) (*User, error) {
	return r.CreateUserWithOptions(ctx, UserCreateOptions{
		Username: username,
		Password: password,
		Role:     role,
	})
}

// CreateUserWithOptions creates a new user with account metadata.
func (r *UserRepository) CreateUserWithOptions(ctx context.Context, opts UserCreateOptions) (*User, error) {
	role := opts.Role
	if role == "" {
		role = "user"
	}
	if role != "user" && role != "admin" {
		return nil, fmt.Errorf("role must be 'user' or 'admin'")
	}
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(opts.Password), r.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	var id int64
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO users (username, email, email_verified, password_hash, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, opts.Username, strings.TrimSpace(opts.Email), boolToInt(opts.EmailVerified), string(hash), role, now, now).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{
		ID:            id,
		Username:      opts.Username,
		Email:         strings.TrimSpace(opts.Email),
		EmailVerified: opts.EmailVerified,
		Role:          role,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// GetUser retrieves a user by username
func (r *UserRepository) GetUser(ctx context.Context, username string) (*User, error) {
	user := &User{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, email, email_verified, password_hash, role, created_at, updated_at
		FROM users
		WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.Email, scanBool(&user.EmailVerified), &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)

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
		SELECT id, username, email, email_verified, password_hash, role, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id).Scan(&user.ID, &user.Username, &user.Email, scanBool(&user.EmailVerified), &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)

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

	now := time.Now()
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO tokens (user_id, name, token_hash, scopes, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`, userID, name, tokenHash, scopesStr, now, expiresAt).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	return &Token{
		ID:        id,
		UserID:    userID,
		Name:      name,
		Token:     token, // Return full token only on creation
		TokenHash: tokenHash,
		Scopes:    scopes,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateToken validates a token and returns the associated user and scopes
func (r *UserRepository) ValidateToken(ctx context.Context, token string) (*User, []string, error) {
	hash := sha256.Sum256([]byte(token))
	tokenHash := base64.RawURLEncoding.EncodeToString(hash[:])

	var tokenID, userID int64
	var scopesStr string
	var expiresAt, revokedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, scopes, expires_at, revoked_at
		FROM tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&tokenID, &userID, &scopesStr, &expiresAt, &revokedAt)

	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("invalid token")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate token: %w", err)
	}

	if revokedAt.Valid {
		return nil, nil, fmt.Errorf("token has been revoked")
	}
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		return nil, nil, fmt.Errorf("token has expired")
	}

	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	var scopes []string
	if scopesStr != "" {
		scopes = strings.Split(scopesStr, ",")
	}

	// Update last_used_at asynchronously — best-effort, never blocks auth
	go func() {
		_, _ = r.db.ExecContext(context.Background(), `UPDATE tokens SET last_used_at = ? WHERE id = ?`, time.Now(), tokenID)
	}()

	return user, scopes, nil
}

// ListTokens lists all tokens for a user
func (r *UserRepository) ListTokens(ctx context.Context, userID int64) ([]*Token, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, name, token_hash, scopes, created_at, expires_at, revoked_at, last_used_at
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
		var expiresAt, revokedAt, lastUsedAt sql.NullTime

		if err := rows.Scan(&token.ID, &token.UserID, &token.Name, &token.TokenHash,
			&scopesStr, &token.CreatedAt, &expiresAt, &revokedAt, &lastUsedAt); err != nil {
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
		if lastUsedAt.Valid {
			token.LastUsedAt = &lastUsedAt.Time
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

// ListUsers returns all users ordered by creation date
func (r *UserRepository) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, email, email_verified, role, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, scanBool(&u.EmailVerified), &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// UpdateUserRole updates a user's role
func (r *UserRepository) UpdateUserRole(ctx context.Context, username, role string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET role = ?, updated_at = ? WHERE username = ?
	`, role, time.Now(), username)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// DeleteUser permanently removes a user
func (r *UserRepository) DeleteUser(ctx context.Context, username string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE username = ?`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// CreateEmailVerificationToken creates a one-time verification token for a user.
func (r *UserRepository) CreateEmailVerificationToken(ctx context.Context, userID int64, ttl time.Duration) (*EmailVerificationToken, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate verification token: %w", err)
	}
	token := "ev_" + base64.RawURLEncoding.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	tokenHash := base64.RawURLEncoding.EncodeToString(hash[:])
	now := time.Now()
	expiresAt := now.Add(ttl)
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (user_id, token_hash, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`, userID, tokenHash, now, expiresAt); err != nil {
		return nil, fmt.Errorf("failed to create verification token: %w", err)
	}
	return &EmailVerificationToken{Token: token, ExpiresAt: expiresAt}, nil
}

// VerifyEmailToken marks the token's user as email-verified.
func (r *UserRepository) VerifyEmailToken(ctx context.Context, token string) (*User, error) {
	hash := sha256.Sum256([]byte(token))
	tokenHash := base64.RawURLEncoding.EncodeToString(hash[:])

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var tokenID, userID int64
	var expiresAt time.Time
	var usedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT id, user_id, expires_at, used_at
		FROM email_verification_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&tokenID, &userID, &expiresAt, &usedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid verification token")
		}
		return nil, fmt.Errorf("failed to query verification token: %w", err)
	}
	if usedAt.Valid {
		return nil, fmt.Errorf("verification token has already been used")
	}
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("verification token has expired")
	}
	now := time.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET email_verified = 1, updated_at = ? WHERE id = ?`, now, userID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE email_verification_tokens SET used_at = ? WHERE id = ?`, now, tokenID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetUserByID(ctx, userID)
}

// ChangePassword verifies the current password and updates to a new one.
func (r *UserRepository) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), r.bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		string(hash), time.Now(), userID,
	)
	return err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

type boolScanner struct {
	dest *bool
}

func scanBool(dest *bool) sql.Scanner {
	return boolScanner{dest: dest}
}

func (s boolScanner) Scan(value interface{}) error {
	switch v := value.(type) {
	case bool:
		*s.dest = v
	case int64:
		*s.dest = v != 0
	case int:
		*s.dest = v != 0
	case []byte:
		*s.dest = string(v) != "0" && string(v) != ""
	case string:
		*s.dest = v != "0" && v != ""
	case nil:
		*s.dest = false
	default:
		return fmt.Errorf("cannot scan %T into bool", value)
	}
	return nil
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
