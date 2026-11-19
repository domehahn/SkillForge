package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/skillforge/skill-registry/internal/metadata"
)

func TestArtifactRegistryDependencyGraphAndLockfile(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()
	ctx := context.Background()

	skill := artifactPackage(t, map[string]string{
		"artifact.yaml": artifactManifest("skill", "demo", "review-skill", "1.2.0", "SKILL.md", ""),
		"SKILL.md":      "# Review Skill",
	})
	if _, err := reg.PublishArtifact(ctx, "skill", "demo", "review-skill", "1.2.0", skill, "tgz", "tester"); err != nil {
		t.Fatal(err)
	}

	agent := artifactPackage(t, map[string]string{
		"artifact.yaml": artifactManifest("agent", "demo", "reviewer", "1.0.0", "AGENT.md", `
    - kind: skill
      namespace: demo
      name: review-skill
      version: ^1.0.0`),
		"AGENT.md": "# Reviewer",
	})
	result, err := reg.PublishArtifact(ctx, "agent", "demo", "reviewer", "1.0.0", agent, "tgz", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Version.Lockfile.Resolved) != 1 || result.Version.Lockfile.Resolved[0].Version != "1.2.0" {
		t.Fatalf("unexpected lockfile: %#v", result.Version.Lockfile)
	}

	graph, err := reg.ArtifactGraph(ctx, "agent", "demo", "reviewer", "latest")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("unexpected graph: %#v", graph)
	}
	if err := reg.PromoteArtifact(ctx, "agent", "demo", "reviewer", "1.0.0", "production", "tester"); err != nil {
		t.Fatal(err)
	}
	resolved, err := reg.ResolveArtifactVersion(ctx, "agent", "demo", "reviewer", "production")
	if err != nil || resolved.Version != "1.0.0" {
		t.Fatalf("promotion did not resolve: %#v %v", resolved, err)
	}
}

