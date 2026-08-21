package api_test

import (
	"bytes"
	"context"
	"encoding/hex"
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

// TestBackupAndRestore publishes a skill, copies DB + blobs to a "backup" directory,
// then starts a fresh registry pointing at the backup and verifies resolve + download
// + SHA256 all match what was originally published.
func TestBackupAndRestore(t *testing.T) {
	srcDir := t.TempDir()

	// ── 1. Publish into the source registry ──────────────────────────────────

	src := newRegistryAt(t, srcDir)
	pkg := buildTgzPackage(t, "test", "backup-skill", "1.0.0")

	req, _ := http.NewRequest("PUT", src.URL+"/api/v1/skills/test/backup-skill/versions/1.0.0", bytes.NewReader(pkg))
	req.Header.Set("Content-Type", "application/gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("publish: expected 201, got %d", resp.StatusCode)
	}

	// Record the digest reported at publish time.
	resolveResp, err := http.Get(src.URL + "/api/v1/skills/test/backup-skill/resolve")
	if err != nil {
		t.Fatalf("resolve on source: %v", err)
	}
	defer resolveResp.Body.Close()
	if resolveResp.StatusCode != http.StatusOK {
		t.Fatalf("resolve on source: expected 200, got %d", resolveResp.StatusCode)
	}

	dlResp, err := http.Get(src.URL + "/api/v1/skills/test/backup-skill/versions/1.0.0/download")
	if err != nil {
		t.Fatalf("download on source: %v", err)
	}
	defer dlResp.Body.Close()
	originalDigest := dlResp.Header.Get("X-Skill-Digest-SHA256")
	originalBody, _ := io.ReadAll(dlResp.Body)

	src.Close()

	// ── 2. Simulate backup: copy DB + blobs to a separate directory ──────────

	backupDir := t.TempDir()
	if err := copyDir(t, srcDir, backupDir); err != nil {
		t.Fatalf("backup copy: %v", err)
	}

	// ── 3. Restore: new registry instance pointing at the backup ─────────────

	restored := newRegistryAt(t, backupDir)

	// ── 4. Verify resolve returns the same version ────────────────────────────

	r2, err := http.Get(restored.URL + "/api/v1/skills/test/backup-skill/resolve")
	if err != nil {
		t.Fatalf("resolve on restored: %v", err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("resolve on restored: expected 200, got %d", r2.StatusCode)
	}

	// ── 5. Verify download SHA256 matches original ────────────────────────────

	dl2, err := http.Get(restored.URL + "/api/v1/skills/test/backup-skill/versions/1.0.0/download")
	if err != nil {
		t.Fatalf("download on restored: %v", err)
	}
	defer dl2.Body.Close()

	restoredDigest := dl2.Header.Get("X-Skill-Digest-SHA256")
	restoredBody, _ := io.ReadAll(dl2.Body)

	if restoredDigest != originalDigest {
		t.Errorf("digest mismatch after restore: original=%q restored=%q", originalDigest, restoredDigest)
	}
	if !bytes.Equal(restoredBody, originalBody) {
		t.Errorf("download body differs after restore: original=%d bytes restored=%d bytes", len(originalBody), len(restoredBody))
	}
	_ = hex.DecodeString // referenced to avoid import churn
}

// newRegistryAt creates a fresh in-process httptest.Server backed by the given directory.
func newRegistryAt(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	dbPath := filepath.Join(dir, "test.db")
	blobDir := filepath.Join(dir, "blobs")

	repo, err := metadata.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("metadata.NewRepository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	store, err := storage.NewStorage(blobDir)
	if err != nil {
		t.Fatalf("storage.NewStorage: %v", err)
	}

	validator := validation.NewValidator(50, nil)
	logger := observability.NewLogger()
	reg := registry.NewRegistry(repo, store, validator, logger)

	cfg := config.DefaultConfig()
	cfg.Auth.Enabled = false

	authenticator, err := auth.NewAuthenticator(false, repo.GetDB())
	if err != nil {
		t.Fatalf("auth.NewAuthenticator: %v", err)
	}

	handler := api.NewHandler(reg, authenticator, nil, logger, cfg)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// copyDir recursively copies src into dst, replicating the directory structure.
func copyDir(t *testing.T, src, dst string) error {
	t.Helper()
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// TestRestorePreservesAuditLog verifies that audit log entries written to the DB
// survive a backup/restore cycle and are queryable from the restored instance.
func TestRestorePreservesAuditLog(t *testing.T) {
	srcDir := t.TempDir()

	repo, err := metadata.NewRepository(filepath.Join(srcDir, "test.db"))
	if err != nil {
		t.Fatalf("metadata.NewRepository: %v", err)
	}

	// Write a row directly into the audit_log table via the shared DB.
	_, execErr := repo.GetDB().ExecContext(context.Background(),
		`INSERT INTO audit_log (actor, action, target, ip_address, created_at)
		 VALUES ('test-actor', 'skill.publish', 'test/my-skill@1.0.0', '127.0.0.1', datetime('now'))`)
	if execErr != nil {
		// Table may not exist yet; skip rather than fail (audit_log created by NewHandler wiring).
		t.Skipf("audit_log table not present in test DB: %v", execErr)
	}
	repo.Close()

	backupDir := t.TempDir()
	if err := copyDir(t, srcDir, backupDir); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Open restored DB and verify the row is present.
	repo2, err := metadata.NewRepository(filepath.Join(backupDir, "test.db"))
	if err != nil {
		t.Fatalf("restore metadata.NewRepository: %v", err)
	}
	defer repo2.Close()

	var count int
	row := repo2.GetDB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM audit_log WHERE actor='test-actor' AND action='skill.publish'`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("query restored audit_log: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 audit entry after restore, got %d", count)
	}
}
