package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestStorage(t *testing.T) (*Storage, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return storage, cleanup
}

func TestNewStorage(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	if storage == nil {
		t.Fatal("expected storage to be non-nil")
	}

	// Verify directories were created
	blobsDir := filepath.Join(tmpDir, "blobs", "sha256")
	if _, err := os.Stat(blobsDir); os.IsNotExist(err) {
		t.Errorf("blobs directory was not created: %s", blobsDir)
	}

	packagesDir := filepath.Join(tmpDir, "packages")
	if _, err := os.Stat(packagesDir); os.IsNotExist(err) {
		t.Errorf("packages directory was not created: %s", packagesDir)
	}
}

func TestNewStorage_InvalidDir(t *testing.T) {
	// Use a path that cannot be created
	invalidPath := "/proc/invalid/test"
	_, err := NewStorage(invalidPath)
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestNewStorage_PackagesDirError(t *testing.T) {
	// Skip if running as root
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	tmpDir := t.TempDir()

	// Create blobs directory successfully
	blobsDir := filepath.Join(tmpDir, "blobs", "sha256")
	if err := os.MkdirAll(blobsDir, 0755); err != nil {
		t.Fatalf("failed to create blobs dir: %v", err)
	}

	// Create a file named "packages" so the packages directory creation will fail
	packagesPath := filepath.Join(tmpDir, "packages")
	if err := os.WriteFile(packagesPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create packages file: %v", err)
	}

	// NewStorage should fail when creating packages directory
	_, err := NewStorage(tmpDir)
	if err == nil {
		t.Error("expected error when packages directory creation fails")
	}
}

func TestStore(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	data := []byte("test package data")
	digest, err := storage.Store("test", "my-skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if digest == "" {
		t.Error("expected non-empty digest")
	}

	// Verify blob exists
	if !storage.Exists(digest) {
		t.Error("blob should exist after Store")
	}

	// Verify content
	retrieved, err := storage.Retrieve(digest)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("expected data %q, got %q", data, retrieved)
	}
}

func TestStoreReplacingNamedPackageDoesNotMutateOldBlob(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	oldData := []byte("old immutable content")
	oldDigest, err := storage.Store("test", "my-skill", "1.0.0", oldData)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Store("test", "my-skill", "1.0.0", []byte("new content")); err != nil {
		t.Fatal(err)
	}
	retrieved, err := storage.Retrieve(oldDigest)
	if err != nil {
		t.Fatal(err)
	}
	if string(retrieved) != string(oldData) {
		t.Fatalf("old content-addressed blob was mutated: %q", retrieved)
	}
}

func TestStore_LargeData(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create 1MB of data
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	digest, err := storage.Store("test", "large-skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store failed for large data: %v", err)
	}

	retrieved, err := storage.Retrieve(digest)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	if len(retrieved) != len(data) {
		t.Errorf("expected %d bytes, got %d", len(data), len(retrieved))
	}
}

func TestRetrieve_NotFound(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	_, err := storage.Retrieve("nonexistent1234567890abcdef")
	if err == nil {
		t.Error("expected error for nonexistent package, got nil")
	}
}

