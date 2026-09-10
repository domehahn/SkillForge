package storage_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/skillforge/skill-registry/internal/storage"
)

func TestStorage_IntegrityAndCorruption(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	content := []byte("hello skillforge storage integrity")
	hash := sha256.Sum256(content)
	expectedDigest := hex.EncodeToString(hash[:])

	// Store artifact
	digest, err := store.Store("ns", "name", "1.0.0", content)
	if err != nil {
		t.Fatalf("failed to store artifact: %v", err)
	}
	if digest != expectedDigest {
		t.Fatalf("expected digest %s, got %s", expectedDigest, digest)
	}

	// Retrieve intact blob
	retrieved, err := store.Retrieve(digest)
	if err != nil {
		t.Fatalf("failed to retrieve blob: %v", err)
	}
	if !bytes.Equal(retrieved, content) {
		t.Fatalf("retrieved content mismatch")
	}

	// Retrieve via reader
	rc, err := store.RetrieveReader(digest)
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}
	readContent, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("failed to read via reader: %v", err)
	}
	if !bytes.Equal(readContent, content) {
		t.Fatalf("read content mismatch")
	}

	// Corrupt the underlying file
	prefix1 := digest[:2]
	prefix2 := digest[2:4]
	blobPath := filepath.Join(tmpDir, "blobs", "sha256", prefix1, prefix2, digest)
	if err := os.WriteFile(blobPath, []byte("corrupted data!"), 0644); err != nil {
		t.Fatalf("failed to corrupt file: %v", err)
	}

	// Retrieve should now fail with ErrCorruptBlob
	_, err = store.Retrieve(digest)
	if err == nil {
		t.Fatal("expected error reading corrupted blob, got nil")
	}
	if !errors.Is(err, storage.ErrCorruptBlob) {
		t.Fatalf("expected ErrCorruptBlob, got %v", err)
	}

	// RetrieveReader should fail on EOF with ErrCorruptBlob
	rcCorrupt, err := store.RetrieveReader(digest)
	if err != nil {
		t.Fatalf("unexpected error getting reader for corrupted blob: %v", err)
	}
	_, err = io.ReadAll(rcCorrupt)
	_ = rcCorrupt.Close()
	if err == nil {
		t.Fatal("expected error reading corrupted blob via reader, got nil")
	}
	if !errors.Is(err, storage.ErrCorruptBlob) {
		t.Fatalf("expected ErrCorruptBlob via reader, got %v", err)
	}
}
