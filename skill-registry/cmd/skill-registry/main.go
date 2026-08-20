package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skillforge/skill-registry/internal/api"
	"github.com/skillforge/skill-registry/internal/audit"
	"github.com/skillforge/skill-registry/internal/auth"
	"github.com/skillforge/skill-registry/internal/backup"
	"github.com/skillforge/skill-registry/internal/config"
	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/observability"
	"github.com/skillforge/skill-registry/internal/ratelimit"
	"github.com/skillforge/skill-registry/internal/registry"
	"github.com/skillforge/skill-registry/internal/storage"
	"github.com/skillforge/skill-registry/internal/validation"
)

func main() {
	// Check for admin commands first
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		runAdminCommand()
		return
	}

	configPath := flag.String("config", "./config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}
	if err := cfg.ValidateProduction(); err != nil {
		log.Fatalf("Production readiness failed: %v", err)
	}

	// Initialize logger
	logger := observability.NewLogger()
	logger.Info("starting skill registry", "config_path", *configPath)

	// Initialize metadata repository
	repo, err := metadata.NewRepositoryWithConfig(cfg.Database.Driver, databaseDSN(cfg))
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// Initialize storage
	store, err := newStorageBackend(context.Background(), cfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize validator
	validator := validation.NewValidator(cfg.Storage.MaxPackageSizeMB, cfg.Validation.BlockedExtensions)

	// Initialize registry
	reg := registry.NewRegistry(repo, store, validator, logger)

	// Initialize authenticator
	authenticator, err := auth.NewAuthenticatorWithBcryptCost(cfg.Auth.Enabled, repo.GetDB(), cfg.Auth.BcryptCost)
	if err != nil {
		log.Fatalf("Failed to initialize authenticator: %v", err)
	}

	// Initialize audit log
	auditRepo, err := audit.NewRepository(repo.GetDB())
	if err != nil {
		log.Fatalf("Failed to initialize audit log: %v", err)
	}

	// Auto-create initial admin user from env vars (idempotent — skipped if user exists)
	if adminUser := os.Getenv("SKILL_REGISTRY_INITIAL_ADMIN_USER"); adminUser != "" {
		adminPass := os.Getenv("SKILL_REGISTRY_INITIAL_ADMIN_PASSWORD")
		if adminPass == "" {
			log.Fatalf("SKILL_REGISTRY_INITIAL_ADMIN_USER set but SKILL_REGISTRY_INITIAL_ADMIN_PASSWORD is empty")
		}
		userRepo := authenticator.GetUserRepo()
		if _, err := userRepo.GetUser(context.Background(), adminUser); err != nil {
			if _, err := userRepo.CreateUserWithOptions(context.Background(), auth.UserCreateOptions{
				Username:      adminUser,
				Email:         os.Getenv("SKILL_REGISTRY_INITIAL_ADMIN_EMAIL"),
				Password:      adminPass,
				Role:          "admin",
				EmailVerified: true,
			}); err != nil {
				logger.Error("failed to create initial admin user", "error", err)
			} else {
				logger.Info("created initial admin user", "username", adminUser)
			}
		}
	}

	// Initialize API handler
	handler := api.NewHandler(reg, authenticator, auditRepo, logger, cfg)

	var backupScheduler *backup.Scheduler
	if cfg.Backup.Enabled {
		if cfg.Database.Driver == "postgres" || cfg.Database.Driver == "postgresql" {
			logger.Info("database backup scheduler skipped for postgres; use pg_dump or managed database backups", "driver", cfg.Database.Driver)
		} else {
			backupScheduler = backup.NewScheduler(
				databaseDSN(cfg),
				cfg.Storage.DataDir,
				cfg.Backup.OutputDir,
				time.Duration(cfg.Backup.IntervalMinutes)*time.Minute,
				cfg.Backup.RetentionCopies,
				logger,
			)
			backupScheduler.Start()
			logger.Info("scheduled backups enabled", "output_dir", cfg.Backup.OutputDir, "interval_minutes", cfg.Backup.IntervalMinutes, "retention_copies", cfg.Backup.RetentionCopies)
		}
	}

	// Setup router
	r := chi.NewRouter()
	r.Use(panicRecovery(logger))
	r.Use(observability.RequestIDMiddleware)
	r.Use(observability.MetricsMiddleware)
	r.Use(observability.LoggingMiddleware(logger))
	if cfg.Security.HeadersEnabled {
		r.Use(observability.SecurityHeadersMiddleware(cfg.Security))
	}
	if cfg.RateLimit.Enabled {
		limiter := ratelimit.New(cfg.RateLimit.RequestsPerMinute, time.Minute, cfg.Security.TrustedProxies)
		r.Use(ratelimit.Middleware(limiter))
	}

	handler.RegisterRoutes(r)

	// Create HTTP server
	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("skill registry listening", "addr", cfg.Server.Addr, "tls", cfg.TLS.Enabled)
		var serveErr error
		if cfg.TLS.Enabled {
			serveErr = srv.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		} else {
			serveErr = srv.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", serveErr)
		}
	}()

	// Optional HTTP→HTTPS redirect listener
	if cfg.TLS.Enabled && cfg.TLS.HTTPRedirect {
		httpAddr := cfg.TLS.HTTPAddr
		if httpAddr == "" {
			httpAddr = ":80"
		}
		go func() {
			logger.Info("HTTP redirect listener", "addr", httpAddr)
			redirect := &http.Server{
				Addr:              httpAddr,
				ReadHeaderTimeout: 10 * time.Second,
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					target := "https://" + r.Host + r.URL.RequestURI()
					http.Redirect(w, r, target, http.StatusMovedPermanently)
				}),
			}
			if err := redirect.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("HTTP redirect listener failed", "error", err)
			}
		}()
	}

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}
	if backupScheduler != nil {
		if err := backupScheduler.Stop(ctx); err != nil {
			logger.Error("backup scheduler shutdown failed", "error", err)
		}
	}

	logger.Info("server stopped")
}

