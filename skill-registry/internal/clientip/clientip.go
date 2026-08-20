// Package clientip is the single place SkillForge resolves a request's
// client IP address for audit logging and rate limiting. Before this
// package existed, both internal/api and internal/ratelimit had their own
// copy of the same logic, and it unconditionally trusted the
// client-supplied X-Forwarded-For header — any client could set that
// header to an arbitrary value and have it recorded in the audit log or
// used as their rate-limit bucket key, trivially bypassing per-IP rate
// limiting (a fresh header value per request looks like a fresh client
// every time) and forging the IP attributed to their actions in the audit
// trail.
package clientip

import (
	"net"
	"net/http"
	"strings"
)

// Resolver resolves a request's client IP, honoring X-Forwarded-For only
// when the request's immediate peer (RemoteAddr) is a trusted proxy.
type Resolver struct {
	trusted []*net.IPNet
}

// NewResolver builds a Resolver from a list of CIDR strings (e.g.
// "10.0.0.0/8", "127.0.0.1/32"). Invalid entries are skipped — config
// validation (config.Config.Validate) is expected to have already rejected
// a malformed CIDR before this is called; skipping rather than panicking
// here just means "not trusted", the safe direction to fail in.
func NewResolver(trustedProxyCIDRs []string) *Resolver {
	r := &Resolver{}
	for _, cidr := range trustedProxyCIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		r.trusted = append(r.trusted, ipnet)
	}
	return r
}

// Resolve returns the client IP for r. If RemoteAddr is not a trusted
// proxy, X-Forwarded-For is ignored entirely and RemoteAddr's address is
// used — a client behind an untrusted or absent proxy cannot spoof its
// resolved IP by setting the header itself.
func (res *Resolver) Resolve(r *http.Request) string {
	peer := remoteAddrIP(r.RemoteAddr)
	if res.isTrusted(peer) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0]); first != "" {
				return first
			}
		}
	}
	if peer != "" {
		return peer
	}
	return r.RemoteAddr
}

func (res *Resolver) isTrusted(ip string) bool {
	if ip == "" || len(res.trusted) == 0 {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, ipnet := range res.trusted {
		if ipnet.Contains(parsed) {
			return true
		}
	}
	return false
}

func remoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// RemoteAddr without a port (e.g. in some test harnesses) — treat
		// the whole value as the address.
		return remoteAddr
	}
	return host
}
