package packaging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skillforge/skill-registry/internal/validation"
)

func TestBuildDeterministicPackage(t *testing.T) {
	dir := createCanonicalSkill(t)
	validator := validation.NewValidator(50, nil)
	opts := Options{Format: "tgz", OutputDir: filepath.Join(t.TempDir(), "dist"), SourceCommit: "abc123", Provenance: true}

	first, firstData, err := Build(dir, opts, validator)
	if err != nil {
		t.Fatalf("Build() first error = %v", err)
	}
	second, secondData, err := Build(dir, opts, validator)
	if err != nil {
		t.Fatalf("Build() second error = %v", err)
	}
	if first.SHA256 != second.SHA256 {
		t.Fatalf("digest mismatch: %s != %s", first.SHA256, second.SHA256)
	}
	if string(firstData) != string(secondData) {
		t.Fatal("package bytes are not deterministic")
	}
	if first.FileName != "gitlab-policy-reviewer-1.5.0.tgz" {
		t.Fatalf("unexpected file name: %s", first.FileName)
	}
	if first.Manifest.SourceCommit != "abc123" {
		t.Fatalf("source commit not recorded")
	}
}

func createCanonicalSkill(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"SKILL.md": "# GitLab Policy Reviewer\n\nReviews policies.\n",
		"VERSION":  "1.5.0\n",
		"skill.yaml": `name: gitlab-policy-reviewer
namespace: default
version: 1.5.0
description: Reviews GitLab security policies.
owners:
  - platform-security
license: MIT
compatible_with:
  - codex
  - gitlab-duo
entrypoint: SKILL.md
tags:
  - security
security:
  requires_network: false
  requires_secrets: false
  writes_files: false
  runs_commands: false
`,
		"CHANGELOG.md": "# Changelog\n\n## 1.5.0\n\n### Added\n- Added initial skill implementation.\n",
		"README.md":    "# GitLab Policy Reviewer\n",
		".git/config":  "[core]\nrepositoryformatversion = 0\n",
	}
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
