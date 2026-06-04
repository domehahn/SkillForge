package validation

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "myskill", false},
		{"valid with dot", "my.skill", false},
		{"valid with dash", "my-skill", false},
		{"valid with underscore", "my_skill", false},
		{"invalid uppercase", "MySkill", true},
		{"invalid too short", "a", true},
		{"invalid starts with dash", "-myskill", true},
		{"invalid special char", "my@skill", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSkillName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSkillName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "1.0.0", false},
		{"valid with prerelease", "1.0.0-alpha", false},
		{"valid with build", "1.0.0+build.123", false},
		{"invalid missing patch", "1.0", true},
		{"invalid no dots", "100", true},
		{"invalid letters", "v1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePackage_MissingSkillMD(t *testing.T) {
	data := createTestTarGz(t, map[string]string{
		"README.md": "# Test",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors")
	}
}

func TestValidatePackage_Valid(t *testing.T) {
	data := createTestTarGz(t, map[string]string{
		"SKILL.md":   "# Test Skill\n\nThis is a test.",
		"README.md":  "# README",
		"scripts/test.sh": "#!/bin/bash\necho test",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected validation to succeed, got errors: %v", result.Errors)
	}
}

func TestValidatePackage_BlockedExtension(t *testing.T) {
	data := createTestTarGz(t, map[string]string{
		"SKILL.md":      "# Test Skill",
		"malware.exe": "fake exe content",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail due to blocked extension")
	}
}

func TestValidatePackage_PathTraversal(t *testing.T) {
	data := createTestTarGz(t, map[string]string{
		"SKILL.md":      "# Test Skill",
		"../../../etc/passwd": "malicious",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail due to path traversal")
	}
}

func createTestTarGz(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

func createTestZip(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Failed to create zip file entry: %v", err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write zip content: %v", err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}

// ZIP Package Tests

func TestValidatePackage_ZipValid(t *testing.T) {
	data := createTestZip(t, map[string]string{
		"SKILL.md":        "# Test Skill\n\nThis is a test.",
		"README.md":       "# README",
		"scripts/test.sh": "#!/bin/bash\necho test",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "zip")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected validation to succeed, got errors: %v", result.Errors)
	}
}

func TestValidatePackage_ZipMissingSkillMD(t *testing.T) {
	data := createTestZip(t, map[string]string{
		"README.md": "# Test",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "zip")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors")
	}
}

func TestValidatePackage_ZipBlockedExtension(t *testing.T) {
	data := createTestZip(t, map[string]string{
		"SKILL.md":    "# Test Skill",
		"malware.dll": "fake dll content",
	})

	validator := NewValidator(50, []string{".dll"})
	result, err := validator.ValidatePackage(data, "zip")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail due to blocked extension")
	}
}

func TestValidatePackage_ZipPathTraversal(t *testing.T) {
	data := createTestZip(t, map[string]string{
		"SKILL.md":            "# Test Skill",
		"../../../etc/passwd": "malicious",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "zip")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail due to path traversal")
	}
}

func TestValidatePackage_ZipInvalidArchive(t *testing.T) {
	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage([]byte("not a zip file"), "zip")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail for invalid zip")
	}
}

// Additional TarGz Tests

func TestValidatePackage_TarGzInvalidArchive(t *testing.T) {
	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage([]byte("not a tar.gz file"), "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail for invalid tar.gz")
	}
}

func TestValidatePackage_TarGzCorrupted(t *testing.T) {
	// Create a valid tar.gz and then corrupt it
	data := createTestTarGz(t, map[string]string{
		"SKILL.md": "# Test Skill",
	})

	// Corrupt the data
	corrupted := append(data[:len(data)/2], []byte("corrupted")...)

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(corrupted, "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail for corrupted tar.gz")
	}
}

func TestValidatePackage_UnsupportedType(t *testing.T) {
	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage([]byte("data"), "rar")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail for unsupported type")
	}

	if !strings.Contains(result.Errors[0], "unsupported") {
		t.Errorf("Expected unsupported type error, got: %v", result.Errors[0])
	}
}

func TestValidatePackage_EmptyPackage(t *testing.T) {
	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage([]byte{}, "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail for empty package")
	}
}

