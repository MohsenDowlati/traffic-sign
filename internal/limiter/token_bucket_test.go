package limiter_test

import (
	"testing"
	"time"

	"github.com/MohsenDowlati/traffic-sign/internal/limiter"
)

func TestTokenBucketLimiter(t *testing.T) {
	t.Run("basic allow: 10 requests allowed, 11th blocked", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		tb := limiter.NewTokenBucket(10, 1.0, clock)

		key := "user-1"
		for i := 1; i <= 10; i++ {
			res, _ := tb.Allow(key, 1)
			if !res.Allowed {
				t.Fatalf("request #%d should be allowed, got blocked", i)
			}
		}

		res, _ := tb.Allow(key, 1)
		if res.Allowed {
			t.Fatal("11th request should be blocked")
		}
		if res.RetryAfter <= 0 {
			t.Errorf("expected RetryAfter > 0, got %v", res.RetryAfter)
		}
	})

	t.Run("refill: tokens replenish over time", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		tb := limiter.NewTokenBucket(10, 10.0, clock)

		key := "user-refill"
		for i := 0; i < 10; i++ {
			_, err := tb.Allow(key, 1)
			if err != nil {
				t.Fatal(err)
			}
		}

		if res, _ := tb.Allow(key, 1); res.Allowed {
			t.Fatal("expected request to be rejected when bucket is empty")
		}

		clock.Advance(100 * time.Millisecond)

		res, _ := tb.Allow(key, 1)
		if !res.Allowed {
			t.Fatalf("expected request to be allowed after 100ms refill")
		}

		if res, _ := tb.Allow(key, 1); res.Allowed {
			t.Fatal("expected subsequent request to be blocked")
		}
	})

	t.Run("burst capacity: allows bursts higher than refill rate", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		tb := limiter.NewTokenBucket(20, 5.0, clock)

		key := "user-burst"
		for i := 1; i <= 20; i++ {
			res, _ := tb.Allow(key, 1)
			if !res.Allowed {
				t.Fatalf("burst request #%d failed, expected capacity to handle it", i)
			}
		}

		if res, _ := tb.Allow(key, 1); res.Allowed {
			t.Fatal("request beyond burst capacity should be rejected")
		}
	})

	t.Run("cost > 1: cost=5 should consume 5 tokens", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		tb := limiter.NewTokenBucket(10, 1.0, clock)

		key := "user-cost"

		res, _ := tb.Allow(key, 5)
		if !res.Allowed {
			t.Fatalf("expected 5-token request to be allowed")
		}

		res, _ = tb.Allow(key, 5)
		if !res.Allowed {
			t.Fatalf("expected second 5-token request to be allowed")
		}

		res, _ = tb.Allow(key, 1)
		if res.Allowed {
			t.Fatal("expected bucket to be empty after consuming 10 tokens")
		}
	})

	t.Run("key isolation: key A does not affect key B", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		tb := limiter.NewTokenBucket(2, 1.0, clock)

		keyA := "tenant-A"
		keyB := "tenant-B"

		tb.Allow(keyA, 1)
		tb.Allow(keyA, 1)
		if res, _ := tb.Allow(keyA, 1); res.Allowed {
			t.Fatal("key A should be rate limited")
		}

		resB1, _ := tb.Allow(keyB, 1)
		if !resB1.Allowed {
			t.Fatalf("key B should not be affected by key A")
		}

		resB2, _ := tb.Allow(keyB, 1)
		if !resB2.Allowed {
			t.Fatalf("key B should have its full capacity available")
		}
	})
}

func TestTTLAndCleanup(t *testing.T) {
	t.Run("cleanup removes expired keys and decreases map size", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		ttl := 5 * time.Minute
		tb := limiter.NewTokenBucketWithConfig(10, 1.0, clock, 16, ttl)

		key1, _ := limiter.GenerateRandomKey()
		key2, _ := limiter.GenerateRandomKey()

		tb.Allow(key1, 1)
		tb.Allow(key2, 1)

		if count := tb.TotalKeys(); count != 2 {
			t.Fatalf("expected 2 keys in map, got %d", count)
		}

		clock.Advance(2 * time.Minute)
		removed := tb.Cleanup(clock.Now())
		if removed != 0 || tb.TotalKeys() != 2 {
			t.Fatalf("no keys should be cleaned up yet, removed: %d, remaining: %d", removed, tb.TotalKeys())
		}

		tb.Allow(key1, 1)

		clock.Advance(4 * time.Minute)
		removed = tb.Cleanup(clock.Now())

		if removed != 1 {
			t.Fatalf("expected exactly 1 key removed, got %d", removed)
		}
		if count := tb.TotalKeys(); count != 1 {
			t.Fatalf("expected 1 remaining key, got %d", count)
		}

		clock.Advance(2 * time.Minute)
		removed = tb.Cleanup(clock.Now())
		if removed != 1 || tb.TotalKeys() != 0 {
			t.Fatalf("expected key1 to be cleaned up, remaining: %d", tb.TotalKeys())
		}
	})

	t.Run("expired key resets state on lazy access if not cleaned yet", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		ttl := 2 * time.Minute
		tb := limiter.NewTokenBucketWithConfig(5, 0.1, clock, 8, ttl)

		key := "user-idle"

		for i := 0; i < 5; i++ {
			tb.Allow(key, 1)
		}

		res, _ := tb.Allow(key, 1)
		if res.Allowed {
			t.Fatal("expected request to be blocked")
		}

		clock.Advance(3 * time.Minute)

		res, _ = tb.Allow(key, 1)
		if !res.Allowed {
			t.Fatal("expected idle key to reset and be allowed")
		}
	})
}
