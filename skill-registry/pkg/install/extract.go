package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Marker is written to <skill-dir>/.agent-skill-install.json after a successful install.
type Marker struct {
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	Version     string    `json:"version"`
	Digest      string    `json:"digest"`
	Registry    string    `json:"registry"`
	InstalledAt time.Time `json:"installed_at"`
	Files       []string  `json:"files"`
}

// ExtractTarGz extracts a gzipped tar archive into targetDir.
func ExtractTarGz(data []byte, targetDir string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, err := SafeExtractionPath(targetDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(f, tr)
			f.Close()
			if copyErr != nil {
				return copyErr
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("unsafe symlink or hardlink rejected: %s", header.Name)
		}
	}
	return nil
}

// ExtractZip extracts a ZIP archive into targetDir.
func ExtractZip(data []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		target, err := SafeExtractionPath(targetDir, entry.Name)
		if err != nil {
			return err
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if entry.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe symlink rejected: %s", entry.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		src, err := entry.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

// SafeExtractionPath resolves name relative to root and rejects path traversal.
func SafeExtractionPath(root, name string) (string, error) {
	target := filepath.Join(root, name)
	rootClean := filepath.Clean(root)
	if target != rootClean && !strings.HasPrefix(filepath.Clean(target), rootClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive path escapes target directory: %s", name)
	}
	return target, nil
}

// WriteMarker writes m as JSON to <dir>/.agent-skill-install.json.
func WriteMarker(dir string, m Marker) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".agent-skill-install.json"), append(data, '\n'), 0644)
}

// ListFiles returns a sorted list of all files under root (relative paths, slash-separated).
func ListFiles(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err == nil {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files
}
