package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Manifest describes an on-disk backup bundle.
type Manifest struct {
	CreatedAt  time.Time `json:"created_at"`
	Database   string    `json:"database"`
	StorageDir string    `json:"storage_dir"`
}

// Scheduler periodically creates backup bundles and prunes old copies.
type Scheduler struct {
	dbPath     string
	storageDir string
	outputDir  string
	interval   time.Duration
	retention  int
	logger     *slog.Logger
	stop       chan struct{}
	done       chan struct{}
	once       sync.Once
}

// NewScheduler creates a scheduled backup worker.
func NewScheduler(dbPath, storageDir, outputDir string, interval time.Duration, retention int, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		dbPath:     dbPath,
		storageDir: storageDir,
		outputDir:  outputDir,
		interval:   interval,
		retention:  retention,
		logger:     logger,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Start starts the scheduler in the background.
func (s *Scheduler) Start() {
	go s.run()
}

// Stop asks the scheduler to stop and waits for it.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.once.Do(func() { close(s.stop) })
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scheduler) run() {
	defer close(s.done)
	s.createOnce()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.createOnce()
		case <-s.stop:
			return
		}
	}
}

func (s *Scheduler) createOnce() {
	name := "backup-" + time.Now().UTC().Format("20060102T150405Z")
	dest := filepath.Join(s.outputDir, name)
	if _, err := Create(context.Background(), s.dbPath, s.storageDir, dest); err != nil {
		s.logger.Error("scheduled backup failed", "error", err, "dest", dest)
		return
	}
	s.logger.Info("scheduled backup created", "dest", dest)
	if err := Prune(s.outputDir, s.retention); err != nil {
		s.logger.Error("scheduled backup prune failed", "error", err, "output_dir", s.outputDir)
	}
}

// Create writes a consistent SQLite snapshot plus storage files into destDir.
func Create(ctx context.Context, dbPath, storageDir, destDir string) (*Manifest, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path is required")
	}
	if storageDir == "" {
		return nil, fmt.Errorf("storage directory is required")
	}
	if destDir == "" {
		return nil, fmt.Errorf("backup output directory is required")
	}
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	dbBackupPath := filepath.Join(destDir, "registry.db")
	if err := snapshotSQLite(ctx, dbPath, dbBackupPath); err != nil {
		return nil, err
	}

	storageBackupDir := filepath.Join(destDir, "data")
	if err := copyDir(storageDir, storageBackupDir, map[string]bool{
		filepath.Clean(dbPath):                true,
		filepath.Clean(dbPath + "-wal"):       true,
		filepath.Clean(dbPath + "-shm"):       true,
		filepath.Clean(dbBackupPath):          true,
		filepath.Clean(dbBackupPath + "-wal"): true,
		filepath.Clean(dbBackupPath + "-shm"): true,
	}); err != nil {
		return nil, fmt.Errorf("copy storage: %w", err)
	}

	manifest := &Manifest{
		CreatedAt:  time.Now().UTC(),
		Database:   "registry.db",
		StorageDir: "data",
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(destDir, "manifest.json"), append(data, '\n'), 0640); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}
	return manifest, nil
}

func snapshotSQLite(ctx context.Context, src, dst string) error {
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replace database backup: %w", err)
	}
	db, err := sql.Open("sqlite3", src)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("checkpoint database: %w", err)
	}
	query := "VACUUM INTO '" + strings.ReplaceAll(dst, "'", "''") + "'"
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("snapshot database: %w", err)
	}
	return nil
}

func copyDir(src, dst string, skip map[string]bool) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		cleanPath := filepath.Clean(path)
		if skip[cleanPath] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, cleanPath)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(cleanPath, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// Prune removes oldest backup-* directories until retention is satisfied.
func Prune(outputDir string, retention int) error {
	if retention < 1 {
		return fmt.Errorf("retention must be >= 1")
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var backups []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "backup-") {
			backups = append(backups, filepath.Join(outputDir, entry.Name()))
		}
	}
	sort.Strings(backups)
	for len(backups) > retention {
		if err := os.RemoveAll(backups[0]); err != nil {
			return err
		}
		backups = backups[1:]
	}
	return nil
}
