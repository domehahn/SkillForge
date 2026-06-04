package auth

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	// Create temporary database
	dbFile := t.TempDir() + "/test.db"
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(dbFile)
	}

	return db, cleanup
}

func TestUserRepository_CreateUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewUserRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "testuser", "password123", "user")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", user.Username)
	}

	if user.Role != "user" {
		t.Errorf("expected role 'user', got '%s'", user.Role)
	}

	// Verify password was hashed by attempting authentication
	_, err = repo.AuthenticateUser(ctx, "testuser", "password123")
	if err != nil {
		t.Errorf("authentication should succeed with correct password: %v", err)
	}

	// Verify wrong password fails
	_, err = repo.AuthenticateUser(ctx, "testuser", "wrongpassword")
	if err == nil {
		t.Error("authentication should fail with wrong password")
	}
}

func TestUserRepository_AuthenticateUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewUserRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Create a test user
	_, err = repo.CreateUser(ctx, "testuser", "password123", "user")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Test successful authentication
	user, err := repo.AuthenticateUser(ctx, "testuser", "password123")
	if err != nil {
		t.Fatalf("authentication should succeed: %v", err)
	}

	if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", user.Username)
	}

	// Test failed authentication (wrong password)
	_, err = repo.AuthenticateUser(ctx, "testuser", "wrongpassword")
	if err == nil {
		t.Error("authentication should fail with wrong password")
	}

	// Test failed authentication (non-existent user)
	_, err = repo.AuthenticateUser(ctx, "nonexistent", "password123")
	if err == nil {
		t.Error("authentication should fail for non-existent user")
	}
}

func TestUserRepository_CreateToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewUserRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Create a test user
	user, err := repo.CreateUser(ctx, "testuser", "password123", "user")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create a token
	scopes := []string{"read", "write"}
	expiresIn := 24 * time.Hour
	token, err := repo.CreateToken(ctx, user.ID, "Test Token", scopes, &expiresIn)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	if token.UserID != user.ID {
		t.Errorf("expected user_id %d, got %d", user.ID, token.UserID)
	}

	if token.Name != "Test Token" {
		t.Errorf("expected name 'Test Token', got '%s'", token.Name)
	}

	if len(token.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(token.Scopes))
	}

	if !strings.HasPrefix(token.Token, "skt_") {
		t.Errorf("token should start with 'skt_', got '%s'", token.Token[:4])
	}

	if token.TokenHash == "" {
		t.Error("token hash should be set")
	}
}

func TestUserRepository_ValidateToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewUserRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Create a test user
	user, err := repo.CreateUser(ctx, "testuser", "password123", "user")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create a token
	scopes := []string{"read", "write"}
	expiresIn := 24 * time.Hour
	token, err := repo.CreateToken(ctx, user.ID, "Test Token", scopes, &expiresIn)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Validate the token
	validatedUser, validatedScopes, err := repo.ValidateToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("token validation should succeed: %v", err)
	}

	if validatedUser.ID != user.ID {
		t.Errorf("expected user id %d, got %d", user.ID, validatedUser.ID)
	}

	if len(validatedScopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(validatedScopes))
	}

	// Test invalid token
	_, _, err = repo.ValidateToken(ctx, "skt_invalid_token")
	if err == nil {
		t.Error("validation should fail for invalid token")
	}

	// Test revoked token
	err = repo.RevokeToken(ctx, token.ID, user.ID)
	if err != nil {
		t.Fatalf("failed to revoke token: %v", err)
	}

	_, _, err = repo.ValidateToken(ctx, token.Token)
	if err == nil {
		t.Error("validation should fail for revoked token")
	}
}

func TestUserRepository_ListTokens(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewUserRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Create a test user
	user, err := repo.CreateUser(ctx, "testuser", "password123", "user")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create multiple tokens
	expiresIn := 24 * time.Hour
	_, err = repo.CreateToken(ctx, user.ID, "Token 1", []string{"read"}, &expiresIn)
	if err != nil {
		t.Fatalf("failed to create token 1: %v", err)
	}

	_, err = repo.CreateToken(ctx, user.ID, "Token 2", []string{"write"}, &expiresIn)
	if err != nil {
		t.Fatalf("failed to create token 2: %v", err)
	}

	// List tokens
	tokens, err := repo.ListTokens(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}

	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

func TestUserRepository_RevokeToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewUserRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Create a test user
	user, err := repo.CreateUser(ctx, "testuser", "password123", "user")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create a token
	expiresIn := 24 * time.Hour
	token, err := repo.CreateToken(ctx, user.ID, "Test Token", []string{"read"}, &expiresIn)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Revoke the token
	err = repo.RevokeToken(ctx, token.ID, user.ID)
	if err != nil {
		t.Fatalf("failed to revoke token: %v", err)
	}

	// Verify token is revoked
	_, _, err = repo.ValidateToken(ctx, token.Token)
	if err == nil {
		t.Error("validation should fail for revoked token")
	}
}
