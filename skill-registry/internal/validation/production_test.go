package validation

import (
	"strings"
	"testing"
)

func TestPublishValidationRequiresCanonicalFiles(t *testing.T) {
	validator := NewValidator(50, nil)
	data := createTestTarGz(t, map[string]string{"SKILL.md": "# Skill\n"})
	result, err := validator.ValidatePackageWithProfile(data, "tgz", ProfilePublish)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatal("publish validation should reject SKILL.md-only package")
	}
	assertHasError(t, result.Errors, "VERSION is required")
	assertHasError(t, result.Errors, "skill.yaml is required")
	assertHasError(t, result.Errors, "CHANGELOG.md is required")
}

func TestPublishValidationRejectsVersionMismatch(t *testing.T) {
	validator := NewValidator(50, nil)
	data := canonicalPackage(t, map[string]string{"VERSION": "1.5.1\n"})
	result, err := validator.ValidatePackageWithProfile(data, "tgz", ProfilePublish)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatal("version mismatch should fail")
	}
	assertHasError(t, result.Errors, "does not match VERSION")
}

func TestPublishValidationRejectsUnknownPlatform(t *testing.T) {
	validator := NewValidator(50, nil)
	data := canonicalPackage(t, map[string]string{"skill.yaml": canonicalSkillYAML("unknown-ai")})
	result, err := validator.ValidatePackageWithProfile(data, "tgz", ProfilePublish)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatal("unknown platform should fail")
	}
	assertHasError(t, result.Errors, "unknown compatible_with platform")
}

func TestPublishValidationRejectsDotEnv(t *testing.T) {
	validator := NewValidator(50, nil)
	data := canonicalPackage(t, map[string]string{".env": "API_KEY=abcdefghijklmnopqrstuvwxyz123456\n"})
	result, err := validator.ValidatePackageWithProfile(data, "tgz", ProfilePublish)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatal(".env must not be packageable")
	}
	// .env is rejected at path level by packageio.IsForbiddenPackagePath before content scanning.
	assertHasError(t, result.Errors, "forbidden path")
}

func TestPublishValidationDetectsLikelySecret(t *testing.T) {
	validator := NewValidator(50, nil)
	data := canonicalPackage(t, map[string]string{"config/secrets.yaml": "api_key: 'abcdefghijklmnopqrstuvwxyz123456'\n"})
	result, err := validator.ValidatePackageWithProfile(data, "tgz", ProfilePublish)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatal("likely secret should fail")
	}
	assertHasError(t, result.Errors, "likely secret")
}

func canonicalPackage(t *testing.T, overrides map[string]string) []byte {
	files := map[string]string{
		"SKILL.md":     "# GitLab Policy Reviewer\n",
		"VERSION":      "1.5.0\n",
		"skill.yaml":   canonicalSkillYAML("codex"),
		"CHANGELOG.md": "# Changelog\n\n## 1.5.0\n\n### Added\n- Added initial skill implementation.\n",
		"README.md":    "# README\n",
	}
	for k, v := range overrides {
		files[k] = v
	}
	return createTestTarGz(t, files)
}

func canonicalSkillYAML(platform string) string {
	return "name: gitlab-policy-reviewer\nnamespace: default\nversion: 1.5.0\ndescription: Reviews GitLab security policies.\nowners:\n  - platform-security\nlicense: MIT\ncompatible_with:\n  - " + platform + "\nentrypoint: SKILL.md\nsecurity:\n  requires_network: false\n  requires_secrets: false\n  writes_files: false\n  runs_commands: false\n"
}

func assertHasError(t *testing.T, errors []string, contains string) {
	t.Helper()
	for _, err := range errors {
		if strings.Contains(err, contains) {
			return
		}
	}
	t.Fatalf("expected error containing %q, got %#v", contains, errors)
}
