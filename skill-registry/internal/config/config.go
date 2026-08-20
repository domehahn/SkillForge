package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the registry configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	TLS        TLSConfig        `yaml:"tls"`
	Security   SecurityConfig   `yaml:"security"`
	Storage    StorageConfig    `yaml:"storage"`
	Database   DatabaseConfig   `yaml:"database"`
	Auth       AuthConfig       `yaml:"auth"`
	Email      EmailConfig      `yaml:"email"`
	Backup     BackupConfig     `yaml:"backup"`
	Production ProductionConfig `yaml:"production"`
	Proxy      ProxyConfig      `yaml:"proxy"`
	Validation ValidationConfig `yaml:"validation"`
	RateLimit  RateLimitConfig  `yaml:"rate_limit"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
	HTTPRedirect bool   `yaml:"http_redirect"` // redirect plain HTTP to HTTPS
	HTTPAddr     string `yaml:"http_addr"`     // addr for the HTTP redirect listener
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Addr string `yaml:"addr"`
}

// SecurityConfig controls HTTP security hardening headers and public CORS surfaces.
type SecurityConfig struct {
	HeadersEnabled        bool   `yaml:"headers_enabled"`
	ContentSecurityPolicy string `yaml:"content_security_policy"`
	HSTSEnabled           bool   `yaml:"hsts_enabled"`
	HSTSMaxAgeSeconds     int    `yaml:"hsts_max_age_seconds"`
	OpenAPICORSOrigin     string `yaml:"openapi_cors_origin"`
	// TrustedProxies lists CIDR ranges (e.g. "10.0.0.0/8", "127.0.0.1/32")
	// whose X-Forwarded-For header is honored for client IP resolution
	// (audit logging, rate limiting). A request whose immediate peer
	// (RemoteAddr) is NOT in this list has its X-Forwarded-For header
	// ignored entirely — the peer's own address is used instead. Empty by
	// default: with no trusted proxies configured, X-Forwarded-For is never
	// honored, which is the safe default for a directly-exposed server.
	// Deploying behind a reverse proxy that sets X-Forwarded-For requires
	// listing that proxy's address here, or every client can spoof its
	// audit-logged and rate-limited identity by just setting the header
	// itself.
	TrustedProxies []string `yaml:"trusted_proxies"`
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	DataDir             string   `yaml:"data_dir"`
	MaxPackageSizeMB    int      `yaml:"max_package_size_mb"`
	AllowedPackageTypes []string `yaml:"allowed_package_types"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	Path   string `yaml:"path"`
	DSN    string `yaml:"dsn"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled                  bool         `yaml:"enabled"`
	BcryptCost               int          `yaml:"bcrypt_cost"`
	RequireEmailVerification bool         `yaml:"require_email_verification"`
	Tokens                   []TokenEntry `yaml:"tokens"`
}

// EmailConfig controls SMTP email delivery.
type EmailConfig struct {
	SMTPEnabled  bool   `yaml:"smtp_enabled"`
	SMTPHost     string `yaml:"smtp_host"`
	SMTPPort     int    `yaml:"smtp_port"`
	SMTPUsername string `yaml:"smtp_username"`
	SMTPPassword string `yaml:"smtp_password"`
	From         string `yaml:"from"`
	BaseURL      string `yaml:"base_url"`
}

// BackupConfig controls scheduled local backup creation.
type BackupConfig struct {
	Enabled         bool   `yaml:"enabled"`
	OutputDir       string `yaml:"output_dir"`
	IntervalMinutes int    `yaml:"interval_minutes"`
	RetentionCopies int    `yaml:"retention_copies"`
}

// ProductionConfig controls startup production-readiness enforcement.
type ProductionConfig struct {
	Mode    bool `yaml:"mode"`
	Enforce bool `yaml:"enforce"`
}

// TokenEntry represents a single auth token configuration
type TokenEntry struct {
	Name     string   `yaml:"name"`
	TokenEnv string   `yaml:"token_env"`
	Scopes   []string `yaml:"scopes"`
}

// ProxyConfig holds upstream proxy configuration
type ProxyConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Upstreams []string `yaml:"upstreams"`
}

// ValidationConfig holds validation rules
type ValidationConfig struct {
	BlockedExtensions []string `yaml:"blocked_extensions"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr: ":8080",
		},
		Security: SecurityConfig{
			HeadersEnabled:        true,
			ContentSecurityPolicy: "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; object-src 'none'; img-src 'self' data:; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self' https://cdn.jsdelivr.net; connect-src 'self'",
			HSTSEnabled:           true,
			HSTSMaxAgeSeconds:     31536000,
			OpenAPICORSOrigin:     "*",
		},
		Storage: StorageConfig{
			DataDir:             "./data",
			MaxPackageSizeMB:    50,
			AllowedPackageTypes: []string{"tgz", "zip"},
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			Path:   "./data/registry.db",
		},
		Auth: AuthConfig{
			Enabled:                  false,
			BcryptCost:               10,
			RequireEmailVerification: false,
			Tokens:                   []TokenEntry{},
		},
		Email: EmailConfig{
			SMTPEnabled: false,
			SMTPPort:    587,
			From:        "SkillForge <noreply@localhost>",
			BaseURL:     "http://localhost:8080",
		},
		Backup: BackupConfig{
			Enabled:         false,
			OutputDir:       "./backups",
			IntervalMinutes: 360,
			RetentionCopies: 14,
		},
		Production: ProductionConfig{
			Mode:    false,
			Enforce: false,
		},
		Proxy: ProxyConfig{
			Enabled:   false,
			Upstreams: []string{},
		},
		Validation: ValidationConfig{
			BlockedExtensions: []string{".exe", ".dll", ".so", ".dylib", ".bin"},
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 300,
		},
	}
}