func panicRecovery(logger interface{ Error(string, ...any) }) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rv := recover(); rv != nil {
					logger.Error("panic recovered", "error", rv, "path", r.URL.Path)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func runAdminCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  skill-registry admin create-user --username <user> --password <pass> --role <role> [--email <email>] [--email-verified] [--bcrypt-cost <cost>]")
		fmt.Println("  skill-registry admin backup --output <dir> [--db <path>] [--data-dir <dir>]")
		fmt.Println("  skill-registry admin production-check [--config <path>]")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "create-user":
		createUserCommand()
	case "backup":
		backupCommand()
	case "production-check":
		productionCheckCommand()
	default:
		fmt.Fprintf(os.Stderr, "Unknown admin command: %s\n", subcommand)
		os.Exit(1)
	}
}

func productionCheckCommand() {
	configPath := "./config.yaml"
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--config" && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
			i++
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		os.Exit(1)
	}
	issues := cfg.ProductionIssues()
	if len(issues) == 0 {
		fmt.Println("Production readiness check passed")
		return
	}
	fmt.Println("Production readiness issues:")
	for _, issue := range issues {
		fmt.Printf(" - %s\n", issue)
	}
	os.Exit(1)
}

func createUserCommand() {
	var username, password, role, email, dbDriver, dbDSN string
	var dbPath string
	bcryptCost := 10
	emailVerified := false

	// Parse flags
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--username" && i+1 < len(os.Args) {
			username = os.Args[i+1]
			i++
		} else if os.Args[i] == "--password" && i+1 < len(os.Args) {
			password = os.Args[i+1]
			i++
		} else if os.Args[i] == "--role" && i+1 < len(os.Args) {
			role = os.Args[i+1]
			i++
		} else if os.Args[i] == "--db" && i+1 < len(os.Args) {
			dbPath = os.Args[i+1]
			i++
		} else if os.Args[i] == "--db-driver" && i+1 < len(os.Args) {
			dbDriver = os.Args[i+1]
			i++
		} else if os.Args[i] == "--db-dsn" && i+1 < len(os.Args) {
			dbDSN = os.Args[i+1]
			i++
		} else if os.Args[i] == "--email" && i+1 < len(os.Args) {
			email = os.Args[i+1]
			i++
		} else if os.Args[i] == "--email-verified" {
			emailVerified = true
		} else if os.Args[i] == "--bcrypt-cost" && i+1 < len(os.Args) {
			if parsed, err := strconv.Atoi(os.Args[i+1]); err == nil {
				bcryptCost = parsed
			}
			i++
		}
	}

	if username == "" {
		fmt.Fprintln(os.Stderr, "Error: --username is required")
		os.Exit(1)
	}

	if password == "" {
		fmt.Fprintln(os.Stderr, "Error: --password is required")
		os.Exit(1)
	}

	if role == "" {
		role = "user"
	}

	if role != "user" && role != "admin" {
		fmt.Fprintf(os.Stderr, "Error: role must be 'user' or 'admin', got '%s'\n", role)
		os.Exit(1)
	}

	if dbPath == "" {
		dbPath = "./data/registry.db"
	}
	if dbDriver == "" {
		dbDriver = "sqlite"
	}
	if dbDSN == "" {
		dbDSN = dbPath
	}

	// Initialize database
	repo, err := metadata.NewRepositoryWithConfig(dbDriver, dbDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	// Create user repository
	userRepo, err := auth.NewUserRepositoryWithBcryptCost(repo.GetDB(), bcryptCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize user repository: %v\n", err)
		os.Exit(1)
	}

	// Create user
	user, err := userRepo.CreateUserWithOptions(context.Background(), auth.UserCreateOptions{
		Username:      username,
		Email:         email,
		Password:      password,
		Role:          role,
		EmailVerified: emailVerified || role == "admin",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ User created successfully!\n")
	fmt.Printf("   Username: %s\n", user.Username)
	if user.Email != "" {
		fmt.Printf("   Email: %s\n", user.Email)
		fmt.Printf("   Email verified: %t\n", user.EmailVerified)
	}
	fmt.Printf("   Role: %s\n", user.Role)
	fmt.Printf("   ID: %d\n", user.ID)
}

func databaseDSN(cfg *config.Config) string {
	if cfg.Database.Driver == "postgres" || cfg.Database.Driver == "postgresql" {
		return cfg.Database.DSN
	}
	return cfg.Database.Path
}

// newStorageBackend constructs the storage.Backend selected by
// cfg.Storage.Backend. Kept here rather than in internal/storage itself so
// that package doesn't need to import internal/config — config describes
// the app's settings, storage shouldn't need to know the whole app's
// config shape just to pick a backend.
func newStorageBackend(ctx context.Context, cfg *config.Config) (storage.Backend, error) {
	switch cfg.Storage.Backend {
	case "", "filesystem":
		return storage.NewStorage(cfg.Storage.DataDir)
	case "s3":
		return storage.NewS3Storage(ctx,
			cfg.Storage.S3.Endpoint,
			cfg.Storage.S3.Region,
			cfg.Storage.S3.Bucket,
			cfg.Storage.S3.AccessKey,
			cfg.Storage.S3.SecretKey,
			cfg.Storage.S3.UseSSL,
			cfg.Storage.S3.PathStyle,
		)
	default:
		// cfg.Validate() should have already rejected this, but don't
		// silently fall back to filesystem if it somehow wasn't called.
		return nil, fmt.Errorf("unknown storage.backend %q", cfg.Storage.Backend)
	}
}

func backupCommand() {
	dbPath := "./data/registry.db"
	dataDir := "./data"
	output := ""
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--db" && i+1 < len(os.Args) {
			dbPath = os.Args[i+1]
			i++
		} else if os.Args[i] == "--data-dir" && i+1 < len(os.Args) {
			dataDir = os.Args[i+1]
			i++
		} else if os.Args[i] == "--output" && i+1 < len(os.Args) {
			output = os.Args[i+1]
			i++
		}
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "Error: --output is required")
		os.Exit(1)
	}
	manifest, err := backup.Create(context.Background(), dbPath, dataDir, output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Backup failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Backup created successfully\n")
	fmt.Printf("   Output: %s\n", output)
	fmt.Printf("   Database: %s\n", manifest.Database)
	fmt.Printf("   Storage: %s\n", manifest.StorageDir)
	fmt.Printf("   Created at: %s\n", manifest.CreatedAt.Format(time.RFC3339))
}
