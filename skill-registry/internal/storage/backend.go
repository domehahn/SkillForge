package storage

import "io"

// Backend is the blob storage abstraction internal/registry depends on.
// *Storage (this package) is the default filesystem-backed implementation,
// suitable for a single node. *S3Storage is an S3-compatible object-store
// implementation for running more than one registry instance against the
// same artifact set — a local filesystem directory is not shared across
// pods/hosts, so a filesystem-only Backend meant multiple instances would
// each see a different, incomplete set of stored artifacts.
type Backend interface {
	// Store stores skill package data and returns its SHA-256 digest.
	Store(namespace, name, version string, data []byte) (digest string, err error)
	// StoreArtifact is Store generalized to a non-"skill" artifact kind.
	StoreArtifact(kind, namespace, name, version string, data []byte) (digest string, err error)
	// Retrieve reads back stored data by its SHA-256 digest.
	Retrieve(digest string) ([]byte, error)
	// RetrieveReader is Retrieve without buffering the whole blob in memory.
	RetrieveReader(digest string) (io.ReadCloser, error)
	// Delete removes a stored blob by digest.
	Delete(digest string) error
	// Exists reports whether a blob with the given digest is stored.
	Exists(digest string) bool
}

var (
	_ Backend = (*Storage)(nil)
	_ Backend = (*S3Storage)(nil)
)
