package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func response(req *http.Request, status int, headers http.Header, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

func TestFetchSkillVerifiesMetadataAndPackageDigest(t *testing.T) {
	packageData := []byte("verified package bytes")
	digestBytes := sha256.Sum256(packageData)
	digest := hex.EncodeToString(digestBytes[:])
	metadataBytes, _ := json.Marshal(map[string]interface{}{
		"namespace": "demo", "name": "review", "version": "1.2.3",
		"digest_sha256": digest, "package_type": "tgz", "size_bytes": len(packageData),
	})
	client := NewClient([]Upstream{{URL: "https://registry.example.com", BearerToken: "read-token"}}, int64(len(packageData)))
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer read-token" {
			t.Fatalf("upstream bearer token missing on %s", r.URL.Path)
		}
		if r.URL.Path == "/api/v1/skills/demo/review/versions/1.2.3" {
			return response(r, http.StatusOK, http.Header{"Content-Type": []string{"application/json"}}, string(metadataBytes)), nil
		}
		if r.URL.Path == "/api/v1/skills/demo/review/versions/1.2.3/download" {
			return response(r, http.StatusOK, http.Header{"X-Skill-Digest-Sha256": []string{digest}}, string(packageData)), nil
		}
		return response(r, http.StatusNotFound, nil, ""), nil
	})
	fetched, err := client.FetchSkill(context.Background(), "demo", "review", "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Version.DigestSHA256 != digest || string(fetched.Data) != string(packageData) {
		t.Fatalf("unexpected fetched package: version=%+v data=%q", fetched.Version, fetched.Data)
	}
}

func TestFetchSkillRejectsDigestMismatchAndRedirect(t *testing.T) {
	packageData := []byte("tampered")
	declaredBytes := sha256.Sum256([]byte("trusted"))
	digest := hex.EncodeToString(declaredBytes[:])
	redirect := false
	metadataBytes, _ := json.Marshal(map[string]interface{}{
		"namespace": "demo", "name": "review", "version": "1.2.3",
		"digest_sha256": digest, "package_type": "zip", "size_bytes": len(packageData),
	})
	client := NewClient([]Upstream{{URL: "https://registry.example.com"}}, 1024)
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/skills/demo/review/versions/1.2.3" {
			return response(r, http.StatusOK, nil, string(metadataBytes)), nil
		}
		if redirect {
			return response(r, http.StatusFound, http.Header{"Location": []string{"http://127.0.0.1:1/internal"}}, ""), nil
		}
		return response(r, http.StatusOK, http.Header{"X-Skill-Digest-Sha256": []string{digest}}, string(packageData)), nil
	})
	if err := client.http.CheckRedirect(&http.Request{}, nil); err != http.ErrUseLastResponse {
		t.Fatalf("redirect policy must stop at the original response, got %v", err)
	}
	if _, err := client.FetchSkill(context.Background(), "demo", "review", "1.2.3"); err == nil {
		t.Fatal("digest-mismatched package must not be accepted")
	}
	redirect = true
	if _, err := client.FetchSkill(context.Background(), "demo", "review", "1.2.3"); err == nil {
		t.Fatal("redirected package request must not be accepted")
	}
}

func TestFetchSkillReturnsNotFound(t *testing.T) {
	client := NewClient([]Upstream{{URL: "https://registry.example.com"}}, 1024)
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(r, http.StatusNotFound, nil, ""), nil
	})
	if _, err := client.FetchSkill(context.Background(), "demo", "missing", "1.0.0"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFetchSkillRejectsMismatchedCoordinatesBeforeDownload(t *testing.T) {
	metadataBytes, _ := json.Marshal(map[string]interface{}{
		"namespace": "other", "name": "review", "version": "1.0.0",
		"digest_sha256": strings.Repeat("a", 64), "package_type": "tgz", "size_bytes": 10,
	})
	client := NewClient([]Upstream{{URL: "https://registry.example.com"}}, 1024)
	requests := 0
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		return response(r, http.StatusOK, nil, string(metadataBytes)), nil
	})
	if _, err := client.FetchSkill(context.Background(), "demo", "review", "1.0.0"); err == nil {
		t.Fatal("upstream metadata with mismatched coordinates must be rejected")
	}
	if requests != 1 {
		t.Fatalf("mismatched metadata should be rejected before downloading, requests=%d", requests)
	}
}

func TestFetchArtifactVerifiesMetadataAndPackageDigest(t *testing.T) {
	data := []byte("artifact bytes")
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	metadataBytes, _ := json.Marshal(map[string]interface{}{
		"kind": "agent", "namespace": "demo", "name": "reviewer", "version": "1.0.0",
		"digest_sha256": digest, "package_type": "tgz", "size_bytes": len(data),
	})
	client := NewClient([]Upstream{{URL: "https://registry.example.com"}}, 1024)
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v1/artifacts/agent/demo/reviewer/versions/stable":
			return response(r, http.StatusOK, nil, string(metadataBytes)), nil
		case "/api/v1/artifacts/agent/demo/reviewer/versions/1.0.0/download":
			return response(r, http.StatusOK, http.Header{"X-Artifact-Digest-Sha256": []string{digest}}, string(data)), nil
		default:
			return response(r, http.StatusNotFound, nil, ""), nil
		}
	})
	pkg, err := client.FetchArtifact(context.Background(), "agent", "demo", "reviewer", "stable")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Version.Version != "1.0.0" || pkg.Version.DigestSHA256 != digest || string(pkg.Data) != string(data) {
		t.Fatalf("unexpected fetched artifact: %#v", pkg)
	}
}
