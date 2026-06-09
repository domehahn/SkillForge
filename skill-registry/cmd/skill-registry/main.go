package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skillforge/skill-registry/internal/api"
	"github.com/skillforge/skill-registry/internal/audit"
	"github.com/skillforge/skill-registry/internal/auth"
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

	// Initialize logger
	logger := observability.NewLogger()
	logger.Info("starting skill registry", "config_path", *configPath)

	// Initialize metadata repository
	repo, err := metadata.NewRepository(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// Initialize storage
	store, err := storage.NewStorage(cfg.Storage.DataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize validator
	validator := validation.NewValidator(cfg.Storage.MaxPackageSizeMB, cfg.Validation.BlockedExtensions)

	// Initialize registry
	reg := registry.NewRegistry(repo, store, validator, logger)

	// Initialize authenticator
	authenticator, err := auth.NewAuthenticator(cfg.Auth.Enabled, repo.GetDB())
	if err != nil {
		log.Fatalf("Failed to initialize authenticator: %v", err)
	}

	// Initialize audit log
	auditRepo, err := audit.NewRepository(repo.GetDB())
	if err != nil {
		log.Fatalf("Failed to initialize audit log: %v", err)
	}

	// Initialize API handler
	handler := api.NewHandler(reg, authenticator, auditRepo, logger, cfg)

	// Setup router
	r := chi.NewRouter()
	r.Use(observability.RequestIDMiddleware)
	r.Use(observability.LoggingMiddleware(logger))
	if cfg.RateLimit.Enabled {
		limiter := ratelimit.New(cfg.RateLimit.RequestsPerMinute, time.Minute)
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
		logger.Info("skill registry listening", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

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

	logger.Info("server stopped")
}

func runAdminCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  skill-registry admin create-user --username <user> --password <pass> --role <role>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "create-user":
		createUserCommand()
	default:
		fmt.Fprintf(os.Stderr, "Unknown admin command: %s\n", subcommand)
		os.Exit(1)
	}
}

func createUserCommand() {
	var username, password, role string
	var dbPath string

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

	// Initialize database
	repo, err := metadata.NewRepository(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	// Create user repository
	userRepo, err := auth.NewUserRepository(repo.GetDB())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize user repository: %v\n", err)
		os.Exit(1)
	}

	// Create user
	user, err := userRepo.CreateUser(context.Background(), username, password, role)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ User created successfully!\n")
	fmt.Printf("   Username: %s\n", user.Username)
	fmt.Printf("   Role: %s\n", user.Role)
	fmt.Printf("   ID: %d\n", user.ID)
}
