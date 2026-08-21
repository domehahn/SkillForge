package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestMemoryBackendAllowsUpToRate(t *testing.T) {
	b := NewMemoryBackend(3, time.Minute)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		ok, err := b.Allow(ctx, "k")
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if !ok {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}
	ok, err := b.Allow(ctx, "k")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if ok {
		t.Fatal("expected the 4th request within the window to be rejected")
	}
}

func TestMemoryBackendResetsAfterWindow(t *testing.T) {
	b := NewMemoryBackend(1, 50*time.Millisecond)
	ctx := context.Background()
	ok, _ := b.Allow(ctx, "k")
	if !ok {
		t.Fatal("expected the first request to be allowed")
	}
	ok, _ = b.Allow(ctx, "k")
	if ok {
		t.Fatal("expected the second request in the same window to be rejected")
	}
	time.Sleep(60 * time.Millisecond)
	ok, _ = b.Allow(ctx, "k")
	if !ok {
		t.Fatal("expected a request after the window elapsed to be allowed again")
	}
}

func TestMemoryBackendKeysAreIndependent(t *testing.T) {
	b := NewMemoryBackend(1, time.Minute)
	ctx := context.Background()
	ok, _ := b.Allow(ctx, "a")
	if !ok {
		t.Fatal("expected key \"a\" to be allowed")
	}
	ok, _ = b.Allow(ctx, "b")
	if !ok {
		t.Fatal("expected key \"b\" to be independently allowed")
	}
}