func TestRetrieveReader(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	data := []byte("test data for reader")
	digest, err := storage.Store("test", "skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	reader, err := storage.RetrieveReader(digest)
	if err != nil {
		t.Fatalf("RetrieveReader failed: %v", err)
	}
	defer reader.Close()

	retrieved := make([]byte, len(data))
	n, err := reader.Read(retrieved)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if n != len(data) {
		t.Errorf("expected to read %d bytes, got %d", len(data), n)
	}

	if string(retrieved) != string(data) {
		t.Errorf("expected data %q, got %q", data, retrieved)
	}
}

func TestRetrieveReader_NotFound(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	_, err := storage.RetrieveReader("nonexistent1234567890abcdef")
	if err == nil {
		t.Error("expected error for nonexistent package, got nil")
	}
}

func TestDelete(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	data := []byte("test data to delete")
	digest, err := storage.Store("test", "skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify it exists
	if !storage.Exists(digest) {
		t.Error("blob should exist before Delete")
	}

	// Delete it
	err = storage.Delete(digest)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	if storage.Exists(digest) {
		t.Error("blob should not exist after Delete")
	}
}

func TestDelete_NonExistent(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Deleting non-existent package should not error
	err := storage.Delete("nonexistent1234567890abcdef")
	if err != nil {
		t.Errorf("Delete of non-existent package should not error, got: %v", err)
	}
}

func TestExists(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Non-existent
	if storage.Exists("nonexistent1234567890abcdef") {
		t.Error("Exists should return false for non-existent package")
	}

	// Store a package
	data := []byte("test data")
	digest, err := storage.Store("test", "skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Should exist now
	if !storage.Exists(digest) {
		t.Error("Exists should return true for existing package")
	}
}

func TestBlobPath(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	digest := "abcdef1234567890"
	path := storage.blobPath(digest)

	expectedSuffix := filepath.Join("blobs", "sha256", "ab", "cd", digest)
	if !filepath.IsAbs(path) {
		t.Error("blobPath should return absolute path")
	}

	if !strings.HasPrefix(path, storage.dataDir) {
		t.Errorf("blobPath should start with dataDir %s, got %s", storage.dataDir, path)
	}

	if !strings.HasSuffix(path, expectedSuffix) {
		t.Errorf("blobPath should end with %s, got %s", expectedSuffix, path)
	}
}

func TestPackagePath(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	path := storage.packagePath("myteam", "my-skill", "1.0.0")

	expectedSuffix := filepath.Join("packages", "myteam", "my-skill", "1.0.0.tgz")
	if !strings.HasSuffix(path, expectedSuffix) {
		t.Errorf("packagePath should end with %s, got %s", expectedSuffix, path)
	}
}

func TestLinkOrCopy(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create source file
	srcPath := filepath.Join(storage.dataDir, "source.txt")
	data := []byte("test data for link or copy")
	if err := os.WriteFile(srcPath, data, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Test linkOrCopy
	dstPath := filepath.Join(storage.dataDir, "destination.txt")
	if err := storage.linkOrCopy(srcPath, dstPath); err != nil {
		t.Fatalf("linkOrCopy failed: %v", err)
	}

	// Verify destination file exists and has correct content
	retrieved, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("expected data %q, got %q", data, retrieved)
	}
}

func TestLinkOrCopy_InvalidSource(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	err := storage.linkOrCopy("/nonexistent/source", "/tmp/dest")
	if err == nil {
		t.Error("linkOrCopy should fail with invalid source")
	}
}

func TestStore_CreatesDirsRecursively(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Store with deep namespace/name structure
	data := []byte("test")
	digest, err := storage.Store("org/team", "project/skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if !storage.Exists(digest) {
		t.Error("blob should exist after Store with deep paths")
	}
}

func TestLinkOrCopy_FallbackToCopy(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create source file
	srcPath := filepath.Join(storage.dataDir, "src_fallback.txt")
	data := []byte("fallback copy test")
	if err := os.WriteFile(srcPath, data, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create destination file first - this will cause Link to fail,
	// forcing fallback to copy
	dstPath := filepath.Join(storage.dataDir, "dst_exists.txt")
	if err := os.WriteFile(dstPath, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to create destination file: %v", err)
	}

	// This should succeed by using copy instead of link
	if err := storage.linkOrCopy(srcPath, dstPath); err != nil {
		t.Fatalf("linkOrCopy failed with fallback: %v", err)
	}

	// Verify content was copied
	retrieved, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("expected %q, got %q", data, retrieved)
	}
}

func TestLinkOrCopy_FailToCreateDestination(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create source file
	srcPath := filepath.Join(storage.dataDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Try to create destination in non-existent directory
	// (after link fails, copy should also fail to open dest)
	dstPath := "/proc/invalid/dest.txt"
	err := storage.linkOrCopy(srcPath, dstPath)
	if err == nil {
		t.Error("linkOrCopy should fail with invalid destination path")
	}
}

func TestRetrieve_ReadError(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Store a file
	data := []byte("test data")
	digest, err := storage.Store("test", "skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Make the file unreadable
	blobPath := storage.blobPath(digest)
	if err := os.Chmod(blobPath, 0000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(blobPath, 0644) // restore for cleanup

	// Try to retrieve - should fail
	_, err = storage.Retrieve(digest)
	if err == nil {
		t.Error("Retrieve should fail with unreadable file")
	}
}

func TestRetrieveReader_ReadError(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Store a file
	data := []byte("test data")
	digest, err := storage.Store("test", "skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Make the file unreadable
	blobPath := storage.blobPath(digest)
	if err := os.Chmod(blobPath, 0000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(blobPath, 0644) // restore for cleanup

	// Try to retrieve reader - should fail
	_, err = storage.RetrieveReader(digest)
	if err == nil {
		t.Error("RetrieveReader should fail with unreadable file")
	}
}

func TestStore_BlobDirCreationError(t *testing.T) {
	// This test is OS-dependent and may not work on all systems
	// Skip if we can't create the necessary conditions
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	// Make blobs dir read-only to prevent subdirectory creation
	blobsDir := filepath.Join(tmpDir, "blobs", "sha256")
	if err := os.Chmod(blobsDir, 0444); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(blobsDir, 0755) // restore for cleanup

	// Try to store - should fail to create blob subdirectory
	_, err = storage.Store("test", "skill", "1.0.0", []byte("test"))
	if err == nil {
		t.Error("Store should fail when blob directory creation fails")
	}
}

func TestStore_PackageDirCreationError(t *testing.T) {
	// Skip if running as root
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	// Make packages dir read-only
	packagesDir := filepath.Join(tmpDir, "packages")
	if err := os.Chmod(packagesDir, 0444); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(packagesDir, 0755) // restore

	// Store should fail when creating package directory
	_, err = storage.Store("test", "skill", "1.0.0", []byte("test"))
	if err == nil {
		t.Error("Store should fail when package directory creation fails")
	}
}

func TestStore_WriteBlobError(t *testing.T) {
	// Skip if running as root
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	// Create the digest path structure manually
	data := []byte("test")
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])
	prefix1 := digest[:2]
	prefix2 := digest[2:4]
	blobDir := filepath.Join(tmpDir, "blobs", "sha256", prefix1, prefix2)
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatalf("failed to create blob dir: %v", err)
	}

	// Make the directory read-only so WriteFile will fail
	if err := os.Chmod(blobDir, 0555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(blobDir, 0755) // restore

	// Store should fail when writing blob
	_, err = storage.Store("test", "skill", "1.0.0", data)
	if err == nil {
		t.Error("Store should fail when writing blob fails")
	}
}

func TestDelete_Error(t *testing.T) {
	// Skip if running as root
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	tmpDir := t.TempDir()
	storage, err := NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	// Store a file
	data := []byte("test")
	digest, err := storage.Store("test", "skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Make the parent directory non-writable
	blobPath := storage.blobPath(digest)
	parentDir := filepath.Dir(blobPath)
	if err := os.Chmod(parentDir, 0555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(parentDir, 0755) // restore

	// Delete should fail
	err = storage.Delete(digest)
	if err == nil {
		t.Error("Delete should fail when file cannot be removed")
	}
}
