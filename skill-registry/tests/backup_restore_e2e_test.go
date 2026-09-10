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
	"os"
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

// TestBackupAndRestore_FullLifecycle performs a complete automated backup & restore test:
// 1. Publish artifact
// 2. Create namespace member (ACL)
// 3. Create API token
// 4. Attach attestation
// 5. Yank artifact
// 6. Backup database + blobs
// 7. Destroy original persistence
// 8. Restore from backup
// 9. Restart registry on restored persistence
// 10. Verify namespace, ACL, token, attestation, and yanked status are intact.
func TestBackupAndRestore_FullLifecycle(t *testing.T) {
	srcDir := t.TempDir()
	dbPath := filepath.Join(srcDir, "registry.db")

	repo, err := metadata.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	store, err := storage.NewStorage(srcDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	logger := observability.NewLogger()
	validator := validation.NewValidator(50, []string{".exe"})
	authenticator, err := auth.NewAuthenticator(true, repo.GetDB())
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Auth.Enabled = true

	reg := registry.NewRegistry(repo, store, validator, logger)
	handler := api.NewHandler(reg, authenticator, nil, logger, cfg)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	srv := httptest.NewServer(router)

	ctx := context.Background()
	user, err := authenticator.GetUserRepo().CreateUserWithOptions(ctx, auth.UserCreateOptions{
		Username: "backup-user", Email: "backup@example.com", Password: "Password123!", Role: "user", EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. Create Token
	token, err := authenticator.GetUserRepo().CreateToken(ctx, user.ID, "backup-token", []string{"read", "write"}, nil)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// 2. Create ACL
	if err := repo.UpsertNamespaceMember(ctx, &metadata.NamespaceMember{Namespace: "br-ns", Username: "backup-user", Role: "owner"}); err != nil {
		t.Fatalf("failed to create ACL: %v", err)
	}

	// 3. Publish artifact
	pkgData, err := buildTestTgz("br-ns", "br-skill", "1.0.0")
	if err != nil {
		t.Fatalf("failed to build package: %v", err)
	}

	pubReq, _ := http.NewRequest("PUT", srv.URL+"/api/v1/skills/br-ns/br-skill/versions/1.0.0", bytes.NewReader(pkgData))
	pubReq.Header.Set("Content-Type", "application/gzip")
	pubReq.Header.Set("Authorization", "Bearer "+token.Token)
	pubResp, err := http.DefaultClient.Do(pubReq)
	if err != nil || pubResp.StatusCode != http.StatusCreated {
		t.Fatalf("publish failed: status=%v, err=%v", pubResp.Status, err)
	}
	pubResp.Body.Close()

	// Retrieve digest
	v, err := reg.GetVersion(ctx, "br-ns", "br-skill", "1.0.0")
	if err != nil || v == nil {
		t.Fatalf("failed to get published version: %v", err)
	}
	digest := v.DigestSHA256

	// 4. Attach attestation
	attestBody, _ := json.Marshal(map[string]interface{}{
		"type":      "scan",
		"digest":    digest,
		"predicate": map[string]interface{}{"status": "passed", "version": 1, "subject": map[string]interface{}{"sha256": digest}},
	})
	attestReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/artifacts/skill/br-ns/br-skill/versions/1.0.0/attestations", bytes.NewReader(attestBody))
	attestReq.Header.Set("Content-Type", "application/json")
	attestReq.Header.Set("Authorization", "Bearer "+token.Token)
	attestResp, err := http.DefaultClient.Do(attestReq)
	if err != nil || (attestResp.StatusCode != http.StatusOK && attestResp.StatusCode != http.StatusCreated) {
		t.Fatalf("attestation failed: status=%v, err=%v", attestResp.Status, err)
	}
	attestResp.Body.Close()

	// 5. Yank artifact
	yankBody, _ := json.Marshal(map[string]string{"reason": "backup test yank"})
	yankReq, _ := http.NewRequest("POST", srv.URL+"/api/v1/skills/br-ns/br-skill/versions/1.0.0/yank", bytes.NewReader(yankBody))
	yankReq.Header.Set("Content-Type", "application/json")
	yankReq.Header.Set("Authorization", "Bearer "+token.Token)
	yankResp, err := http.DefaultClient.Do(yankReq)
	if err != nil || yankResp.StatusCode != http.StatusOK {
		t.Fatalf("yank failed: status=%v, err=%v", yankResp.Status, err)
	}
	yankResp.Body.Close()

	// Stop original server & close DB
	srv.Close()
	repo.Close()

	// 6. Backup: Copy persistence
	backupDir := t.TempDir()
	if err := copyDirectory(srcDir, backupDir); err != nil {
		t.Fatalf("failed to copy backup: %v", err)
	}

	// 7. Destroy original persistence
	_ = os.RemoveAll(srcDir)

	// 8. Restore & Restart
	restoredRepo, err := metadata.NewRepository(filepath.Join(backupDir, "registry.db"))
	if err != nil {
		t.Fatalf("failed to open restored DB: %v", err)
	}
	defer restoredRepo.Close()

	restoredStore, err := storage.NewStorage(backupDir)
	if err != nil {
		t.Fatalf("failed to open restored storage: %v", err)
	}

	restoredAuth, err := auth.NewAuthenticator(true, restoredRepo.GetDB())
	if err != nil {
		t.Fatalf("failed to create restored authenticator: %v", err)
	}

	restoredReg := registry.NewRegistry(restoredRepo, restoredStore, validator, logger)
	restoredHandler := api.NewHandler(restoredReg, restoredAuth, nil, logger, cfg)
	restoredRouter := chi.NewRouter()
	restoredHandler.RegisterRoutes(restoredRouter)
	restoredSrv := httptest.NewServer(restoredRouter)
	defer restoredSrv.Close()

	// 10. Verification
	// Verify token works
	getReq, _ := http.NewRequest("GET", restoredSrv.URL+"/api/v1/skills/br-ns/br-skill/versions/1.0.0", nil)
	getReq.Header.Set("Authorization", "Bearer "+token.Token)
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil || getResp.StatusCode != http.StatusOK {
		t.Fatalf("restored version query failed: status=%v, err=%v", getResp.Status, err)
	}
	var versionDTO metadata.SkillVersion
	_ = json.NewDecoder(getResp.Body).Decode(&versionDTO)
	getResp.Body.Close()

	if !versionDTO.Yanked {
		t.Errorf("expected restored version to remain yanked")
	}
	if versionDTO.DigestSHA256 != digest {
		t.Errorf("digest mismatch on restored version: expected %s, got %s", digest, versionDTO.DigestSHA256)
	}

	// Verify attestation survives
	attestQuery, _ := http.NewRequest("GET", restoredSrv.URL+"/api/v1/artifacts/skill/br-ns/br-skill/versions/1.0.0/attestations", nil)
	attestQuery.Header.Set("Authorization", "Bearer "+token.Token)
	attestQueryResp, err := http.DefaultClient.Do(attestQuery)
	if err != nil || attestQueryResp.StatusCode != http.StatusOK {
		t.Fatalf("restored attestations query failed: status=%v", attestQueryResp.Status)
	}
	var attestRespBody struct {
		Attestations []metadata.Attestation `json:"attestations"`
	}
	_ = json.NewDecoder(attestQueryResp.Body).Decode(&attestRespBody)
	attestQueryResp.Body.Close()
	if len(attestRespBody.Attestations) != 1 {
		t.Fatalf("expected 1 restored attestation, got %d", len(attestRespBody.Attestations))
	}

	// Verify ACL survives
	role, err := restoredRepo.NamespaceRole(ctx, "br-ns", "backup-user")
	if err != nil || role != "owner" {
		t.Errorf("restored ACL check failed: expected owner, got role=%q, err=%v", role, err)
	}

	// Verify package blob data download survives and matches digest
	dlReq, _ := http.NewRequest("GET", restoredSrv.URL+"/api/v1/skills/br-ns/br-skill/versions/1.0.0/download", nil)
	dlReq.Header.Set("Authorization", "Bearer "+token.Token)
	dlResp, err := http.DefaultClient.Do(dlReq)
	if err != nil || dlResp.StatusCode != http.StatusOK {
		t.Fatalf("restored download failed: status=%v", dlResp.Status)
	}
	downloadedBytes, _ := io.ReadAll(dlResp.Body)
	dlResp.Body.Close()

	dlHash := sha256.Sum256(downloadedBytes)
	dlDigest := hex.EncodeToString(dlHash[:])
	if dlDigest != digest {
		t.Errorf("downloaded restored blob digest mismatch: expected %s, got %s", digest, dlDigest)
	}
}

func buildTestTgz(namespace, name, version string) ([]byte, error) {
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
	_ = tw.Close()
	_ = gzw.Close()
	return buf.Bytes(), nil
}

func copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
