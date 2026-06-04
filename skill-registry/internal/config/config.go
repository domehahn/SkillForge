package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the registry configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Storage    StorageConfig    `yaml:"storage"`
	Database   DatabaseConfig   `yaml:"database"`
	Auth       AuthConfig       `yaml:"auth"`
	Proxy      ProxyConfig      `yaml:"proxy"`
	Validation ValidationConfig `yaml:"validation"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Addr string `yaml:"addr"`
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	DataDir              string   `yaml:"data_dir"`
	MaxPackageSizeMB     int      `yaml:"max_package_size_mb"`
	AllowedPackageTypes  []string `yaml:"allowed_package_types"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled bool         `yaml:"enabled"`
	Tokens  []TokenEntry `yaml:"tokens"`
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
		Storage: StorageConfig{
			DataDir:             "./data",
			MaxPackageSizeMB:    50,
			AllowedPackageTypes: []string{"tgz", "zip"},
		},
		Database: DatabaseConfig{
			Path: "./data/registry.db",
		},
		Auth: AuthConfig{
			Enabled: false,
			Tokens:  []TokenEntry{},
		},
		Proxy: ProxyConfig{
			Enabled:   false,
			Upstreams: []string{},
		},
		Validation: ValidationConfig{
			BlockedExtensions: []string{".exe", ".dll", ".so", ".dylib", ".bin"},
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
	if v := os.Getenv("SKILL_REGISTRY_MAX_PACKAGE_SIZE_MB"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Storage.MaxPackageSizeMB = i
		}
	}
	if v := os.Getenv("SKILL_REGISTRY_AUTH_ENABLED"); v != "" {
		cfg.Auth.Enabled = strings.ToLower(v) == "true" || v == "1"
	}
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
