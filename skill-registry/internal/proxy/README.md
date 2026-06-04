# Skill Registry Proxy

The Skill Registry includes an interface for upstream proxy support, allowing it to cache packages from remote registries.

## Interface

```go
type UpstreamRegistry interface {
    Resolve(ctx context.Context, namespace, name, version string) (*metadata.SkillVersion, error)
    Download(ctx context.Context, namespace, name, version string) (io.ReadCloser, *metadata.SkillVersion, error)
}
```

## Configuration

```yaml
proxy:
  enabled: true
  upstreams:
    - https://skills.example.com
    - https://public-skills.org
```

## Behavior

When a package is not found locally and proxy is enabled:

1. Registry checks each upstream in order
2. If found, downloads and caches locally
3. Marks the cached version with `source=upstream`
4. Preserves original digest
5. Subsequent requests use cached copy

## Implementation Status

The proxy interface and configuration are defined. Full implementation including:
- HTTP client for upstream registries
- Caching logic
- Cache invalidation
- Upstream authentication

Will be added in Phase 2.

## Security Considerations

When implementing proxy support:
- Validate upstream URLs
- Use TLS for upstream connections
- Verify digests from upstream
- Implement timeout and retry logic
- Cache upstream failures temporarily
- Support authentication for private upstreams
