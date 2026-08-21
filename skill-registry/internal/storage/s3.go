package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Storage is a Backend implementation backed by an S3-compatible object
// store (AWS S3, MinIO, and most self-hosted equivalents that speak the S3
// API). Every registry instance pointed at the same bucket/prefix shares
// the same artifact set, which a local filesystem Backend cannot do across
// more than one host — this is what makes running SkillForge as more than
// one replica behind a load balancer actually work for artifact storage,
// not just for the (already horizontally-scalable) metadata database.
type S3Storage struct {
	client *minio.Client
	bucket string
}

// NewS3Storage creates an S3Storage against the given bucket, creating the
// bucket if it doesn't already exist. cfg mirrors config.S3Config's fields
// directly to avoid this package importing internal/config (which would be
// a layering inversion — config shouldn't need to know about storage
// backends, and storage shouldn't need to know about the whole app config).
func NewS3Storage(ctx context.Context, endpoint, region, bucket, accessKey, secretKey string, useSSL, pathStyle bool) (*S3Storage, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	}
	if pathStyle {
		opts.BucketLookup = minio.BucketLookupPath
	}
	client, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("create S3 client: %w", err)
	}

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
			return nil, fmt.Errorf("create bucket %q: %w", bucket, err)
		}
	}

	return &S3Storage{client: client, bucket: bucket}, nil
}

func (s *S3Storage) blobKey(digest string) string {
	return "blobs/sha256/" + digest
}

func (s *S3Storage) packageKey(kind, namespace, name, version string) string {
	if kind == "skill" {
		return fmt.Sprintf("packages/%s/%s/%s.tgz", namespace, name, version)
	}
	return fmt.Sprintf("packages/%s/%s/%s/%s.pkg", kind, namespace, name, version)
}

// Store stores skill package data and returns its SHA-256 digest.
func (s *S3Storage) Store(namespace, name, version string, data []byte) (string, error) {
	return s.StoreArtifact("skill", namespace, name, version, data)
}

// StoreArtifact stores data as a content-addressed blob (skipped if a blob
// with that digest already exists) and, for browsability, also as a named
// object at the package's own key — mirroring the local filesystem
// Backend's dual-write behavior.
func (s *S3Storage) StoreArtifact(kind, namespace, name, version string, data []byte) (string, error) {
	ctx := context.Background()
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])

	blobKey := s.blobKey(digest)
	if _, err := s.client.StatObject(ctx, s.bucket, blobKey, minio.StatObjectOptions{}); err != nil {
		if !isNotFound(err) {
			return "", fmt.Errorf("stat blob %q: %w", blobKey, err)
		}
		if _, err := s.client.PutObject(ctx, s.bucket, blobKey, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		}); err != nil {
			return "", fmt.Errorf("store blob %q: %w", blobKey, err)
		}
	}

	packageKey := s.packageKey(kind, namespace, name, version)
	if _, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: packageKey},
		minio.CopySrcOptions{Bucket: s.bucket, Object: blobKey},
	); err != nil {
		return "", fmt.Errorf("link package %q: %w", packageKey, err)
	}

	return digest, nil
}

// Retrieve reads back stored data by its SHA-256 digest.
func (s *S3Storage) Retrieve(digest string) ([]byte, error) {
	rc, err := s.RetrieveReader(digest)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read package: %w", err)
	}
	return data, nil
}

// RetrieveReader is Retrieve without buffering the whole blob in memory.
func (s *S3Storage) RetrieveReader(digest string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(context.Background(), s.bucket, s.blobKey(digest), minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	// GetObject doesn't itself fail on a missing key — the error surfaces
	// on first read/stat, so check now rather than handing back a reader
	// that will fail unpredictably on first use.
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		if isNotFound(err) {
			return nil, fmt.Errorf("package not found")
		}
		return nil, fmt.Errorf("stat object: %w", err)
	}
	return obj, nil
}

// Delete removes a stored blob by digest.
func (s *S3Storage) Delete(digest string) error {
	err := s.client.RemoveObject(context.Background(), s.bucket, s.blobKey(digest), minio.RemoveObjectOptions{})
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("delete blob: %w", err)
	}
	return nil
}

// Exists reports whether a blob with the given digest is stored.
func (s *S3Storage) Exists(digest string) bool {
	_, err := s.client.StatObject(context.Background(), s.bucket, s.blobKey(digest), minio.StatObjectOptions{})
	return err == nil
}

func isNotFound(err error) bool {
	var errResp minio.ErrorResponse
	if errors.As(err, &errResp) {
		return errResp.Code == "NoSuchKey" || errResp.Code == "NotFound"
	}
	return minio.ToErrorResponse(err).Code == "NoSuchKey"
}
