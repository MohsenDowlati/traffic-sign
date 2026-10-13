//go:build integration

package limiter_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MohsenDowlati/traffic-sign/internal/limiter"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupRedisContainer(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}

	endpoint, err := redisContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to get redis endpoint: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: endpoint,
	})

	cleanup := func() {
		_ = rdb.Close()
		_ = redisContainer.Terminate(ctx)
	}

	return rdb, cleanup
}

func TestRedisTokenBucket_ConcurrencyAndCapacity(t *testing.T) {
	rdb, teardown := setupRedisContainer(t)
	defer teardown()

	capacity := 10.0
	refillRate := 2.0
	tb := limiter.NewRedisTokenBucket(rdb, capacity, refillRate, 5*time.Second, nil)

	key := "user-concurrent-test"
	totalRequests := 50
	var wg sync.WaitGroup
	var allowedCount int64

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := tb.Allow(key, 1)
			if err == nil && res.Allowed {
				atomic.AddInt64(&allowedCount, 1)
			}
		}()
	}
	wg.Wait()

	if allowedCount != int64(capacity) {
		t.Errorf("expected exactly %d allowed requests under concurrency, got %d", int64(capacity), allowedCount)
	}
}

func TestRedisTokenBucket_TTLExpiration(t *testing.T) {
	rdb, teardown := setupRedisContainer(t)
	defer teardown()

	ttl := 2 * time.Second
	tb := limiter.NewRedisTokenBucket(rdb, 5, 1, ttl, nil)

	key := "user-expiring"

	res, err := tb.Allow(key, 1)
	if err != nil || !res.Allowed {
		t.Fatalf("first request should succeed: %v", err)
	}

	ctx := context.Background()
	redisKey := fmt.Sprintf("rl:%s", key)

	exists, err := rdb.Exists(ctx, redisKey).Result()
	if err != nil || exists == 0 {
		t.Fatal("key should exist in redis")
	}

	time.Sleep(ttl + 500*time.Millisecond)

	existsAfter, err := rdb.Exists(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("redis query failed: %v", err)
	}
	if existsAfter != 0 {
		t.Errorf("expected key to self-expire after TTL, but it still exists")
	}
}
