package packaging

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/domehahn/sklib/spec"
	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/validation"
)

const normalizedTimestamp = "1970-01-01T00:00:00Z"

type Options struct {
	Format       string
	OutputDir    string
	SourceCommit string
	Provenance   bool
	Sign         bool
	DryRun       bool
}

type Result struct {
	Path          string                  `json:"path"`
	FileName      string                  `json:"file_name"`
	Name          string                  `json:"name"`
	Namespace     string                  `json:"namespace"`
	Version       string                  `json:"version"`
	SHA256        string                  `json:"sha256"`
	PackageType   string                  `json:"package_type"`
	Manifest      spec.PackageManifest    `json:"manifest"`
	SkillManifest *metadata.SkillManifest `json:"skill_manifest,omitempty"`
	Warnings      []string                `json:"warnings,omitempty"`
	Signature     string                  `json:"signature,omitempty"`
}

type fileEntry struct {
	Rel     string
	Source  string
	Content []byte
	Mode    int64
}

func Build(skillDir string, opts Options, validator *validation.Validator) (*Result, []byte, error) {
	if opts.Format == "" {
		opts.Format = "tgz"
	}
	if opts.OutputDir == "" {
		opts.OutputDir = "dist"
	}
	if opts.Format != "tgz" && opts.Format != "zip" {
		return nil, nil, fmt.Errorf("unsupported package format %q", opts.Format)
	}
	if validator == nil {
		validator = validation.NewValidator(1024, nil)
	}
	entries, err := collectFiles(skillDir)
	if err != nil {
		return nil, nil, err
	}
	preflightData, err := writeArchive(entries, opts.Format)
	if err != nil {
		return nil, nil, err
	}
	validationResult, err := validator.ValidatePackageWithProfile(preflightData, opts.Format, validation.ProfileStrict)
	if err != nil {
		return nil, nil, err
	}
	if !validationResult.Valid {
		return nil, nil, fmt.Errorf("strict validation failed: %s", strings.Join(validationResult.Errors, "; "))
	}
	manifest := validationResult.Metadata
	if manifest == nil {
		return nil, nil, fmt.Errorf("skill.yaml metadata is required")
	}

	checksums := checksumsText(entries)
	entries = append(entries, fileEntry{Rel: "checksums.txt", Content: []byte(checksums), Mode: 0644})

	filePaths := make([]string, len(entries))
	for i, e := range entries {
		filePaths[i] = e.Rel
	}
	sort.Strings(filePaths)

	pkgManifest := spec.PackageManifest{
		SpecVersion:    1,
		Name:           manifest.Name,
		Namespace:      manifest.Namespace,
		Version:        manifest.Version,
		Description:    manifest.Description,
		Entrypoint:     manifest.Entrypoint,
		CompatibleWith: manifest.CompatibleWith,
		PackageType:    opts.Format,
		SourceCommit:   opts.SourceCommit,
		PackagedBy:     "skforge",
		PackagedAt:     normalizedTimestamp,
		License:        manifest.License,
		Files:          filePaths,
	}
	if opts.Provenance && opts.SourceCommit != "" {
		pkgManifest.Provenance = "source:" + opts.SourceCommit
	}
	manifestBytes, err := json.MarshalIndent(pkgManifest, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	entries = append(entries, fileEntry{Rel: "manifest.json", Content: append(manifestBytes, '\n'), Mode: 0644})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Rel < entries[j].Rel })

	data, err := writeArchive(entries, opts.Format)
	if err != nil {
		return nil, nil, err
	}
	digest := sha256.Sum256(data)
	digestHex := hex.EncodeToString(digest[:])
	pkgManifest.SHA256 = digestHex
	result := &Result{
		FileName: fmt.Sprintf("%s-%s.%s", manifest.Name, manifest.Version, opts.Format),
		Name:     manifest.Name, Namespace: manifest.Namespace, Version: manifest.Version,
		SHA256: digestHex, PackageType: opts.Format, Manifest: pkgManifest,
		SkillManifest: metadata.SkillManifestFromSpec(manifest), Warnings: validationResult.Warnings,
	}
	if opts.Sign {
		result.Signature = LocalSignature(digestHex)
	}
	if !opts.DryRun {
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			return nil, nil, err
		}
		result.Path = filepath.Join(opts.OutputDir, result.FileName)
		if err := os.WriteFile(result.Path, data, 0644); err != nil {
			return nil, nil, err
		}
	}
	return result, data, nil
}

func LocalSignature(digest string) string {
	sum := sha256.Sum256([]byte("skillforge-local-signer:" + digest))
	return "local-sha256:" + hex.EncodeToString(sum[:])
}

func collectFiles(root string) ([]fileEntry, error) {
	var entries []fileEntry
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldExclude(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed: %s", rel)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, fileEntry{Rel: rel, Source: path, Content: content, Mode: normalizedMode(info.Mode(), rel)})
		return nil
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Rel < entries[j].Rel })
	return entries, err
}

func shouldExclude(rel string, entry os.DirEntry) bool {
	base := filepath.Base(rel)
	if base == ".git" || base == ".DS_Store" || base == ".idea" || base == ".vscode" {
		return true
	}
	for _, part := range strings.Split(rel, "/") {
		switch part {
		case "node_modules", ".venv", "target", "dist", ".cache", "__pycache__":
			return true
		}
	}
	return strings.HasSuffix(base, "~") || strings.HasSuffix(base, ".swp")
}

func normalizedMode(mode os.FileMode, rel string) int64 {
	if mode&0111 != 0 || strings.HasSuffix(rel, ".sh") {
		return 0755
	}
	return 0644
}

func checksumsText(entries []fileEntry) string {
	var b strings.Builder
	for _, e := range entries {
		sum := sha256.Sum256(e.Content)
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(sum[:]), e.Rel)
	}
	return b.String()
}

func writeArchive(entries []fileEntry, format string) ([]byte, error) {
	switch format {
	case "zip":
		return writeZip(entries)
	default:
		return writeTgz(entries)
	}
}

func writeTgz(entries []fileEntry) ([]byte, error) {
	var buf bytes.Buffer
	gzw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	gzw.Name = ""
	gzw.ModTime = time.Unix(0, 0).UTC()
	tw := tar.NewWriter(gzw)
	for _, e := range entries {
		header := &tar.Header{
			Name: e.Rel, Mode: e.Mode, Size: int64(len(e.Content)),
			ModTime: time.Unix(0, 0).UTC(), AccessTime: time.Unix(0, 0).UTC(), ChangeTime: time.Unix(0, 0).UTC(),
			Typeflag: tar.TypeReg, Uid: 0, Gid: 0, Uname: "", Gname: "",
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tw.Write(e.Content); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeZip(entries []fileEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		header := &zip.FileHeader{Name: e.Rel, Method: zip.Deflate}
		header.SetMode(os.FileMode(e.Mode))
		header.Modified = time.Unix(0, 0).UTC()
		w, err := zw.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(w, bytes.NewReader(e.Content)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
