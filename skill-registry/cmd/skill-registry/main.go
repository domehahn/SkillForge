package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skillforge/skill-registry/internal/api"
	"github.com/skillforge/skill-registry/internal/auth"
	"github.com/skillforge/skill-registry/internal/config"
	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/observability"
	"github.com/skillforge/skill-registry/internal/registry"
	"github.com/skillforge/skill-registry/internal/storage"
	"github.com/skillforge/skill-registry/internal/validation"
)

func main() {
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
	tokens := cfg.ResolveTokens()
	authenticator := auth.NewAuthenticator(cfg.Auth.Enabled, tokens)

	// Initialize API handler
	handler := api.NewHandler(reg, authenticator, logger, cfg)

	// Setup router
	r := chi.NewRouter()
	r.Use(observability.RequestIDMiddleware)
	r.Use(observability.LoggingMiddleware(logger))

	handler.RegisterRoutes(r)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
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
