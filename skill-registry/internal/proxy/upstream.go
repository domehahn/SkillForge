package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/skillforge/skill-registry/internal/metadata"
)

var ErrNotFound = errors.New("upstream package version not found")

type Upstream struct {
	URL         string
	BearerToken string
}

type SkillPackage struct {
	Version *metadata.SkillVersion
	Data    []byte
}

type ArtifactPackage struct {
	Version *metadata.ArtifactVersion
	Data    []byte
}

type SkillFetcher interface {
	FetchSkill(context.Context, string, string, string) (*SkillPackage, error)
}

type ArtifactFetcher interface {
	FetchArtifact(context.Context, string, string, string, string) (*ArtifactPackage, error)
}

type Client struct {
	upstreams []Upstream
	maxBytes  int64
	http      *http.Client
}

func NewClient(upstreams []Upstream, maxBytes int64) *Client {
	return &Client{
		upstreams: append([]Upstream(nil), upstreams...),
		maxBytes:  maxBytes,
		http: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) FetchSkill(ctx context.Context, namespace, name, version string) (*SkillPackage, error) {
	var lastErr error
	for _, upstream := range c.upstreams {
		pkg, err := c.fetchFrom(ctx, upstream, namespace, name, version)
		if err == nil {
			return pkg, nil
		}
		if errors.Is(err, ErrNotFound) {
			lastErr = err
			continue
		}
		return nil, err
	}
	if lastErr != nil {
		return nil, ErrNotFound
	}
	return nil, ErrNotFound
}

// FetchArtifact retrieves an artifact version from configured upstreams and verifies
// its coordinates, declared size, response digest, and content digest before returning it.
func (c *Client) FetchArtifact(ctx context.Context, kind, namespace, name, ref string) (*ArtifactPackage, error) {
	var lastErr error
	for _, upstream := range c.upstreams {
		pkg, err := c.fetchArtifactFrom(ctx, upstream, kind, namespace, name, ref)
		if err == nil {
			return pkg, nil
		}
		if errors.Is(err, ErrNotFound) {
			lastErr = err
			continue
		}
		return nil, err
	}
	if lastErr != nil {
		return nil, ErrNotFound
	}
	return nil, ErrNotFound
}

func (c *Client) fetchArtifactFrom(ctx context.Context, upstream Upstream, kind, namespace, name, ref string) (*ArtifactPackage, error) {
	base, err := url.Parse(upstream.URL)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return nil, fmt.Errorf("invalid configured upstream URL")
	}
	root := strings.TrimRight(upstream.URL, "/") + "/api/v1/artifacts/" + url.PathEscape(kind) + "/" + url.PathEscape(namespace) + "/" + url.PathEscape(name) + "/versions/"
	resp, err := c.get(ctx, upstream, root+url.PathEscape(ref))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("upstream artifact metadata request failed with status %d", resp.StatusCode)
	}
	metadataBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read upstream artifact metadata: %w", err)
	}
	if len(metadataBytes) > 1<<20 {
		return nil, fmt.Errorf("upstream artifact metadata exceeds 1 MiB")
	}
	var version metadata.ArtifactVersion
	if err := json.Unmarshal(metadataBytes, &version); err != nil {
		return nil, fmt.Errorf("decode upstream artifact metadata: %w", err)
	}
	if version.Kind != kind || version.Namespace != namespace || version.Name != name || version.Version == "" {
		return nil, fmt.Errorf("upstream artifact metadata coordinates do not match the request")
	}
	if version.PackageType != "tgz" && version.PackageType != "zip" {
		return nil, fmt.Errorf("upstream artifact package type %q is not supported", version.PackageType)
	}
	digest := strings.TrimPrefix(version.DigestSHA256, "sha256:")
	digestBytes, err := hex.DecodeString(digest)
	if err != nil || len(digestBytes) != sha256.Size {
		return nil, fmt.Errorf("upstream artifact metadata contains an invalid SHA-256 digest")
	}
	if version.SizeBytes <= 0 || version.SizeBytes > c.maxBytes {
		return nil, fmt.Errorf("upstream artifact size is outside the configured limit")
	}
	downloadURL := root + url.PathEscape(version.Version) + "/download"
	download, err := c.get(ctx, upstream, downloadURL)
	if err != nil {
		return nil, err
	}
	defer download.Body.Close()
	if download.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream artifact package request failed with status %d", download.StatusCode)
	}
	if strings.TrimPrefix(download.Header.Get("X-Artifact-Digest-SHA256"), "sha256:") != digest {
		return nil, fmt.Errorf("upstream artifact download digest header does not match metadata")
	}
	data, err := io.ReadAll(io.LimitReader(download.Body, c.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upstream artifact package: %w", err)
	}
	if int64(len(data)) > c.maxBytes || int64(len(data)) != version.SizeBytes {
		return nil, fmt.Errorf("upstream artifact size does not match metadata or configured limit")
	}
	actual := sha256.Sum256(data)
	if hex.EncodeToString(actual[:]) != digest {
		return nil, fmt.Errorf("upstream artifact digest verification failed")
	}
	return &ArtifactPackage{Version: &version, Data: data}, nil
}

