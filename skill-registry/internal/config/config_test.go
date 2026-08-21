package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if cfg.Server.Addr != ":8080" {
		t.Errorf("expected server addr :8080, got %s", cfg.Server.Addr)
	}

	if cfg.Storage.DataDir != "./data" {
		t.Errorf("expected data dir ./data, got %s", cfg.Storage.DataDir)
	}

	if cfg.Storage.MaxPackageSizeMB != 50 {
		t.Errorf("expected max package size 50, got %d", cfg.Storage.MaxPackageSizeMB)
	}

	if len(cfg.Storage.AllowedPackageTypes) != 2 {
		t.Errorf("expected 2 allowed package types, got %d", len(cfg.Storage.AllowedPackageTypes))
	}

	if cfg.Database.Path != "./data/registry.db" {
		t.Errorf("expected db path ./data/registry.db, got %s", cfg.Database.Path)
	}

	if cfg.Auth.Enabled {
		t.Error("expected auth disabled by default")
	}

	if len(cfg.Auth.Tokens) != 0 {
		t.Errorf("expected empty tokens list, got %d", len(cfg.Auth.Tokens))
	}

	if cfg.Proxy.Enabled {
		t.Error("expected proxy disabled by default")
	}

	if len(cfg.Validation.BlockedExtensions) == 0 {
		t.Error("expected some blocked extensions by default")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Load should not fail for non-existent file: %v", err)
	}

	// Should return defaults
	if cfg.Server.Addr != ":8080" {
		t.Errorf("expected default addr, got %s", cfg.Server.Addr)
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load with empty path should not fail: %v", err)
	}

	// Should return defaults
	if cfg.Server.Addr != ":8080" {
		t.Errorf("expected default addr, got %s", cfg.Server.Addr)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  addr: ":9090"
storage:
  data_dir: "/custom/data"
  max_package_size_mb: 100
  allowed_package_types:
    - tgz
    - zip
database:
  path: "/custom/db.sqlite"
auth:
  enabled: true
  bcrypt_cost: 12
  require_email_verification: true
  tokens:
    - name: admin
      token_env: ADMIN_TOKEN
      scopes:
        - read
        - write
proxy:
  enabled: true
  upstreams:
    - https://example.com
validation:
  blocked_extensions:
    - .exe
    - .dll
security:
  headers_enabled: true
  hsts_enabled: true
  hsts_max_age_seconds: 123
  openapi_cors_origin: "https://docs.example.com"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Addr != ":9090" {
		t.Errorf("expected addr :9090, got %s", cfg.Server.Addr)
	}

	if cfg.Storage.DataDir != "/custom/data" {
		t.Errorf("expected data dir /custom/data, got %s", cfg.Storage.DataDir)
	}

	if cfg.Storage.MaxPackageSizeMB != 100 {
		t.Errorf("expected max size 100, got %d", cfg.Storage.MaxPackageSizeMB)
	}

	if cfg.Database.Path != "/custom/db.sqlite" {
		t.Errorf("expected db path /custom/db.sqlite, got %s", cfg.Database.Path)
	}

	if !cfg.Auth.Enabled {
		t.Error("expected auth enabled")
	}
	if cfg.Auth.BcryptCost != 12 {
		t.Errorf("expected bcrypt cost 12, got %d", cfg.Auth.BcryptCost)
	}
	if !cfg.Auth.RequireEmailVerification {
		t.Error("expected email verification requirement")
	}

	if len(cfg.Auth.Tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(cfg.Auth.Tokens))
	}

	if cfg.Auth.Tokens[0].Name != "admin" {
		t.Errorf("expected token name admin, got %s", cfg.Auth.Tokens[0].Name)
	}

	if !cfg.Proxy.Enabled {
		t.Error("expected proxy enabled")
	}

	if len(cfg.Proxy.Upstreams) != 1 {
		t.Errorf("expected 1 upstream, got %d", len(cfg.Proxy.Upstreams))
	}
	if cfg.Security.HSTSMaxAgeSeconds != 123 {
		t.Errorf("expected HSTS max age 123, got %d", cfg.Security.HSTSMaxAgeSeconds)
	}
	if cfg.Security.OpenAPICORSOrigin != "https://docs.example.com" {
		t.Errorf("expected configured OpenAPI CORS origin, got %s", cfg.Security.OpenAPICORSOrigin)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `
server:
  addr: [invalid yaml structure
`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "unreadable.yaml")

	if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Make file unreadable
	if err := os.Chmod(configPath, 0000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(configPath, 0644)

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for unreadable file")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	// Save original env vars
	originalAddr := os.Getenv("SKILL_REGISTRY_ADDR")
	originalDataDir := os.Getenv("SKILL_REGISTRY_DATA_DIR")
	originalDBPath := os.Getenv("SKILL_REGISTRY_DB_PATH")
	originalMaxSize := os.Getenv("SKILL_REGISTRY_MAX_PACKAGE_SIZE_MB")
	originalAuthEnabled := os.Getenv("SKILL_REGISTRY_AUTH_ENABLED")
	originalBcryptCost := os.Getenv("SKILL_REGISTRY_AUTH_BCRYPT_COST")
	originalRequireEmail := os.Getenv("SKILL_REGISTRY_REQUIRE_EMAIL_VERIFICATION")
	originalOpenAPIOrigin := os.Getenv("SKILL_REGISTRY_OPENAPI_CORS_ORIGIN")

	// Restore after test
	defer func() {
		os.Setenv("SKILL_REGISTRY_ADDR", originalAddr)
		os.Setenv("SKILL_REGISTRY_DATA_DIR", originalDataDir)
		os.Setenv("SKILL_REGISTRY_DB_PATH", originalDBPath)
		os.Setenv("SKILL_REGISTRY_MAX_PACKAGE_SIZE_MB", originalMaxSize)
		os.Setenv("SKILL_REGISTRY_AUTH_ENABLED", originalAuthEnabled)
		os.Setenv("SKILL_REGISTRY_AUTH_BCRYPT_COST", originalBcryptCost)
		os.Setenv("SKILL_REGISTRY_REQUIRE_EMAIL_VERIFICATION", originalRequireEmail)
		os.Setenv("SKILL_REGISTRY_OPENAPI_CORS_ORIGIN", originalOpenAPIOrigin)
	}()

	// Set test env vars
	os.Setenv("SKILL_REGISTRY_ADDR", ":9999")
	os.Setenv("SKILL_REGISTRY_DATA_DIR", "/env/data")
	os.Setenv("SKILL_REGISTRY_DB_PATH", "/env/db.sqlite")
	os.Setenv("SKILL_REGISTRY_MAX_PACKAGE_SIZE_MB", "200")
	os.Setenv("SKILL_REGISTRY_AUTH_ENABLED", "true")
	os.Setenv("SKILL_REGISTRY_AUTH_BCRYPT_COST", "11")
	os.Setenv("SKILL_REGISTRY_REQUIRE_EMAIL_VERIFICATION", "true")
	os.Setenv("SKILL_REGISTRY_OPENAPI_CORS_ORIGIN", "https://docs.example.com")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.Server.Addr != ":9999" {
		t.Errorf("expected addr :9999, got %s", cfg.Server.Addr)
	}

	if cfg.Storage.DataDir != "/env/data" {
		t.Errorf("expected data dir /env/data, got %s", cfg.Storage.DataDir)
	}

	if cfg.Database.Path != "/env/db.sqlite" {
		t.Errorf("expected db path /env/db.sqlite, got %s", cfg.Database.Path)
	}

	if cfg.Storage.MaxPackageSizeMB != 200 {
		t.Errorf("expected max size 200, got %d", cfg.Storage.MaxPackageSizeMB)
	}

	if !cfg.Auth.Enabled {
		t.Error("expected auth enabled")
	}
	if cfg.Auth.BcryptCost != 11 {
		t.Errorf("expected bcrypt cost 11, got %d", cfg.Auth.BcryptCost)
	}
	if !cfg.Auth.RequireEmailVerification {
		t.Error("expected require email verification enabled")
	}
	if cfg.Security.OpenAPICORSOrigin != "https://docs.example.com" {
		t.Errorf("expected OpenAPI CORS origin override, got %s", cfg.Security.OpenAPICORSOrigin)
	}
}

