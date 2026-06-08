package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/observability"
	"github.com/skillforge/skill-registry/internal/storage"
	"github.com/skillforge/skill-registry/internal/validation"
)

func setupTestRegistry(t *testing.T) (*Registry, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")

	repo, err := metadata.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	store, err := storage.NewStorage(dataDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	validator := validation.NewValidator(50, []string{".exe"})
	logger := observability.NewLogger()

	reg := NewRegistry(repo, store, validator, logger)

	cleanup := func() {
		repo.Close()
		os.RemoveAll(tmpDir)
	}

	return reg, cleanup
}

func TestPublish(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	// Create a simple valid package
	data := createTestPackage(t)

	skillVersion, err := reg.Publish(ctx, "test", "hello-skill", "1.0.0", data, "tgz", PublishOptions{
		CreatedBy: "test-user",
	})

	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if skillVersion.Name != "hello-skill" {
		t.Errorf("Expected name 'hello-skill', got '%s'", skillVersion.Name)
	}

	if skillVersion.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", skillVersion.Version)
	}
}

func TestPublishDuplicate(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()
	data := createTestPackage(t)

	// Publish once
	_, err := reg.Publish(ctx, "test", "hello-skill", "1.0.0", data, "tgz", PublishOptions{
		CreatedBy: "test-user",
	})
	if err != nil {
		t.Fatalf("First publish failed: %v", err)
	}

	// Publish again without force
	_, err = reg.Publish(ctx, "test", "hello-skill", "1.0.0", data, "tgz", PublishOptions{
		CreatedBy: "test-user",
	})
	if err == nil {
		t.Error("Expected duplicate publish to fail")
	}

	// Published versions remain immutable, even with force.
	_, err = reg.Publish(ctx, "test", "hello-skill", "1.0.0", data, "tgz", PublishOptions{
		CreatedBy: "test-user",
		Force:     true,
	})
	if err == nil {
		t.Error("Expected forced duplicate publish to fail")
	}
}

func TestDistTagsAndDownloadCount(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()
	ctx := context.Background()
	data := createTestPackage(t)

	if _, err := reg.Publish(ctx, "test", "hello-skill", "1.0.0", data, "tgz", PublishOptions{CreatedBy: "test-user"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.SetDistTag(ctx, "test", "hello-skill", "stable", "1.0.0", "test-user"); err != nil {
		t.Fatal(err)
	}
	tags, err := reg.ListDistTags(ctx, "test", "hello-skill")
	if err != nil {
		t.Fatal(err)
	}
	if tags["latest"] != "1.0.0" || tags["stable"] != "1.0.0" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	if _, _, err := reg.Download(ctx, "test", "hello-skill", "stable"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := reg.Download(ctx, "test", "hello-skill", "latest"); err != nil {
		t.Fatal(err)
	}
	skill, versions, err := reg.GetSkill(ctx, "test", "hello-skill")
	if err != nil {
		t.Fatal(err)
	}
	if skill.Downloads != 2 || versions[0].Downloads != 2 {
		t.Fatalf("unexpected download counts: skill=%d version=%d", skill.Downloads, versions[0].Downloads)
	}
}

func TestGetVersion(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()
	data := createTestPackage(t)

	// Publish
	_, err := reg.Publish(ctx, "test", "hello-skill", "1.0.0", data, "tgz", PublishOptions{
		CreatedBy: "test-user",
	})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Get version
	skillVersion, err := reg.GetVersion(ctx, "test", "hello-skill", "1.0.0")
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if skillVersion.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", skillVersion.Version)
	}
}

func TestDownload(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()
	data := createTestPackage(t)

	// Publish
	published, err := reg.Publish(ctx, "test", "hello-skill", "1.0.0", data, "tgz", PublishOptions{
		CreatedBy: "test-user",
	})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Download
	downloadedData, skillVersion, err := reg.Download(ctx, "test", "hello-skill", "1.0.0")
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if skillVersion.DigestSHA256 != published.DigestSHA256 {
		t.Error("Digest mismatch")
	}

	if len(downloadedData) != len(data) {
		t.Error("Downloaded data size mismatch")
	}
}

func createTestPackage(t *testing.T) []byte {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	files := map[string]string{
		"SKILL.md":      "# Hello Skill\n\nA test skill.",
		"VERSION":       "1.0.0\n",
		"skill.yaml":    "name: hello-skill\nnamespace: test\nversion: 1.0.0\ndescription: A test skill.\nowners:\n  - test-team\nlicense: MIT\ncompatible_with:\n  - codex\nentrypoint: SKILL.md\nsecurity:\n  requires_network: false\n  requires_secrets: false\n  writes_files: false\n  runs_commands: false\n",
		"CHANGELOG.md":  "# Changelog\n\n## 1.0.0\n\n### Added\n- Initial test skill.\n",
		"README.md":     "# README",
		"manifest.json": `{"spec_version":1,"name":"hello-skill","namespace":"test","version":"1.0.0","entrypoint":"SKILL.md","compatible_with":["codex"],"package_type":"tgz","packaged_by":"skforge","packaged_at":"1970-01-01T00:00:00Z"}`,
		"checksums.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  SKILL.md\n",
	}

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
	}

	tw.Close()
	gzw.Close()

	return buf.Bytes()
}