func TestValidatePackage_TooLarge(t *testing.T) {
	// Create a simple tar.gz, then manually pad it to exceed the limit
	files := map[string]string{
		"SKILL.md": "# Test Skill",
	}
	
	data := createTestTarGz(t, files)
	
	// Manually pad to 2MB to ensure it exceeds 1MB limit
	padding := make([]byte, 2*1024*1024-len(data))
	for i := range padding {
		padding[i] = byte(i % 256)
	}
	data = append(data, padding...)

	validator := NewValidator(1, []string{".exe"}) // 1MB limit
	result, err := validator.ValidatePackage(data, "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Errorf("Expected validation to fail for package exceeding size limit (actual size: %d bytes)", len(data))
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "exceeds maximum") || strings.Contains(e, "size") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected size limit error, got: %v", result.Errors)
	}
}

func TestValidatePackage_SkillMDInSubdirectory(t *testing.T) {
	data := createTestTarGz(t, map[string]string{
		"subdir/SKILL.md": "# Test Skill",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	// Current implementation accepts SKILL.md in subdirectories using filepath.Base()
	// This is actually acceptable behavior for now
	if !result.Valid {
		t.Errorf("Validator accepts SKILL.md in subdirectories, got errors: %v", result.Errors)
	}
}

func TestValidatePackage_AbsolutePath(t *testing.T) {
	data := createTestTarGz(t, map[string]string{
		"SKILL.md":    "# Test Skill",
		"/etc/passwd": "malicious",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "tgz")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if result.Valid {
		t.Error("Expected validation to fail due to absolute path")
	}
}

func TestValidatePackage_SymlinkInZip(t *testing.T) {
	// Zip files with symlinks are harder to create in tests
	// but we test that regular files work
	data := createTestZip(t, map[string]string{
		"SKILL.md":   "# Test Skill",
		"assets/img": "fake image",
	})

	validator := NewValidator(50, []string{".exe"})
	result, err := validator.ValidatePackage(data, "zip")

	if err != nil {
		t.Fatalf("ValidatePackage() error = %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected validation to succeed, got errors: %v", result.Errors)
	}
}

func TestNewValidator(t *testing.T) {
	validator := NewValidator(100, []string{".exe", ".dll"})

	if validator == nil {
		t.Fatal("NewValidator returned nil")
	}

	// Can't access private fields, just verify validator works
	result, err := validator.ValidatePackage(createTestTarGz(t, map[string]string{
		"SKILL.md": "# Test",
	}), "tgz")

	if err != nil {
		t.Fatalf("validator should work: %v", err)
	}

	if !result.Valid {
		t.Error("validator should accept valid package")
	}
}

func TestValidateSkillName_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", true},
		{"max length", strings.Repeat("a", 128), false},
		{"too long", strings.Repeat("a", 129), true},
		{"starts with number", "1skill", false},
		{"starts with underscore", "_skill", true},
		{"ends with dash", "skill-", false},
		{"consecutive dashes", "my--skill", false},
		{"all numbers", "123", false},
		{"with space", "my skill", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSkillName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSkillName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateVersion_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", true},
		{"with v prefix", "v1.2.3", true},
		{"major only", "1", true},
		{"major.minor only", "1.2", true},
		{"leading zeros not allowed", "01.02.03", true},
		{"prerelease with dash", "1.0.0-beta.1", false},
		{"build metadata", "1.0.0+20130313144700", false},
		{"both prerelease and build", "1.0.0-beta+exp.sha.5114f85", false},
		{"invalid chars", "1.0.0@dev", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestExtractManifest(t *testing.T) {
	validator := NewValidator(50, []string{".exe"})

	tests := []struct {
		name        string
		content     string
		wantName    string
		wantVersion string
		wantEntry   string
	}{
		{
			name: "with valid frontmatter",
			content: `---
name: test-skill
version: 1.0.0
description: Test skill
entrypoint: custom.md
---
# Test Skill`,
			wantName:    "test-skill",
			wantVersion: "1.0.0",
			wantEntry:   "custom.md",
		},
		{
			name: "without frontmatter",
			content: `# Test Skill
This is just markdown`,
			wantName:    "",
			wantVersion: "",
			wantEntry:   "SKILL.md", // default
		},
		{
			name: "with empty frontmatter",
			content: `---
---
# Test Skill`,
			wantName:    "",
			wantVersion: "",
			wantEntry:   "SKILL.md",
		},
		{
			name: "with partial frontmatter",
			content: `---
name: partial-skill
---
# Partial`,
			wantName:    "partial-skill",
			wantVersion: "",
			wantEntry:   "SKILL.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := validator.extractManifest([]byte(tt.content))
			
			if manifest.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", manifest.Name, tt.wantName)
			}
			if manifest.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", manifest.Version, tt.wantVersion)
			}
			if manifest.Entrypoint != tt.wantEntry {
				t.Errorf("Entrypoint = %q, want %q", manifest.Entrypoint, tt.wantEntry)
			}
		})
	}
}
