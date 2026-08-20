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
	// Backend selects the counting strategy: "memory" (default — a single
	// instance's own counters, not shared with other replicas) or "redis"
	// (shared across every replica pointed at the same Redis instance —
	// required for the limit to actually hold once more than one registry
	// instance is running behind a load balancer).
	Backend string      `yaml:"backend"`
	Redis   RedisConfig `yaml:"redis"`
}

// RedisConfig configures the Redis-backed rate limiter. Only read when
// rate_limit.backend is "redis".
type RedisConfig struct {
	Addr     string `yaml:"addr"` // e.g. "localhost:6379"
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	// KeyPrefix namespaces this limiter's keys, useful if the same Redis
	// instance is shared with other data.
	KeyPrefix string `yaml:"key_prefix"`
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
	// Backend selects the blob storage implementation: "filesystem" (the
	// default — a local directory, fine for a single node) or "s3" (an
	// S3-compatible object store, required for running more than one
	// registry instance against the same artifact set, since a local
	// filesystem store is not shared across pods/hosts).
	Backend             string   `yaml:"backend"`
	DataDir             string   `yaml:"data_dir"`
	MaxPackageSizeMB    int      `yaml:"max_package_size_mb"`
	AllowedPackageTypes []string `yaml:"allowed_package_types"`
	S3                  S3Config `yaml:"s3"`
}

// S3Config configures the S3-compatible object storage backend. Only read
// when storage.backend is "s3".
type S3Config struct {
	Endpoint  string `yaml:"endpoint"` // e.g. "s3.amazonaws.com" or "minio.internal:9000"
	Region    string `yaml:"region"`   // e.g. "us-east-1"; some S3-compatible backends ignore this
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
	// PathStyle forces path-style addressing (https://host/bucket/key)
	// instead of virtual-hosted-style (https://bucket.host/key) — needed
	// for most non-AWS S3-compatible backends (MinIO, etc).
	PathStyle bool `yaml:"path_style"`
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
			Backend:             "filesystem",
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
			Backend:           "memory",
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
	if v := os.Getenv("SKILL_REGISTRY_STORAGE_BACKEND"); v != "" {
		cfg.Storage.Backend = v
	}
	if v := os.Getenv("SKILL_REGISTRY_S3_ENDPOINT"); v != "" {
		cfg.Storage.S3.Endpoint = v
	}
	if v := os.Getenv("SKILL_REGISTRY_S3_REGION"); v != "" {
		cfg.Storage.S3.Region = v
	}
	if v := os.Getenv("SKILL_REGISTRY_S3_BUCKET"); v != "" {
		cfg.Storage.S3.Bucket = v
	}
	if v := os.Getenv("SKILL_REGISTRY_S3_ACCESS_KEY"); v != "" {
		cfg.Storage.S3.AccessKey = v
	}
	if v := os.Getenv("SKILL_REGISTRY_S3_SECRET_KEY"); v != "" {
		cfg.Storage.S3.SecretKey = v
	}
	if v := os.Getenv("SKILL_REGISTRY_S3_USE_SSL"); v != "" {
		cfg.Storage.S3.UseSSL = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("SKILL_REGISTRY_S3_PATH_STYLE"); v != "" {
		cfg.Storage.S3.PathStyle = strings.ToLower(v) == "true" || v == "1"
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
	if v := os.Getenv("SKILL_REGISTRY_RATE_LIMIT_BACKEND"); v != "" {
		cfg.RateLimit.Backend = v
	}
	if v := os.Getenv("SKILL_REGISTRY_REDIS_ADDR"); v != "" {
		cfg.RateLimit.Redis.Addr = v
	}
	if v := os.Getenv("SKILL_REGISTRY_REDIS_PASSWORD"); v != "" {
		cfg.RateLimit.Redis.Password = v
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
	if cfg.Storage.Backend == "" {
		cfg.Storage.Backend = "filesystem"
	}
	if cfg.Storage.Backend != "filesystem" && cfg.Storage.Backend != "s3" {
		return fmt.Errorf("storage.backend must be \"filesystem\" or \"s3\", got %q", cfg.Storage.Backend)
	}
	if cfg.Storage.Backend == "s3" {
		if cfg.Storage.S3.Bucket == "" {
			return fmt.Errorf("storage.s3.bucket is required when storage.backend is \"s3\"")
		}
		if cfg.Storage.S3.Endpoint == "" {
			return fmt.Errorf("storage.s3.endpoint is required when storage.backend is \"s3\"")
		}
	}
	if cfg.RateLimit.Backend == "" {
		cfg.RateLimit.Backend = "memory"
	}
	if cfg.RateLimit.Backend != "memory" && cfg.RateLimit.Backend != "redis" {
		return fmt.Errorf("rate_limit.backend must be \"memory\" or \"redis\", got %q", cfg.RateLimit.Backend)
	}
	if cfg.RateLimit.Enabled && cfg.RateLimit.Backend == "redis" && cfg.RateLimit.Redis.Addr == "" {
		return fmt.Errorf("rate_limit.redis.addr is required when rate_limit.backend is \"redis\"")
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
