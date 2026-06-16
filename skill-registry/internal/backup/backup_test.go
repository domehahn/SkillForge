package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestCreateBackup(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "registry.db")
	storageDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(filepath.Join(storageDir, "blobs", "sha256", "ab", "cd"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "blobs", "sha256", "ab", "cd", "blob"), []byte("package"), 0644); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE items (name TEXT NOT NULL); INSERT INTO items (name) VALUES ('one')`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	dest := filepath.Join(tmp, "backup")
	manifest, err := Create(context.Background(), dbPath, storageDir, dest)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if manifest.Database != "registry.db" || manifest.StorageDir != "data" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	restored, err := sql.Open("sqlite3", filepath.Join(dest, "registry.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var count int
	if err := restored.QueryRow(`SELECT COUNT(*) FROM items WHERE name = 'one'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 backed-up row, got %d", count)
	}
	if _, err := os.Stat(filepath.Join(dest, "data", "blobs", "sha256", "ab", "cd", "blob")); err != nil {
		t.Fatalf("expected storage blob to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "manifest.json")); err != nil {
		t.Fatalf("expected manifest: %v", err)
	}
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"backup-20260101T000000Z", "backup-20260102T000000Z", "backup-20260103T000000Z"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := Prune(dir, 2); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backup-20260101T000000Z")); !os.IsNotExist(err) {
		t.Fatalf("expected oldest backup to be pruned, stat err=%v", err)
	}
	for _, name := range []string{"backup-20260102T000000Z", "backup-20260103T000000Z"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to remain: %v", name, err)
		}
	}
}
