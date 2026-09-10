package tests

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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

func buildTestPackage(namespace, name, version string) ([]byte, error) {
	skillMD := "# " + name + "\n"
	sum := sha256.Sum256([]byte(skillMD))
	files := map[string]string{
		"SKILL.md": skillMD,
		"VERSION":  version + "\n",
		"skill.yaml": fmt.Sprintf(
			"name: %s\nnamespace: %s\nversion: %s\ndescription: test\nowners:\n  - test\n"+
				"license: MIT\ncompatible_with:\n  - codex\nentrypoint: SKILL.md\n"+
				"security:\n  requires_network: false\n  requires_secrets: false\n"+
				"  writes_files: false\n  runs_commands: false\n",
			name, namespace, version),
		"CHANGELOG.md": fmt.Sprintf("# Changelog\n\n## %s\n\n### Added\n- initial\n", version),
		"manifest.json": fmt.Sprintf(
			`{"spec_version":1,"name":%q,"namespace":%q,"version":%q,"entrypoint":"SKILL.md",`+
				`"compatible_with":["codex"],"package_type":"tgz","packaged_by":"skforge","packaged_at":"1970-01-01T00:00:00Z"}`,
			name, namespace, version),
		"checksums.txt": hex.EncodeToString(sum[:]) + "  SKILL.md\n",
	}
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for fname, content := range files {
		hdr := &tar.Header{Name: fname, Mode: 0644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// TestHA_MultiReplicaConsistency performs an automated HA multi-replica test across 3 registry nodes (A, B, C)
// sharing the same database and storage backends.
// Sequence:
// 1. Publish artifact through Replica A
// 2. Resolve artifact through Replica B
// 3. Download package through Replica C
// 4. Attach attestation via Replica B
// 5. Yank artifact via Replica A
// 6. Query artifact via Replica C
func TestHA_MultiReplicaConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "ha_shared.db")

	// Shared persistence
	repo, err := metadata.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to open shared DB: %v", err)
	}
	defer repo.Close()

	store, err := storage.NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to open shared storage: %v", err)
	}

	logger := observability.NewLogger()
	validator := validation.NewValidator(50, []string{".exe"})
	authenticator, err := auth.NewAuthenticator(true, repo.GetDB())
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Auth.Enabled = true

	// Create 3 replica handlers sharing the exact same DB & Storage backend
	regA := registry.NewRegistry(repo, store, validator, logger)
	handlerA := api.NewHandler(regA, authenticator, nil, logger, cfg)
	routerA := chi.NewRouter()
	handlerA.RegisterRoutes(routerA)
	serverA := httptest.NewServer(routerA)
	defer serverA.Close()

	regB := registry.NewRegistry(repo, store, validator, logger)
	handlerB := api.NewHandler(regB, authenticator, nil, logger, cfg)
	routerB := chi.NewRouter()
	handlerB.RegisterRoutes(routerB)
	serverB := httptest.NewServer(routerB)
	defer serverB.Close()

	regC := registry.NewRegistry(repo, store, validator, logger)
	handlerC := api.NewHandler(regC, authenticator, nil, logger, cfg)
	routerC := chi.NewRouter()
	handlerC.RegisterRoutes(routerC)
	serverC := httptest.NewServer(routerC)
	defer serverC.Close()

	// Create admin user and token
	ctx := context.Background()
	adminUser, err := authenticator.GetUserRepo().CreateUserWithOptions(ctx, auth.UserCreateOptions{
		Username: "ha-admin", Email: "admin@ha.test", Password: "Password123!", Role: "admin", EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("failed to create ha-admin: %v", err)
	}
	token, err := authenticator.GetUserRepo().CreateToken(ctx, adminUser.ID, "ha-token", []string{"read", "write", "admin"}, nil)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Step 1: Publish through Replica A
	pkgData, err := buildTestPackage("ha-ns", "ha-skill", "1.0.0")
	if err != nil {
		t.Fatalf("failed to build test package: %v", err)
	}

	pubReq, _ := http.NewRequest("PUT", serverA.URL+"/api/v1/skills/ha-ns/ha-skill/versions/1.0.0", bytes.NewReader(pkgData))
	pubReq.Header.Set("Content-Type", "application/gzip")
	pubReq.Header.Set("Authorization", "Bearer "+token.Token)
	pubResp, err := serverA.Client().Do(pubReq)
	if err != nil {
		t.Fatalf("Step 1 (Publish via A) failed: %v", err)
	}
	if pubResp.StatusCode != http.StatusCreated {
		t.Fatalf("Step 1 (Publish via A) failed, status: %v", pubResp.Status)
	}
	pubResp.Body.Close()

	// Step 2: Resolve through Replica B
	resolveReq, _ := http.NewRequest("GET", serverB.URL+"/api/v1/skills/ha-ns/ha-skill/resolve?constraint=^1.0.0", nil)
	resolveReq.Header.Set("Authorization", "Bearer "+token.Token)
	resolveResp, err := serverB.Client().Do(resolveReq)
	if err != nil {
		t.Fatalf("Step 2 (Resolve via B) failed: %v", err)
	}
	if resolveResp.StatusCode != http.StatusOK {
		t.Fatalf("Step 2 (Resolve via B) failed, status: %v", resolveResp.Status)
	}
	var resolveDTO struct {
		Version string `json:"version"`
		SHA256  string `json:"sha256"`
	}
	_ = json.NewDecoder(resolveResp.Body).Decode(&resolveDTO)
	resolveResp.Body.Close()
	if resolveDTO.Version != "1.0.0" || resolveDTO.SHA256 == "" {
		t.Fatalf("Step 2 (Resolve via B) invalid resolution output: %+v", resolveDTO)
	}

	// Step 3: Download through Replica C
	dlReq, _ := http.NewRequest("GET", serverC.URL+"/api/v1/skills/ha-ns/ha-skill/versions/1.0.0/download", nil)
	dlReq.Header.Set("Authorization", "Bearer "+token.Token)
	dlResp, err := serverC.Client().Do(dlReq)
	if err != nil {
		t.Fatalf("Step 3 (Download via C) failed: %v", err)
	}
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("Step 3 (Download via C) failed, status: %v", dlResp.Status)
	}
	dlData, err := io.ReadAll(dlResp.Body)
	dlResp.Body.Close()
	if err != nil || len(dlData) == 0 {
		t.Fatalf("Step 3 (Download via C) empty payload")
	}

	// Step 4: Attach attestation via Replica B
	attestBody, _ := json.Marshal(map[string]interface{}{
		"type":      "scan",
		"digest":    resolveDTO.SHA256,
		"predicate": map[string]interface{}{"status": "passed", "version": 1, "subject": map[string]interface{}{"sha256": resolveDTO.SHA256}},
	})
	attestReq, _ := http.NewRequest("PUT", serverB.URL+"/api/v1/artifacts/skill/ha-ns/ha-skill/versions/1.0.0/attestations", bytes.NewReader(attestBody))
	attestReq.Header.Set("Content-Type", "application/json")
	attestReq.Header.Set("Authorization", "Bearer "+token.Token)
	attestResp, err := serverB.Client().Do(attestReq)
	if err != nil {
		t.Fatalf("Step 4 (Attestation via B) failed: %v", err)
	}
	if attestResp.StatusCode != http.StatusOK && attestResp.StatusCode != http.StatusCreated {
		t.Fatalf("Step 4 (Attestation via B) failed, status: %v", attestResp.Status)
	}
	attestResp.Body.Close()

	// Step 5: Yank via Replica A
	yankBody, _ := json.Marshal(map[string]string{"reason": "HA test revocation"})
	yankReq, _ := http.NewRequest("POST", serverA.URL+"/api/v1/skills/ha-ns/ha-skill/versions/1.0.0/yank", bytes.NewReader(yankBody))
	yankReq.Header.Set("Content-Type", "application/json")
	yankReq.Header.Set("Authorization", "Bearer "+token.Token)
	yankResp, err := serverA.Client().Do(yankReq)
	if err != nil {
		t.Fatalf("Step 5 (Yank via A) failed: %v", err)
	}
	if yankResp.StatusCode != http.StatusOK {
		t.Fatalf("Step 5 (Yank via A) failed, status: %v", yankResp.Status)
	}
	yankResp.Body.Close()

	// Step 6: Query via Replica C (Resolution should now fail / return 404 because version is yanked)
	queryReq, _ := http.NewRequest("GET", serverC.URL+"/api/v1/skills/ha-ns/ha-skill/resolve?constraint=^1.0.0", nil)
	queryReq.Header.Set("Authorization", "Bearer "+token.Token)
	queryResp, err := serverC.Client().Do(queryReq)
	if err != nil {
		t.Fatalf("Step 6 (Query via C after Yank) failed: %v", err)
	}
	if queryResp.StatusCode != http.StatusNotFound {
		t.Fatalf("Step 6 (Query via C after Yank) expected 404 NotFound, got status: %v", queryResp.Status)
	}
	queryResp.Body.Close()
}