func TestApplyEnvOverrides_AuthEnabledVariations(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true lowercase", "true", true},
		{"TRUE uppercase", "TRUE", true},
		{"1", "1", true},
		{"false", "false", false},
		{"0", "0", false},
		{"empty", "", false},
		{"other", "yes", false},
	}

	originalAuthEnabled := os.Getenv("SKILL_REGISTRY_AUTH_ENABLED")
	defer os.Setenv("SKILL_REGISTRY_AUTH_ENABLED", originalAuthEnabled)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SKILL_REGISTRY_AUTH_ENABLED", tt.value)
			cfg := DefaultConfig()
			applyEnvOverrides(cfg)

			if cfg.Auth.Enabled != tt.expected {
				t.Errorf("expected auth enabled %v, got %v", tt.expected, cfg.Auth.Enabled)
			}
		})
	}
}

func TestApplyEnvOverrides_InvalidMaxSize(t *testing.T) {
	originalMaxSize := os.Getenv("SKILL_REGISTRY_MAX_PACKAGE_SIZE_MB")
	defer os.Setenv("SKILL_REGISTRY_MAX_PACKAGE_SIZE_MB", originalMaxSize)

	// Set invalid value - should be ignored
	os.Setenv("SKILL_REGISTRY_MAX_PACKAGE_SIZE_MB", "not-a-number")

	cfg := DefaultConfig()
	originalSize := cfg.Storage.MaxPackageSizeMB
	applyEnvOverrides(cfg)

	// Should remain at default value
	if cfg.Storage.MaxPackageSizeMB != originalSize {
		t.Errorf("invalid max size should be ignored, expected %d, got %d", originalSize, cfg.Storage.MaxPackageSizeMB)
	}
}