// Load loads configuration from a file and applies environment overrides
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Load from file if it exists
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
			// File doesn't exist, use defaults
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Apply environment overrides
	applyEnvOverrides(cfg)

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SKILL_REGISTRY_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("SKILL_REGISTRY_DATA_DIR"); v != "" {
		cfg.Storage.DataDir = v
	}
	if v := os.Getenv("SKILL_REGISTRY_DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("SKILL_REGISTRY_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("SKILL_REGISTRY_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("SKILL_REGISTRY_MAX_PACKAGE_SIZE_MB"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Storage.MaxPackageSizeMB = i
		}
	}
	if v := os.Getenv("SKILL_REGISTRY_AUTH_ENABLED"); v != "" {
		cfg.Auth.Enabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_AUTH_BCRYPT_COST"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Auth.BcryptCost = i
		}
	}
	if v := os.Getenv("SKILL_REGISTRY_REQUIRE_EMAIL_VERIFICATION"); v != "" {
		cfg.Auth.RequireEmailVerification = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_SECURITY_HEADERS_ENABLED"); v != "" {
		cfg.Security.HeadersEnabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_HSTS_ENABLED"); v != "" {
		cfg.Security.HSTSEnabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_HSTS_MAX_AGE_SECONDS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Security.HSTSMaxAgeSeconds = i
		}
	}
	if v := os.Getenv("SKILL_REGISTRY_OPENAPI_CORS_ORIGIN"); v != "" {
		cfg.Security.OpenAPICORSOrigin = v
	}
	if v := os.Getenv("SKILL_REGISTRY_TRUSTED_PROXIES"); v != "" {
		var proxies []string
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				proxies = append(proxies, p)
			}
		}
		cfg.Security.TrustedProxies = proxies
	}
	if v := os.Getenv("SKILL_REGISTRY_EMAIL_SMTP_ENABLED"); v != "" {
		cfg.Email.SMTPEnabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_EMAIL_SMTP_HOST"); v != "" {
		cfg.Email.SMTPHost = v
	}
	if v := os.Getenv("SKILL_REGISTRY_EMAIL_SMTP_PORT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Email.SMTPPort = i
		}
	}
	if v := os.Getenv("SKILL_REGISTRY_EMAIL_SMTP_USERNAME"); v != "" {
		cfg.Email.SMTPUsername = v
	}
	if v := os.Getenv("SKILL_REGISTRY_EMAIL_SMTP_PASSWORD"); v != "" {
		cfg.Email.SMTPPassword = v
	}
	if v := os.Getenv("SKILL_REGISTRY_EMAIL_FROM"); v != "" {
		cfg.Email.From = v
	}
	if v := os.Getenv("SKILL_REGISTRY_BASE_URL"); v != "" {
		cfg.Email.BaseURL = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("SKILL_REGISTRY_BACKUP_ENABLED"); v != "" {
		cfg.Backup.Enabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_BACKUP_OUTPUT_DIR"); v != "" {
		cfg.Backup.OutputDir = v
	}
	if v := os.Getenv("SKILL_REGISTRY_BACKUP_INTERVAL_MINUTES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Backup.IntervalMinutes = i
		}
	}
	if v := os.Getenv("SKILL_REGISTRY_BACKUP_RETENTION_COPIES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Backup.RetentionCopies = i
		}
	}
	if v := os.Getenv("SKILL_REGISTRY_PRODUCTION_MODE"); v != "" {
		cfg.Production.Mode = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_PRODUCTION_ENFORCE"); v != "" {
		cfg.Production.Enforce = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_TLS_ENABLED"); v != "" {
		cfg.TLS.Enabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_TLS_CERT_FILE"); v != "" {
		cfg.TLS.CertFile = v
	}
	if v := os.Getenv("SKILL_REGISTRY_TLS_KEY_FILE"); v != "" {
		cfg.TLS.KeyFile = v
	}
}