func (c *Client) fetchFrom(ctx context.Context, upstream Upstream, namespace, name, version string) (*SkillPackage, error) {
	base, err := url.Parse(upstream.URL)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return nil, fmt.Errorf("invalid configured upstream URL")
	}
	coordinates := strings.Join([]string{url.PathEscape(namespace), url.PathEscape(name), "versions", url.PathEscape(version)}, "/")
	versionURL := strings.TrimRight(upstream.URL, "/") + "/api/v1/skills/" + coordinates
	resp, err := c.get(ctx, upstream, versionURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("upstream metadata request failed with status %d", resp.StatusCode)
	}
	metadataBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read upstream metadata: %w", err)
	}
	if len(metadataBytes) > 1<<20 {
		return nil, fmt.Errorf("upstream metadata exceeds 1 MiB")
	}
	var skillVersion metadata.SkillVersion
	if err := json.Unmarshal(metadataBytes, &skillVersion); err != nil {
		return nil, fmt.Errorf("decode upstream metadata: %w", err)
	}
	if skillVersion.Namespace != namespace || skillVersion.Name != name || skillVersion.Version == "" {
		return nil, fmt.Errorf("upstream metadata coordinates do not match the requested skill")
	}
	if skillVersion.PackageType != "tgz" && skillVersion.PackageType != "zip" {
		return nil, fmt.Errorf("upstream package type %q is not supported", skillVersion.PackageType)
	}
	digest := strings.TrimPrefix(skillVersion.DigestSHA256, "sha256:")
	digestBytes, err := hex.DecodeString(digest)
	if err != nil || len(digestBytes) != sha256.Size {
		return nil, fmt.Errorf("upstream metadata contains an invalid SHA-256 digest")
	}
	if skillVersion.SizeBytes <= 0 || skillVersion.SizeBytes > c.maxBytes {
		return nil, fmt.Errorf("upstream package size is outside the configured limit")
	}

	downloadURL := strings.TrimRight(upstream.URL, "/") + "/api/v1/skills/" + url.PathEscape(namespace) + "/" + url.PathEscape(name) + "/versions/" + url.PathEscape(skillVersion.Version) + "/download"
	download, err := c.get(ctx, upstream, downloadURL)
	if err != nil {
		return nil, err
	}
	defer download.Body.Close()
	if download.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream package request failed with status %d", download.StatusCode)
	}
	headerDigest := strings.TrimPrefix(download.Header.Get("X-Skill-Digest-SHA256"), "sha256:")
	if headerDigest != digest {
		return nil, fmt.Errorf("upstream download digest header does not match metadata")
	}
	data, err := io.ReadAll(io.LimitReader(download.Body, c.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upstream package: %w", err)
	}
	if int64(len(data)) > c.maxBytes || int64(len(data)) != skillVersion.SizeBytes {
		return nil, fmt.Errorf("upstream package size does not match metadata or configured limit")
	}
	actualDigest := sha256.Sum256(data)
	if hex.EncodeToString(actualDigest[:]) != digest {
		return nil, fmt.Errorf("upstream package digest verification failed")
	}
	return &SkillPackage{Version: &skillVersion, Data: data}, nil
}

func (c *Client) get(ctx context.Context, upstream Upstream, target string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	if upstream.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+upstream.BearerToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed")
	}
	return resp, nil
}