func TestResolveTokens(t *testing.T) {
	// Save original env vars
	originalToken1 := os.Getenv("TEST_TOKEN_1")
	originalToken2 := os.Getenv("TEST_TOKEN_2")

	defer func() {
		os.Setenv("TEST_TOKEN_1", originalToken1)
		os.Setenv("TEST_TOKEN_2", originalToken2)
	}()

	// Set test tokens
	os.Setenv("TEST_TOKEN_1", "secret1")
	os.Setenv("TEST_TOKEN_2", "secret2")

	cfg := &Config{
		Auth: AuthConfig{
			Enabled: true,
			Tokens: []TokenEntry{
				{
					Name:     "user1",
					TokenEnv: "TEST_TOKEN_1",
					Scopes:   []string{"read", "write"},
				},
				{
					Name:     "user2",
					TokenEnv: "TEST_TOKEN_2",
					Scopes:   []string{"read"},
				},
			},
		},
	}

	tokens := cfg.ResolveTokens()

	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}

	if scopes, ok := tokens["secret1"]; !ok {
		t.Error("expected secret1 in tokens")
	} else if len(scopes) != 2 {
		t.Errorf("expected 2 scopes for secret1, got %d", len(scopes))
	}

	if scopes, ok := tokens["secret2"]; !ok {
		t.Error("expected secret2 in tokens")
	} else if len(scopes) != 1 {
		t.Errorf("expected 1 scope for secret2, got %d", len(scopes))
	}
}

func TestResolveTokens_MissingEnvVar(t *testing.T) {
	// Ensure env var doesn't exist
	os.Unsetenv("NONEXISTENT_TOKEN")

	cfg := &Config{
		Auth: AuthConfig{
			Enabled: true,
			Tokens: []TokenEntry{
				{
					Name:     "user1",
					TokenEnv: "NONEXISTENT_TOKEN",
					Scopes:   []string{"read"},
				},
			},
		},
	}

	tokens := cfg.ResolveTokens()

	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens when env var missing, got %d", len(tokens))
	}
}

func TestResolveTokens_EmptyTokenEnv(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{
			Enabled: true,
			Tokens: []TokenEntry{
				{
					Name:     "user1",
					TokenEnv: "", // empty
					Scopes:   []string{"read"},
				},
			},
		},
	}

	tokens := cfg.ResolveTokens()

	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens when TokenEnv is empty, got %d", len(tokens))
	}
}

func TestResolveTokens_EmptyList(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{
			Enabled: true,
			Tokens:  []TokenEntry{},
		},
	}

	tokens := cfg.ResolveTokens()

	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens from empty list, got %d", len(tokens))
	}
}

func TestValidateProductionEnforce(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Production.Mode = true
	cfg.Production.Enforce = true

	if err := cfg.ValidateProduction(); err == nil {
		t.Fatal("expected production enforcement to fail for default development config")
	}

	cfg.Auth.Enabled = true
	cfg.Auth.BcryptCost = 12
	cfg.Security.HeadersEnabled = true
	cfg.Security.OpenAPICORSOrigin = "https://registry.example.com"
	cfg.TLS.Enabled = true
	cfg.TLS.CertFile = filepath.Join(t.TempDir(), "cert.pem")
	cfg.TLS.KeyFile = filepath.Join(t.TempDir(), "key.pem")
	cfg.RateLimit.Enabled = true
	cfg.Backup.Enabled = true
	cfg.Auth.RequireEmailVerification = true
	cfg.Email.SMTPEnabled = true
	cfg.Email.SMTPHost = "smtp.example.com"

	if err := os.WriteFile(cfg.TLS.CertFile, []byte("cert"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.TLS.KeyFile, []byte("key"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateProduction(); err != nil {
		t.Fatalf("expected production enforcement to pass, got %v", err)
	}
}

func TestValidate_TrustedProxiesRejectsInvalidCIDR(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Security.TrustedProxies = []string{"not-a-cidr"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected an invalid CIDR in security.trusted_proxies to fail validation")
	}
}

func TestValidate_TrustedProxiesAcceptsValidCIDR(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Security.TrustedProxies = []string{"10.0.0.0/8", "127.0.0.1/32"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid CIDRs to pass validation, got %v", err)
	}
}

func TestApplyEnvOverrides_TrustedProxies(t *testing.T) {
	t.Setenv("SKILL_REGISTRY_TRUSTED_PROXIES", "10.0.0.0/8, 127.0.0.1/32,192.168.1.0/24")
	cfg := DefaultConfig()
	applyEnvOverrides(cfg)
	want := []string{"10.0.0.0/8", "127.0.0.1/32", "192.168.1.0/24"}
	if len(cfg.Security.TrustedProxies) != len(want) {
		t.Fatalf("expected %v, got %v", want, cfg.Security.TrustedProxies)
	}
	for i, v := range want {
		if cfg.Security.TrustedProxies[i] != v {
			t.Fatalf("expected %v, got %v", want, cfg.Security.TrustedProxies)
		}
	}
}
