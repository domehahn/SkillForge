package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func request(remoteAddr, xff string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	return req
}

func TestResolveWithNoTrustedProxiesIgnoresXFF(t *testing.T) {
	r := NewResolver(nil)
	got := r.Resolve(request("203.0.113.5:1234", "10.0.0.99"))
	if got != "203.0.113.5" {
		t.Fatalf("expected the peer address to be used when no proxies are trusted, got %q", got)
	}
}

func TestResolveIgnoresXFFFromUntrustedPeer(t *testing.T) {
	// A client directly hitting the server (no real reverse proxy in
	// front of it) can set X-Forwarded-For to anything it likes — that
	// must not be honored just because *some* trusted range is configured
	// elsewhere.
	r := NewResolver([]string{"10.0.0.0/8"})
	got := r.Resolve(request("203.0.113.5:1234", "198.51.100.1"))
	if got != "203.0.113.5" {
		t.Fatalf("expected the peer address (untrusted) to be used, got %q", got)
	}
}

func TestResolveHonorsXFFFromTrustedPeer(t *testing.T) {
	r := NewResolver([]string{"10.0.0.0/8"})
	got := r.Resolve(request("10.0.0.5:1234", "198.51.100.1"))
	if got != "198.51.100.1" {
		t.Fatalf("expected X-Forwarded-For to be honored from a trusted proxy, got %q", got)
	}
}

func TestResolveTakesFirstXFFEntry(t *testing.T) {
	r := NewResolver([]string{"10.0.0.0/8"})
	got := r.Resolve(request("10.0.0.5:1234", "198.51.100.1, 203.0.113.9, 10.0.0.5"))
	if got != "198.51.100.1" {
		t.Fatalf("expected the leftmost (original client) entry, got %q", got)
	}
}

func TestResolveFallsBackWhenXFFMissingFromTrustedPeer(t *testing.T) {
	r := NewResolver([]string{"10.0.0.0/8"})
	got := r.Resolve(request("10.0.0.5:1234", ""))
	if got != "10.0.0.5" {
		t.Fatalf("expected the peer address when a trusted proxy sends no X-Forwarded-For, got %q", got)
	}
}

func TestResolveIgnoresInvalidCIDR(t *testing.T) {
	// NewResolver must not panic on a malformed entry (config.Validate is
	// expected to reject it earlier) — it should just not trust it.
	r := NewResolver([]string{"not-a-cidr"})
	got := r.Resolve(request("203.0.113.5:1234", "198.51.100.1"))
	if got != "203.0.113.5" {
		t.Fatalf("expected an invalid CIDR entry to simply not be trusted, got %q", got)
	}
}

func TestResolveHandlesIPv6(t *testing.T) {
	r := NewResolver([]string{"::1/128"})
	got := r.Resolve(request("[::1]:1234", "198.51.100.1"))
	if got != "198.51.100.1" {
		t.Fatalf("expected X-Forwarded-For to be honored for a trusted IPv6 peer, got %q", got)
	}
}