func TestActivationRequiresScanApprovalAndRejectsStaleReports(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()
	ctx := context.Background()
	digest := strings.Repeat("a", 64)
	manifest := "apiVersion: skillforge.dev/v2\nkind: model-runtime\nmetadata:\n  namespace: demo\n  name: local-llm\n  version: 1.0.0\nspec:\n  entrypoint: runtime.yaml\n  model_runtime:\n    engine: ollama\n    image: ollama/ollama@sha256:" + digest + "\n"
	data := artifactPackage(t, map[string]string{"artifact.yaml": manifest, "runtime.yaml": "engine: ollama"})
	result, err := reg.PublishArtifact(ctx, "model-runtime", "demo", "local-llm", "1.0.0", data, "tgz", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if result.Version.ScanStatus != "pending" {
		t.Fatalf("expected pending scan, got %q", result.Version.ScanStatus)
	}
	if _, err := reg.ChangeActivation(ctx, "demo", "model-runtime", "local-llm", "1.0.0", "install", "test", "owner", false); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.ChangeActivation(ctx, "demo", "model-runtime", "local-llm", "", "approve", "test", "owner", false); err == nil {
		t.Fatal("approval should require a passing scan")
	}
	if err := reg.SetArtifactAttestation(ctx, "model-runtime", "demo", "local-llm", "1.0.0", "verified", "passed"); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.ChangeActivation(ctx, "demo", "model-runtime", "local-llm", "", "approve", "reviewed", "owner", true); err != nil {
		t.Fatal(err)
	}
	a, err := reg.ChangeActivation(ctx, "demo", "model-runtime", "local-llm", "", "activate", "approved", "owner", true)
	if err != nil {
		t.Fatal(err)
	}
	if a.DesiredState != metadata.ActivationStateActive {
		t.Fatalf("unexpected state %q", a.DesiredState)
	}
	if _, err := reg.ReportActivation(ctx, "demo", "model-runtime", "local-llm", "active", "", "controller", a.Generation-1); err == nil {
		t.Fatal("stale generation should be rejected")
	}
	if _, err := reg.ReportActivation(ctx, "demo", "model-runtime", "local-llm", "active", "", "controller", a.Generation); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactWebhookDeliveryNetwork(t *testing.T) {
	if os.Getenv("SKILLFORGE_ENABLE_NETWORK_TESTS") != "1" {
		t.Skip("network listener disabled")
	}
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()
	ctx := context.Background()

	delivered := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delivered <- r.Header.Get("X-SkillForge-Event")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if _, err := reg.CreateWebhook(ctx, "demo", server.URL, []string{"artifact.published"}); err != nil {
		t.Fatal(err)
	}
	data := artifactPackage(t, map[string]string{
		"artifact.yaml": artifactManifest("prompt", "demo", "summary", "1.0.0", "PROMPT.md", ""),
		"PROMPT.md":     "# Summary",
	})
	if _, err := reg.PublishArtifact(ctx, "prompt", "demo", "summary", "1.0.0", data, "tgz", "alice"); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-delivered:
		if event != "artifact.published" {
			t.Fatalf("unexpected event %q", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook was not delivered")
	}
}

func TestArtifactNamespaceACLAndWebhookRegistration(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()
	ctx := context.Background()

	if _, err := reg.UpsertNamespaceMember(ctx, "demo", "alice", "owner"); err != nil {
		t.Fatal(err)
	}
	if allowed, err := reg.AuthorizeNamespace(ctx, "demo", "alice", "owner"); err != nil || !allowed {
		t.Fatalf("owner should be authorized: %v", err)
	}
	if allowed, err := reg.AuthorizeNamespace(ctx, "demo", "bob", "reader"); err != nil || allowed {
		t.Fatalf("unknown member should not be authorized: %v", err)
	}

	if _, err := reg.CreateWebhook(ctx, "demo", "https://example.invalid/hook", []string{"artifact.published"}); err != nil {
		t.Fatal(err)
	}
	hooks, err := reg.ListWebhooks(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 1 || hooks[0].Events[0] != "artifact.published" {
		t.Fatalf("unexpected webhooks: %#v", hooks)
	}
}

func TestArtifactRegistryRejectsMissingDependency(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()
	data := artifactPackage(t, map[string]string{
		"artifact.yaml": artifactManifest("agent", "demo", "broken", "1.0.0", "AGENT.md", `
    - kind: skill
      namespace: demo
      name: missing
      version: latest`),
		"AGENT.md": "# Broken",
	})
	if _, err := reg.PublishArtifact(context.Background(), "agent", "demo", "broken", "1.0.0", data, "tgz", "tester"); err == nil {
		t.Fatal("expected missing dependency to fail")
	}
}

func TestArtifactRegistrySupportsPromptToolAndBundle(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()
	ctx := context.Background()
	cases := []struct {
		kind       string
		name       string
		entrypoint string
	}{
		{"prompt", "summary-prompt", "PROMPT.md"},
		{"tool", "search-tool", "TOOL.md"},
		{"bundle", "review-bundle", "BUNDLE.md"},
	}
	for _, tt := range cases {
		t.Run(tt.kind, func(t *testing.T) {
			data := artifactPackage(t, map[string]string{
				"artifact.yaml": artifactManifest(tt.kind, "demo", tt.name, "1.0.0", tt.entrypoint, ""),
				tt.entrypoint:   "# " + tt.name,
			})
			if _, err := reg.PublishArtifact(ctx, tt.kind, "demo", tt.name, "1.0.0", data, "tgz", "tester"); err != nil {
				t.Fatal(err)
			}
			resolved, err := reg.ResolveArtifactVersion(ctx, tt.kind, "demo", tt.name, "latest")
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Kind != tt.kind {
				t.Fatalf("expected kind %s, got %s", tt.kind, resolved.Kind)
			}
		})
	}
}

func TestArtifactAttestationsUpdateSignatureAndScanMetadata(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()
	ctx := context.Background()
	data := artifactPackage(t, map[string]string{
		"artifact.yaml": artifactManifest("prompt", "demo", "summary", "1.0.0", "PROMPT.md", ""),
		"PROMPT.md":     "# Summary",
	})
	if _, err := reg.PublishArtifact(ctx, "prompt", "demo", "summary", "1.0.0", data, "tgz", "tester"); err != nil {
		t.Fatal(err)
	}
	version, err := reg.ResolveArtifactVersion(ctx, "prompt", "demo", "summary", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	previousSignature, previousScan := version.SignatureStatus, version.ScanStatus
	digest := version.DigestSHA256
	predicate := map[string]interface{}{"version": 1, "subject": map[string]interface{}{"sha256": digest}}
	for _, typ := range []string{"signature", "scan"} {
		if _, err := reg.CreateArtifactAttestation(ctx, "prompt", "demo", "summary", "1.0.0", typ, digest, "tester", predicate); err != nil {
			t.Fatal(err)
		}
	}
	bad := map[string]interface{}{"version": 1, "subject": map[string]interface{}{"sha256": "wrong"}}
	for _, tc := range []struct {
		digest    string
		predicate map[string]interface{}
	}{
		{"wrong", predicate}, {digest, bad}, {digest, nil},
	} {
		if _, err := reg.CreateArtifactAttestation(ctx, "prompt", "demo", "summary", "1.0.0", "scan", tc.digest, "tester", tc.predicate); err == nil {
			t.Fatal("accepted unbound evidence")
		}
	}
	version, err = reg.ResolveArtifactVersion(ctx, "prompt", "demo", "summary", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if version.SignatureStatus != previousSignature || version.ScanStatus != previousScan {
		t.Fatalf("storage incorrectly promoted unverified evidence: %+v", version)
	}

	attestations, err := reg.ListArtifactAttestations(ctx, "prompt", "demo", "summary", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(attestations) != 2 {
		t.Fatalf("expected two attestations, got %d", len(attestations))
	}
}

func artifactManifest(kind, namespace, name, version, entrypoint, dependencies string) string {
	return fmt.Sprintf(`apiVersion: skillforge.dev/v1
kind: %s
metadata:
  namespace: %s
  name: %s
  version: %s
  description: test artifact
spec:
  entrypoint: %s
  dependencies:%s
`, kind, namespace, name, version, entrypoint, dependencies)
}

func artifactPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
