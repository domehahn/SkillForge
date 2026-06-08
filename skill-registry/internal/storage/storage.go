package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Storage handles blob storage for skill packages
type Storage struct {
	dataDir string
}

// NewStorage creates a new storage instance
func NewStorage(dataDir string) (*Storage, error) {
	blobsDir := filepath.Join(dataDir, "blobs", "sha256")
	packagesDir := filepath.Join(dataDir, "packages")

	if err := os.MkdirAll(blobsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blobs directory: %w", err)
	}
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create packages directory: %w", err)
	}

	return &Storage{dataDir: dataDir}, nil
}

// Store stores package data and returns the SHA-256 digest
func (s *Storage) Store(namespace, name, version string, data []byte) (digest string, err error) {
	return s.StoreArtifact("skill", namespace, name, version, data)
}

func (s *Storage) StoreArtifact(kind, namespace, name, version string, data []byte) (digest string, err error) {
	// Compute SHA-256
	hash := sha256.Sum256(data)
	digest = hex.EncodeToString(hash[:])

	// Store as content-addressable blob
	blobPath := s.blobPath(digest)
	if err := os.MkdirAll(filepath.Dir(blobPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create blob directory: %w", err)
	}

	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		tmp, err := os.CreateTemp(filepath.Dir(blobPath), ".blob-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary blob: %w", err)
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			return "", fmt.Errorf("failed to write blob: %w", err)
		}
		if err := tmp.Close(); err != nil {
			return "", fmt.Errorf("failed to close blob: %w", err)
		}
		if err := os.Rename(tmpPath, blobPath); err != nil && !os.IsExist(err) {
			return "", fmt.Errorf("failed to commit blob: %w", err)
		}
	}

	// Also store as named package for easier browsing
	packagePath := s.artifactPackagePath(kind, namespace, name, version)
	if err := os.MkdirAll(filepath.Dir(packagePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create package directory: %w", err)
	}

	// Replace the browsable package reference without mutating an existing blob hard link.
	if err := os.Remove(packagePath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to replace package link: %w", err)
	}
	if err := s.linkOrCopy(blobPath, packagePath); err != nil {
		return "", fmt.Errorf("failed to link package: %w", err)
	}

	return digest, nil
}

// Retrieve retrieves package data by digest
func (s *Storage) Retrieve(digest string) ([]byte, error) {
	blobPath := s.blobPath(digest)
	data, err := os.ReadFile(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("package not found")
		}
		return nil, fmt.Errorf("failed to read package: %w", err)
	}
	return data, nil
}

// RetrieveReader retrieves package data as a reader
func (s *Storage) RetrieveReader(digest string) (io.ReadCloser, error) {
	blobPath := s.blobPath(digest)
	file, err := os.Open(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("package not found")
		}
		return nil, fmt.Errorf("failed to open package: %w", err)
	}
	return file, nil
}

// Delete deletes a package by digest
func (s *Storage) Delete(digest string) error {
	blobPath := s.blobPath(digest)
	if err := os.Remove(blobPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete blob: %w", err)
	}
	return nil
}

// Exists checks if a package exists by digest
func (s *Storage) Exists(digest string) bool {
	_, err := os.Stat(s.blobPath(digest))
	return err == nil
}

func (s *Storage) blobPath(digest string) string {
	// Store as sha256/ab/cd/abcd...
	prefix1 := digest[:2]
	prefix2 := digest[2:4]
	return filepath.Join(s.dataDir, "blobs", "sha256", prefix1, prefix2, digest)
}

func (s *Storage) packagePath(namespace, name, version string) string {
	return filepath.Join(s.dataDir, "packages", namespace, name, version+".tgz")
}

func (s *Storage) artifactPackagePath(kind, namespace, name, version string) string {
	if kind == "skill" {
		return s.packagePath(namespace, name, version)
	}
	return filepath.Join(s.dataDir, "packages", kind, namespace, name, version+".pkg")
}

func (s *Storage) linkOrCopy(src, dst string) error {
	// Try to create a hard link first
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	// Fall back to copy
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