// Validate checks that required fields are populated and consistent.
func (cfg *Config) Validate() error {
	if cfg.Server.Addr == "" {
		return fmt.Errorf("server.addr must not be empty")
	}
	if cfg.Storage.DataDir == "" {
		return fmt.Errorf("storage.data_dir must not be empty")
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "sqlite"
	}
	if cfg.Database.Driver == "sqlite" && cfg.Database.Path == "" {
		return fmt.Errorf("database.path must not be empty")
	}
	if (cfg.Database.Driver == "postgres" || cfg.Database.Driver == "postgresql") && cfg.Database.DSN == "" {
		return fmt.Errorf("database.dsn must not be empty when database.driver is postgres")
	}
	if cfg.Storage.MaxPackageSizeMB <= 0 {
		return fmt.Errorf("storage.max_package_size_mb must be > 0")
	}
	if cfg.Auth.BcryptCost < 4 || cfg.Auth.BcryptCost > 31 {
		return fmt.Errorf("auth.bcrypt_cost must be between 4 and 31")
	}
	if cfg.Security.HSTSMaxAgeSeconds < 0 {
		return fmt.Errorf("security.hsts_max_age_seconds must be >= 0")
	}
	for _, cidr := range cfg.Security.TrustedProxies {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("security.trusted_proxies: invalid CIDR %q: %w", cidr, err)
		}
	}
	if cfg.Auth.RequireEmailVerification && cfg.Email.SMTPEnabled && cfg.Email.SMTPHost == "" {
		return fmt.Errorf("email.smtp_host is required when SMTP email verification is enabled")
	}
	if cfg.Email.SMTPPort < 0 || cfg.Email.SMTPPort > 65535 {
		return fmt.Errorf("email.smtp_port must be between 0 and 65535")
	}
	if cfg.Backup.Enabled {
		if cfg.Backup.OutputDir == "" {
			return fmt.Errorf("backup.output_dir is required when backup.enabled is true")
		}
		if cfg.Backup.IntervalMinutes <= 0 {
			return fmt.Errorf("backup.interval_minutes must be > 0")
		}
		if cfg.Backup.RetentionCopies < 1 {
			return fmt.Errorf("backup.retention_copies must be >= 1")
		}
	}
	if cfg.TLS.Enabled {
		if cfg.TLS.CertFile == "" {
			return fmt.Errorf("tls.cert_file is required when tls.enabled is true")
		}
		if cfg.TLS.KeyFile == "" {
			return fmt.Errorf("tls.key_file is required when tls.enabled is true")
		}
		if _, err := os.Stat(cfg.TLS.CertFile); err != nil {
			return fmt.Errorf("tls.cert_file %q: %w", cfg.TLS.CertFile, err)
		}
		if _, err := os.Stat(cfg.TLS.KeyFile); err != nil {
			return fmt.Errorf("tls.key_file %q: %w", cfg.TLS.KeyFile, err)
		}
	}
	return nil
}

// ProductionIssues returns configuration gaps that matter for production operation.
func (cfg *Config) ProductionIssues() []string {
	var issues []string
	if !cfg.Auth.Enabled {
		issues = append(issues, "auth.enabled must be true")
	}
	if cfg.Auth.BcryptCost < 12 {
		issues = append(issues, "auth.bcrypt_cost should be at least 12")
	}
	if !cfg.Security.HeadersEnabled {
		issues = append(issues, "security.headers_enabled must be true")
	}
	if cfg.Security.OpenAPICORSOrigin == "*" {
		issues = append(issues, "security.openapi_cors_origin should be restricted from '*'")
	}
	if !cfg.TLS.Enabled && !cfg.TLS.HTTPRedirect {
		issues = append(issues, "tls.enabled or a trusted TLS-terminating proxy must be configured")
	}
	if !cfg.RateLimit.Enabled {
		issues = append(issues, "rate_limit.enabled must be true")
	}
	if !cfg.Backup.Enabled {
		issues = append(issues, "backup.enabled should be true with off-host synchronization")
	}
	if cfg.Auth.RequireEmailVerification && !cfg.Email.SMTPEnabled {
		issues = append(issues, "email.smtp_enabled must be true when email verification is required")
	}
	return issues
}

// ValidateProduction returns an error when production enforcement is enabled and gaps remain.
func (cfg *Config) ValidateProduction() error {
	if !cfg.Production.Mode && !cfg.Production.Enforce {
		return nil
	}
	issues := cfg.ProductionIssues()
	if cfg.Production.Enforce && len(issues) > 0 {
		return fmt.Errorf("production readiness check failed: %s", strings.Join(issues, "; "))
	}
	return nil
}

// ResolveTokens resolves token values from environment variables
func (cfg *Config) ResolveTokens() map[string][]string {
	tokens := make(map[string][]string)
	for _, entry := range cfg.Auth.Tokens {
		if entry.TokenEnv != "" {
			if token := os.Getenv(entry.TokenEnv); token != "" {
				tokens[token] = entry.Scopes
			}
		}
	}
	return tokens
}
