package api_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/domehahn/sklib/registryapi"
	"github.com/go-chi/chi/v5"
	"github.com/skillforge/skill-registry/internal/api"
	"github.com/skillforge/skill-registry/internal/auth"
	"github.com/skillforge/skill-registry/internal/config"
	"github.com/skillforge/skill-registry/internal/metadata"
	"github.com/skillforge/skill-registry/internal/observability"
	"github.com/skillforge/skill-registry/internal/registry"
	"github.com/skillforge/skill-registry/internal/storage"
	"github.com/skillforge/skill-registry/internal/validation"
)

// testServer wires up a full in-process server backed by real SQLite + filesystem storage.
type testServer struct {
	*httptest.Server
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	tmpDir := t.TempDir()

	repo, err := metadata.NewRepository(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("metadata.NewRepository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	store, err := storage.NewStorage(filepath.Join(tmpDir, "blobs"))
	if err != nil {
		t.Fatalf("storage.NewStorage: %v", err)
	}

	validator := validation.NewValidator(50, nil)
	logger := observability.NewLogger()
	reg := registry.NewRegistry(repo, store, validator, logger)

	cfg := config.DefaultConfig()
	cfg.Auth.Enabled = false

	authenticator, err := auth.NewAuthenticator(false, repo.GetDB())
	if err != nil {
		t.Fatalf("auth.NewAuthenticator: %v", err)
	}

	handler := api.NewHandler(reg, authenticator, logger, cfg)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &testServer{Server: srv}
}

func (ts *testServer) url(path string) string {
	return ts.URL + path
}

func (ts *testServer) publish(t *testing.T, namespace, name, version string, pkg []byte) {
	t.Helper()
	req, _ := http.NewRequest("PUT",
		ts.url(fmt.Sprintf("/api/v1/skills/%s/%s/versions/%s", namespace, name, version)),
		bytes.NewReader(pkg))
	req.Header.Set("Content-Type", "application/gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("publish request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var body map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&body)
		t.Fatalf("publish: expected 201, got %d — %v", resp.StatusCode, body)
	}
}

// buildTgzPackage creates a valid publish-grade tgz for the given skill.
func buildTgzPackage(t *testing.T, namespace, name, version string) []byte {
	t.Helper()
	files := map[string]string{
		"SKILL.md": "# " + name + "\n",
		"VERSION":  version + "\n",
		"skill.yaml": fmt.Sprintf(
			"name: %s\nnamespace: %s\nversion: %s\ndescription: test\nowners:\n  - test\n"+
				"license: MIT\ncompatible_with:\n  - codex\nentrypoint: SKILL.md\n"+
				"security:\n  requires_network: false\n  requires_secrets: false\n"+
				"  writes_files: false\n  runs_commands: false\n",
			name, namespace, version),
		"CHANGELOG.md": fmt.Sprintf("# Changelog\n\n## %s\n\n### Added\n- initial\n", version),
		"manifest.json": fmt.Sprintf(
			`{"spec_version":1,"name":%q,"namespace":%q,"version":%q,"entrypoint":"SKILL.md",`+
				`"compatible_with":["codex"],"package_type":"tgz","packaged_by":"skforge","packaged_at":"1970-01-01T00:00:00Z"}`,
			name, namespace, version),
		"checksums.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  SKILL.md\n",
	}
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for fname, content := range files {
		hdr := &tar.Header{Name: fname, Mode: 0644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gzw.Close()
	return buf.Bytes()
}

func buildZipPackage(t *testing.T, namespace, name, version string) []byte {
	t.Helper()
	files := map[string]string{
		"SKILL.md": "# " + name + "\n",
		"VERSION":  version + "\n",
		"skill.yaml": fmt.Sprintf(
			"name: %s\nnamespace: %s\nversion: %s\ndescription: test\nowners:\n  - test\n"+
				"license: MIT\ncompatible_with:\n  - codex\nentrypoint: SKILL.md\n"+
				"security:\n  requires_network: false\n  requires_secrets: false\n"+
				"  writes_files: false\n  runs_commands: false\n",
			name, namespace, version),
		"CHANGELOG.md": fmt.Sprintf("# Changelog\n\n## %s\n\n### Added\n- initial\n", version),
		"manifest.json": fmt.Sprintf(
			`{"spec_version":1,"name":%q,"namespace":%q,"version":%q,"entrypoint":"SKILL.md",`+
				`"compatible_with":["codex"],"package_type":"zip","packaged_by":"skforge","packaged_at":"1970-01-01T00:00:00Z"}`,
			name, namespace, version),
		"checksums.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  SKILL.md\n",
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for fname, content := range files {
		w, err := zw.Create(fname)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	zw.Close()
	return buf.Bytes()
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func jsonBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	return body
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// 1. GET /api/v1/capabilities returns all registryapi.CapabilitiesResponse fields
//    plus SkillForge-specific extensions.
func TestCapabilitiesMatchesContract(t *testing.T) {
	srv := newTestServer(t)

	resp, err := http.Get(srv.url("/api/v1/capabilities"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := jsonBody(t, resp)

	for _, field := range []string{"api_version", "package_types", "max_package_size_bytes"} {
		if _, ok := body[field]; !ok {
			t.Errorf("capabilities missing contract field %q", field)
		}
	}
	for _, field := range []string{"auth_mode", "auth_enabled", "supported_platforms"} {
		if _, ok := body[field]; !ok {
			t.Errorf("capabilities missing SkillForge extension field %q", field)
		}
	}
	if v, ok := body["api_version"].(string); !ok || v == "" {
		t.Errorf("api_version must be a non-empty string, got %v", body["api_version"])
	}
}

// 2. PUT publish returns a well-formed registryapi.PublishResponse.
func TestPublishReturnsContractDTO(t *testing.T) {
	srv := newTestServer(t)
	pkg := buildTgzPackage(t, "test", "my-skill", "1.0.0")

	req, _ := http.NewRequest("PUT", srv.url("/api/v1/skills/test/my-skill/versions/1.0.0"), bytes.NewReader(pkg))
	req.Header.Set("Content-Type", "application/gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, jsonBody(t, resp))
	}

	var body registryapi.PublishResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Namespace != "test" || body.Name != "my-skill" || body.Version != "1.0.0" {
		t.Errorf("wrong identity in PublishResponse: %+v", body)
	}
	if body.SHA256 == "" {
		t.Error("PublishResponse.sha256 must not be empty")
	}
	if body.DownloadURL == "" {
		t.Error("PublishResponse.download_url must not be empty")
	}
	if !body.Created {
		t.Error("PublishResponse.created must be true for a new publish")
	}
}

// 3. GET /resolve returns a well-formed registryapi.ResolveResponse.
func TestResolveReturnsContractDTO(t *testing.T) {
	srv := newTestServer(t)
	srv.publish(t, "test", "my-skill", "2.1.0", buildTgzPackage(t, "test", "my-skill", "2.1.0"))

	resp, err := http.Get(srv.url("/api/v1/skills/test/my-skill/resolve"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body registryapi.ResolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Version != "2.1.0" {
		t.Errorf("version: want 2.1.0, got %q", body.Version)
	}
	if body.SHA256 == "" {
		t.Error("ResolveResponse.sha256 must not be empty")
	}
	if body.DownloadURL == "" {
		t.Error("ResolveResponse.download_url must not be empty")
	}
	if body.PackageType == "" {
		t.Error("ResolveResponse.package_type must not be empty")
	}
	if len(body.CompatibleWith) == 0 {
		t.Error("ResolveResponse.compatible_with must not be empty")
	}
}

// 4. Publish rejects a package missing manifest.json.
func TestPublishRejectsMissingManifest(t *testing.T) {
	srv := newTestServer(t)
	files := map[string]string{
		"SKILL.md":     "# hi\n",
		"VERSION":      "1.0.0\n",
		"skill.yaml":   "name: my-skill\nnamespace: test\nversion: 1.0.0\ndescription: t\nowners:\n  - t\nlicense: MIT\ncompatible_with:\n  - codex\nentrypoint: SKILL.md\nsecurity:\n  requires_network: false\n  requires_secrets: false\n  writes_files: false\n  runs_commands: false\n",
		"CHANGELOG.md": "# Changelog\n\n## 1.0.0\n\n### Added\n- init\n",
		"checksums.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  SKILL.md\n",
		// manifest.json intentionally omitted
	}
	pkg := buildTgzFromMap(t, files)

	req, _ := http.NewRequest("PUT", srv.url("/api/v1/skills/test/my-skill/versions/1.0.0"), bytes.NewReader(pkg))
	req.Header.Set("Content-Type", "application/gzip")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing manifest.json, got %d", resp.StatusCode)
	}
	body := jsonBody(t, resp)
	if body["error"] == nil {
		t.Error("error response must contain 'error' field")
	}
}

// 5. Publish rejects a package missing checksums.txt.
func TestPublishRejectsMissingChecksums(t *testing.T) {
	srv := newTestServer(t)
	files := map[string]string{
		"SKILL.md":      "# hi\n",
		"VERSION":       "1.0.0\n",
		"skill.yaml":    "name: my-skill\nnamespace: test\nversion: 1.0.0\ndescription: t\nowners:\n  - t\nlicense: MIT\ncompatible_with:\n  - codex\nentrypoint: SKILL.md\nsecurity:\n  requires_network: false\n  requires_secrets: false\n  writes_files: false\n  runs_commands: false\n",
		"CHANGELOG.md":  "# Changelog\n\n## 1.0.0\n\n### Added\n- init\n",
		"manifest.json": `{"spec_version":1,"name":"my-skill","namespace":"test","version":"1.0.0","entrypoint":"SKILL.md","compatible_with":["codex"],"package_type":"tgz","packaged_by":"skforge","packaged_at":"1970-01-01T00:00:00Z"}`,
		// checksums.txt intentionally omitted
	}
	pkg := buildTgzFromMap(t, files)

	req, _ := http.NewRequest("PUT", srv.url("/api/v1/skills/test/my-skill/versions/1.0.0"), bytes.NewReader(pkg))
	req.Header.Set("Content-Type", "application/gzip")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing checksums.txt, got %d", resp.StatusCode)
	}
}

// 6. Publish accepts a valid ZIP package (skpm package --format zip).
func TestPublishAcceptsZipPackage(t *testing.T) {
	srv := newTestServer(t)
	pkg := buildZipPackage(t, "test", "zip-skill", "1.0.0")

	req, _ := http.NewRequest("PUT", srv.url("/api/v1/skills/test/zip-skill/versions/1.0.0"), bytes.NewReader(pkg))
	req.Header.Set("Content-Type", "application/zip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for ZIP package, got %d: %v", resp.StatusCode, jsonBody(t, resp))
	}

	var body registryapi.PublishResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.SHA256 == "" {
		t.Error("PublishResponse.sha256 must not be empty for ZIP publish")
	}
}

// 7. Download returns the stored bytes and an X-Skill-Digest-SHA256 header that
//    matches the actual content hash.
func TestDownloadMatchesStoredSHA256(t *testing.T) {
	srv := newTestServer(t)
	pkg := buildTgzPackage(t, "test", "my-skill", "1.0.0")
	srv.publish(t, "test", "my-skill", "1.0.0", pkg)

	resp, err := http.Get(srv.url("/api/v1/skills/test/my-skill/versions/1.0.0/download"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	digestHeader := resp.Header.Get("X-Skill-Digest-SHA256")
	if digestHeader == "" {
		t.Fatal("X-Skill-Digest-SHA256 header must be present")
	}

	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	actualDigest := sha256hex(buf.Bytes())

	if actualDigest != digestHeader {
		t.Errorf("digest mismatch: header=%q actual=%q", digestHeader, actualDigest)
	}
}

// 8. Resolve excludes yanked versions and returns the next best stable version.
func TestResolveExcludesYankedVersions(t *testing.T) {
	srv := newTestServer(t)
	srv.publish(t, "test", "my-skill", "1.0.0", buildTgzPackage(t, "test", "my-skill", "1.0.0"))
	srv.publish(t, "test", "my-skill", "1.1.0", buildTgzPackage(t, "test", "my-skill", "1.1.0"))

	yankReq, _ := http.NewRequest("POST",
		srv.url("/api/v1/skills/test/my-skill/versions/1.1.0/yank"),
		bytes.NewBufferString(`{"reason":"broken"}`))
	yankReq.Header.Set("Content-Type", "application/json")
	yankResp, _ := http.DefaultClient.Do(yankReq)
	yankResp.Body.Close()
	if yankResp.StatusCode != http.StatusOK {
		t.Fatalf("yank: expected 200, got %d", yankResp.StatusCode)
	}

	resp, err := http.Get(srv.url("/api/v1/skills/test/my-skill/resolve"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resolve: expected 200, got %d", resp.StatusCode)
	}

	var body registryapi.ResolveResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Version != "1.0.0" {
		t.Errorf("resolve should return 1.0.0 after yanking 1.1.0, got %q", body.Version)
	}
}

// 9. Resolve falls back to deprecated versions when no stable version satisfies.
func TestResolveFallsBackToDeprecatedWhenNothingElse(t *testing.T) {
	srv := newTestServer(t)
	srv.publish(t, "test", "my-skill", "1.0.0", buildTgzPackage(t, "test", "my-skill", "1.0.0"))

	deprecateReq, _ := http.NewRequest("POST",
		srv.url("/api/v1/skills/test/my-skill/versions/1.0.0/deprecate"),
		bytes.NewBufferString(`{"reason":"old"}`))
	deprecateReq.Header.Set("Content-Type", "application/json")
	deprecateResp, _ := http.DefaultClient.Do(deprecateReq)
	deprecateResp.Body.Close()
	if deprecateResp.StatusCode != http.StatusOK {
		t.Fatalf("deprecate: expected 200, got %d", deprecateResp.StatusCode)
	}

	resp, err := http.Get(srv.url("/api/v1/skills/test/my-skill/resolve"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resolve: expected 200, got %d", resp.StatusCode)
	}

	var body registryapi.ResolveResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Version != "1.0.0" {
		t.Errorf("resolve should fall back to deprecated 1.0.0, got %q", body.Version)
	}
}

// 10. Resolve returns 404 when all versions are yanked.
func TestResolveReturns404WhenAllYanked(t *testing.T) {
	srv := newTestServer(t)
	srv.publish(t, "test", "my-skill", "1.0.0", buildTgzPackage(t, "test", "my-skill", "1.0.0"))

	yankReq, _ := http.NewRequest("POST",
		srv.url("/api/v1/skills/test/my-skill/versions/1.0.0/yank"),
		bytes.NewBufferString(`{"reason":"all broken"}`))
	yankReq.Header.Set("Content-Type", "application/json")
	yankResp, _ := http.DefaultClient.Do(yankReq)
	yankResp.Body.Close()

	resp, err := http.Get(srv.url("/api/v1/skills/test/my-skill/resolve"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 when all versions yanked, got %d", resp.StatusCode)
	}
	body := jsonBody(t, resp)
	if body["error"] == nil {
		t.Error("error response must contain 'error' field")
	}
}

// ── low-level helpers ─────────────────────────────────────────────────────────

func buildTgzFromMap(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gzw.Close()
	return buf.Bytes()
}
