package storage_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os/exec"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/skillforge/skill-registry/internal/storage"
)

const (
	faultMinioAccessKey = "minioadmin"
	faultMinioSecretKey = "minioadmin"
)

func startMinIOForFaults(t *testing.T) (string, *minio.Client) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available — skipping S3 fault injection test")
	}

	port := 21000 + rand.Intn(2000)
	containerName := fmt.Sprintf("skillforge-s3-fault-%d", port)
	endpoint := fmt.Sprintf("127.0.0.1:%d", port)

	cmd := exec.Command("docker", "run", "-d", "--rm",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:9000", port),
		"-e", "MINIO_ROOT_USER="+faultMinioAccessKey,
		"-e", "MINIO_ROOT_PASSWORD="+faultMinioSecretKey,
		"minio/minio:latest", "server", "/data",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not start MinIO container: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(faultMinioAccessKey, faultMinioSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("create minio client: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.ListBuckets(context.Background()); err == nil {
			return endpoint, client
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("MinIO container did not become ready in time")
	return "", nil
}

func TestS3Storage_CorruptedObjectFaultInjection(t *testing.T) {
	endpoint, rawClient := startMinIOForFaults(t)
	bucket := fmt.Sprintf("fault-bucket-%d", rand.Intn(1_000_000))
	ctx := context.Background()

	s3Store, err := storage.NewS3Storage(ctx, endpoint, "us-east-1", bucket, faultMinioAccessKey, faultMinioSecretKey, false, true)
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}

	content := []byte("original valid package content for fault test")
	digest, err := s3Store.Store("fault-ns", "fault-skill", "1.0.0", content)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Verify intact
	got, err := s3Store.Retrieve(digest)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("expected initial retrieval success, got data=%q, err=%v", got, err)
	}

	// Overwrite underlying S3 object directly with corrupted bytes
	blobKey := "blobs/sha256/" + digest
	corruptContent := []byte("corrupted payload")
	if _, err := rawClient.PutObject(ctx, bucket, blobKey, bytes.NewReader(corruptContent), int64(len(corruptContent)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	}); err != nil {
		t.Fatalf("failed to corrupt S3 object: %v", err)
	}

	// Retrieve should now detect SHA-256 digest mismatch and return ErrCorruptBlob
	_, err = s3Store.Retrieve(digest)
	if err == nil {
		t.Fatal("expected error retrieving corrupted S3 object, got nil")
	}
	if !errors.Is(err, storage.ErrCorruptBlob) {
		t.Fatalf("expected ErrCorruptBlob, got %v", err)
	}

	// RetrieveReader should also fail on EOF with ErrCorruptBlob
	rc, err := s3Store.RetrieveReader(digest)
	if err != nil {
		t.Fatalf("RetrieveReader error: %v", err)
	}
	_, err = io.ReadAll(rc)
	_ = rc.Close()
	if err == nil {
		t.Fatal("expected error reading corrupted S3 object via reader, got nil")
	}
	if !errors.Is(err, storage.ErrCorruptBlob) {
		t.Fatalf("expected ErrCorruptBlob via reader, got %v", err)
	}
}

func TestS3Storage_MissingObjectFaultInjection(t *testing.T) {
	endpoint, _ := startMinIOForFaults(t)
	bucket := fmt.Sprintf("fault-missing-bucket-%d", rand.Intn(1_000_000))
	ctx := context.Background()

	s3Store, err := storage.NewS3Storage(ctx, endpoint, "us-east-1", bucket, faultMinioAccessKey, faultMinioSecretKey, false, true)
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}

	h := sha256.Sum256([]byte("non-existent"))
	fakeDigest := hex.EncodeToString(h[:])
	_, err = s3Store.Retrieve(fakeDigest)
	if err == nil {
		t.Fatal("expected error for non-existent S3 object, got nil")
	}
}
