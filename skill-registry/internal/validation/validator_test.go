package validation

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
