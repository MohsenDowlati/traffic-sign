package limiter_test

import (
	"github.com/MohsenDowlati/traffic-sign/internal/limiter"
	"testing"
	"time"
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

func TestCreateAndUseKey(t *testing.T) {
	t.Run("Create a Key and allow it once", func(t *testing.T) {

		clock := &limiter.FakeClock{T: time.Now()}
		tb := limiter.NewTokenBucket(10, 1.0, clock)

		ttl := 1 * time.Minute
		key, err := limiter.GenerateRandomKey(ttl)

		if err != nil {
			t.Fatalf("couldn't generate a random key : %v", err)
		}

		res, _ := tb.Allow(key, 1)

		if !res.Allowed {
			t.Fatalf("expected request to be allowed.")
		}
	})
}

func TestExpiredTTL(t *testing.T) {

	t.Run("Expiration of TTl", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		tb := limiter.NewTokenBucket(10, 1.0, clock)

		ttl := 1 * time.Minute
		key, err := limiter.GenerateRandomKey(ttl)
		if err != nil {
			t.Fatalf("couldn't generate a random key : %v", err)
		}

		res, _ := tb.Allow(key, 1)
		if !res.Allowed {
			t.Fatalf("expected request to be allowed.")
		}

		clock.Advance(1 * time.Minute)

		res, _ = tb.Allow(key, 1)

		if res.Allowed {
			t.Fatalf("The key should be expired.")
		}
	})
}
