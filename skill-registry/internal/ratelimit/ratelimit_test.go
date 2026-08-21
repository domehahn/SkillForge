package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeBackend struct {
	allow bool
	err   error
	calls []string
}

func (f *fakeBackend) Allow(_ context.Context, key string) (bool, error) {
	f.calls = append(f.calls, key)
	return f.allow, f.err
}

func TestLimiterPrefersAuthTokenOverIP(t *testing.T) {
	backend := &fakeBackend{allow: true}
	l := New(backend, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"
	req.Header.Set("Authorization", "Bearer abcdefghijklmno")

	l.Allow(req)
	if len(backend.calls) != 1 || backend.calls[0][:4] != "tok:" {
		t.Fatalf("expected a token-derived key, got %v", backend.calls)
	}
}

func TestLimiterFallsBackToClientIP(t *testing.T) {
	backend := &fakeBackend{allow: true}
	l := New(backend, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"

	l.Allow(req)
	if len(backend.calls) != 1 || backend.calls[0] != "ip:203.0.113.5" {
		t.Fatalf("expected an IP-derived key, got %v", backend.calls)
	}
}

func TestLimiterFailsOpenOnBackendError(t *testing.T) {
	backend := &fakeBackend{allow: false, err: errors.New("redis is down")}
	l := New(backend, nil, nil) // nil logger — must not panic

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"

	if !l.Allow(req) {
		t.Fatal("expected a backend error to fail open (allow the request), not block traffic entirely")
	}
}

func TestMiddlewareBlocksWhenNotAllowed(t *testing.T) {
	backend := &fakeBackend{allow: false}
	l := New(backend, nil, nil)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	handler := Middleware(l)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("expected the next handler not to be called when rate-limited")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}

func TestMiddlewarePassesThroughWhenAllowed(t *testing.T) {
	backend := &fakeBackend{allow: true}
	l := New(backend, nil, nil)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	handler := Middleware(l)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected the next handler to be called when allowed")
	}
}
