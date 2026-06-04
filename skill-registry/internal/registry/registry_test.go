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

	// Publish again with force
	_, err = reg.Publish(ctx, "test", "hello-skill", "1.0.0", data, "tgz", PublishOptions{
		CreatedBy: "test-user",
		Force:     true,
	})
	if err != nil {
		t.Errorf("Publish with force failed: %v", err)
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
		"SKILL.md":  "# Hello Skill\n\nA test skill.",
		"README.md": "# README",
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
