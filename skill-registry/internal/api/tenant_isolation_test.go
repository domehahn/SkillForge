package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
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

func setupTenantIsolationServer(t *testing.T) (*chi.Mux, *metadata.Repository, string, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "tenant.db")
	repo, err := metadata.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	store, err := storage.NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	validator := validation.NewValidator(50, []string{".exe"})
	logger := observability.NewLogger()
	reg := registry.NewRegistry(repo, store, validator, logger)

	authenticator, err := auth.NewAuthenticator(true, repo.GetDB())
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}
	userRepo := authenticator.GetUserRepo()

	cfg := config.DefaultConfig()
	cfg.Auth.Enabled = true

	handler := api.NewHandler(reg, authenticator, nil, logger, cfg)

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Create Tenant A user and token
	ctx := context.Background()
	userA, err := userRepo.CreateUserWithOptions(ctx, auth.UserCreateOptions{
		Username: "tenant-a-user", Email: "a@example.com", Password: "Password123!", Role: "user", EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("failed to create user A: %v", err)
	}
	tokenA, err := userRepo.CreateToken(ctx, userA.ID, "token-a", []string{"read", "write"}, nil)
	if err != nil {
		t.Fatalf("failed to create token A: %v", err)
	}

	// Create Tenant B user and token
	userB, err := userRepo.CreateUserWithOptions(ctx, auth.UserCreateOptions{
		Username: "tenant-b-user", Email: "b@example.com", Password: "Password123!", Role: "user", EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("failed to create user B: %v", err)
	}
	tokenB, err := userRepo.CreateToken(ctx, userB.ID, "token-b", []string{"read", "write"}, nil)
	if err != nil {
		t.Fatalf("failed to create token B: %v", err)
	}

	// Setup Tenant B namespace members
	if err := repo.UpsertNamespaceMember(ctx, &metadata.NamespaceMember{Namespace: "tenant-b", Username: "tenant-b-user", Role: "owner"}); err != nil {
		t.Fatalf("failed to set namespace B owner: %v", err)
	}
	if err := repo.UpsertNamespaceMember(ctx, &metadata.NamespaceMember{Namespace: "tenant-a", Username: "tenant-a-user", Role: "owner"}); err != nil {
		t.Fatalf("failed to set namespace A owner: %v", err)
	}

	// Publish a private artifact in Tenant B namespace
	bData := []byte("tenant b private data")
	digest, err := store.StoreArtifact("skill", "tenant-b", "private-skill", "1.0.0", bData)
	if err != nil {
		t.Fatalf("failed to store tenant B artifact: %v", err)
	}

	bArtifact := &metadata.Artifact{
		Kind: "skill", Namespace: "tenant-b", Name: "private-skill", Description: "Private skill B",
		LatestVersion: "1.0.0", Visibility: "private",
	}
	_ = repo.CreateArtifact(ctx, bArtifact)

	bVersion := &metadata.ArtifactVersion{
		ArtifactID: bArtifact.ID, Kind: "skill", Namespace: "tenant-b", Name: "private-skill", Version: "1.0.0",
		DigestSHA256: digest, PackageType: "tgz", SizeBytes: int64(len(bData)), Entrypoint: "SKILL.md",
		Manifest:        &metadata.ArtifactManifest{APIVersion: "skillforge.dev/v1", Kind: "skill", Metadata: metadata.ArtifactMetadata{Namespace: "tenant-b", Name: "private-skill", Version: "1.0.0"}},
		OCIDescriptor:   &metadata.OCIDescriptor{MediaType: "application/vnd.skillforge.artifact.manifest.v1+json", Digest: "sha256:" + digest, Size: int64(len(bData)), ArtifactType: "application/vnd.skillforge.skill.v1"},
		SignatureStatus: "unsigned", ScanStatus: "pending", ValidationStatus: "valid", Source: "local", CreatedBy: "tenant-b-user",
		CreatedAt: time.Now(),
	}
	_ = repo.CreateArtifactVersion(ctx, bVersion)

	return router, repo, tokenA.Token, tokenB.Token
}

func TestTenantIsolation_TenantACannotReadTenantBPrivateArtifact(t *testing.T) {
	router, _, tokenA, _ := setupTenantIsolationServer(t)

	req := httptest.NewRequest("GET", "/api/v1/artifacts/skill/tenant-b/private-skill", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("expected 403 or 404 when Tenant A reads Tenant B private artifact, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantIsolation_TenantACannotPublishIntoTenantBNamespace(t *testing.T) {
	router, _, tokenA, _ := setupTenantIsolationServer(t)

	req := httptest.NewRequest("PUT", "/api/v1/artifacts/skill/tenant-b/hacked-skill/versions/1.0.0", bytes.NewReader([]byte("hacked")))
	req.Header.Set("Authorization", "Bearer "+tokenA)
	req.Header.Set("Content-Type", "application/gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when Tenant A publishes into Tenant B namespace, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantIsolation_TenantACannotModifyTenantBGovernance(t *testing.T) {
	router, _, tokenA, _ := setupTenantIsolationServer(t)

	yankBody, _ := json.Marshal(map[string]string{"reason": "malicious yank"})
	req := httptest.NewRequest("POST", "/api/v1/skills/tenant-b/private-skill/versions/1.0.0/yank", bytes.NewReader(yankBody))
	req.Header.Set("Authorization", "Bearer "+tokenA)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when Tenant A yanks Tenant B artifact, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantIsolation_TenantACannotAttachAttestationToTenantB(t *testing.T) {
	router, _, tokenA, _ := setupTenantIsolationServer(t)

	attestBody, _ := json.Marshal(map[string]interface{}{
		"type":      "scan",
		"digest":    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"predicate": map[string]interface{}{"status": "passed"},
	})
	req := httptest.NewRequest("POST", "/api/v1/artifacts/skill/tenant-b/private-skill/versions/1.0.0/attestations", bytes.NewReader(attestBody))
	req.Header.Set("Authorization", "Bearer "+tokenA)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when Tenant A attaches attestation to Tenant B artifact, got %d: %s", rec.Code, rec.Body.String())
	}
}
