package storage

// Exercises S3Storage against a real MinIO container (not a fake/mocked S3
// server) — the same "test against the real thing, not a stand-in"
// approach used elsewhere in this project's cross-repo E2E tests. Skips
// itself (rather than failing) when Docker isn't available, since this is
// an environment-dependent integration test, not something every CI run
// needs to gate on.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"os/exec"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	testMinioAccessKey = "minioadmin"
	testMinioSecretKey = "minioadmin"
)

// startMinIO launches a disposable MinIO container on a random host port
// and returns its endpoint plus a cleanup func. Skips the calling test if
// Docker isn't available.
func startMinIO(t *testing.T) (endpoint string) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available — skipping S3Storage integration test")
	}

	port := 19000 + rand.Intn(2000)
	containerName := fmt.Sprintf("skillforge-s3-test-%d", port)
	endpoint = fmt.Sprintf("127.0.0.1:%d", port)

	cmd := exec.Command("docker", "run", "-d", "--rm",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:9000", port),
		"-e", "MINIO_ROOT_USER="+testMinioAccessKey,
		"-e", "MINIO_ROOT_PASSWORD="+testMinioSecretKey,
		"minio/minio:latest", "server", "/data",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not start MinIO container (likely no network access to pull the image): %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	// Wait for MinIO to accept connections and serve its health endpoint.
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(testMinioAccessKey, testMinioSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("create minio client: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.ListBuckets(context.Background()); err == nil {
			return endpoint
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("MinIO container did not become ready in time")
	return ""
}

func newTestS3Storage(t *testing.T) *S3Storage {
	t.Helper()
	endpoint := startMinIO(t)
	bucket := fmt.Sprintf("test-bucket-%d", rand.Intn(1_000_000))
	s3, err := NewS3Storage(context.Background(), endpoint, "us-east-1", bucket, testMinioAccessKey, testMinioSecretKey, false, true)
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}
	return s3
}

func TestS3StorageStoreAndRetrieve(t *testing.T) {
	s := newTestS3Storage(t)
	data := []byte("skill package contents")

	digest, err := s.Store("default", "my-skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if !s.Exists(digest) {
		t.Fatal("expected the stored digest to exist")
	}

	got, err := s.Retrieve(digest)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("retrieved data mismatch: got %q, want %q", got, data)
	}
}

func TestS3StorageRetrieveReader(t *testing.T) {
	s := newTestS3Storage(t)
	data := []byte("streamed content")
	digest, err := s.Store("default", "my-skill", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	rc, err := s.RetrieveReader(digest)
	if err != nil {
		t.Fatalf("RetrieveReader: %v", err)
	}
	defer rc.Close()
	buf := make([]byte, len(data))
	if _, err := io.ReadFull(rc, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(buf, data) {
		t.Fatalf("streamed data mismatch: got %q, want %q", buf, data)
	}
}

func TestS3StorageDeduplicatesIdenticalContent(t *testing.T) {
	s := newTestS3Storage(t)
	data := []byte("identical content")

	d1, err := s.Store("default", "skill-a", "1.0.0", data)
	if err != nil {
		t.Fatalf("Store 1: %v", err)
	}
	d2, err := s.Store("default", "skill-b", "2.0.0", data)
	if err != nil {
		t.Fatalf("Store 2: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("expected identical content to produce the same digest, got %q and %q", d1, d2)
	}
}

func TestS3StorageExistsFalseForUnknownDigest(t *testing.T) {
	s := newTestS3Storage(t)
	if s.Exists("0000000000000000000000000000000000000000000000000000000000000000") {
		t.Fatal("expected a never-stored digest to not exist")
	}
}

func TestS3StorageRetrieveMissingDigestErrors(t *testing.T) {
	s := newTestS3Storage(t)
	_, err := s.Retrieve("0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected retrieving a missing digest to error")
	}
}

func TestS3StorageDelete(t *testing.T) {
	s := newTestS3Storage(t)
	digest, err := s.Store("default", "my-skill", "1.0.0", []byte("to be deleted"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := s.Delete(digest); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s.Exists(digest) {
		t.Fatal("expected the blob to no longer exist after Delete")
	}
	// Deleting again (already gone) must not error.
	if err := s.Delete(digest); err != nil {
		t.Fatalf("Delete of an already-deleted blob should be a no-op, got: %v", err)
	}
}

func TestS3StorageStoreArtifactNonSkillKind(t *testing.T) {
	s := newTestS3Storage(t)
	digest, err := s.StoreArtifact("mcp", "default", "my-server", "1.0.0", []byte("mcp package"))
	if err != nil {
		t.Fatalf("StoreArtifact: %v", err)
	}
	got, err := s.Retrieve(digest)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if string(got) != "mcp package" {
		t.Fatalf("unexpected content: %q", got)
	}
}
