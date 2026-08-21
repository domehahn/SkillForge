package ratelimit

// Exercises RedisBackend against a real disposable Redis container, not a
// mock — same approach used for S3Storage (internal/storage/s3_test.go).
// Skips itself if Docker isn't available.

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func startRedis(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available — skipping RedisBackend integration test")
	}

	port := 16379 + rand.Intn(2000)
	containerName := fmt.Sprintf("skillforge-redis-test-%d", port)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	cmd := exec.Command("docker", "run", "-d", "--rm",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:6379", port),
		"redis:7-alpine",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not start Redis container (likely no network access to pull the image): %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { client.Close() })

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Ping(context.Background()).Err(); err == nil {
			return addr
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("Redis container did not become ready in time")
	return ""
}

func newTestRedisBackend(t *testing.T, rate int, window time.Duration) *RedisBackend {
	t.Helper()
	addr := startRedis(t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { client.Close() })
	prefix := fmt.Sprintf("test-%d:", rand.Intn(1_000_000))
	return NewRedisBackend(client, rate, window, prefix)
}

func TestRedisBackendAllowsUpToRate(t *testing.T) {
	b := newTestRedisBackend(t, 3, time.Minute)
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

func TestRedisBackendResetsAfterWindow(t *testing.T) {
	b := newTestRedisBackend(t, 1, 500*time.Millisecond)
	ctx := context.Background()
	ok, _ := b.Allow(ctx, "k")
	if !ok {
		t.Fatal("expected the first request to be allowed")
	}
	ok, _ = b.Allow(ctx, "k")
	if ok {
		t.Fatal("expected the second request in the same window to be rejected")
	}
	time.Sleep(600 * time.Millisecond)
	ok, _ = b.Allow(ctx, "k")
	if !ok {
		t.Fatal("expected a request after the window elapsed to be allowed again")
	}
}

func TestRedisBackendKeysAreIndependent(t *testing.T) {
	b := newTestRedisBackend(t, 1, time.Minute)
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

// TestRedisBackendSharedAcrossClients is the actual point of this backend:
// two independent client connections (standing in for two registry
// replicas) must observe the same counter, unlike MemoryBackend where each
// instance's counters are entirely its own.
func TestRedisBackendSharedAcrossClients(t *testing.T) {
	addr := startRedis(t)
	prefix := fmt.Sprintf("test-%d:", rand.Intn(1_000_000))

	client1 := redis.NewClient(&redis.Options{Addr: addr})
	defer client1.Close()
	client2 := redis.NewClient(&redis.Options{Addr: addr})
	defer client2.Close()

	replicaA := NewRedisBackend(client1, 2, time.Minute, prefix)
	replicaB := NewRedisBackend(client2, 2, time.Minute, prefix)
	ctx := context.Background()

	ok, _ := replicaA.Allow(ctx, "shared-key")
	if !ok {
		t.Fatal("expected replica A's first request to be allowed")
	}
	ok, _ = replicaB.Allow(ctx, "shared-key")
	if !ok {
		t.Fatal("expected replica B's request to be allowed (2nd of the shared budget of 2)")
	}
	ok, _ = replicaA.Allow(ctx, "shared-key")
	if ok {
		t.Fatal("expected replica A's second request to be rejected — the budget was shared with replica B, not per-replica")
	}
}
