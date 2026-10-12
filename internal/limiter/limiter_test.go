package limiter

import (
	"testing"
	"time"
)

func TestHasEnoughToken(t *testing.T) {
	t.Run("when bucket has enough tokens", func(t *testing.T) {
		clock := &FakeClock{t: time.Now()}
		limiter := NewTokenBucket(3, 1.0, clock)

		for i := 0; i < 3; i++ {
			res := limiter.Allow()
			if !res.Allowed {
				t.Fatalf("expected request %d to be allowed, but got disallowed", i+1)
			}
			if res.RetryAfter != 0 {
				t.Errorf("expected RetryAfter to be 0, got %v", res.RetryAfter)
			}
		}
	})
}

func TestNotEnoughToken(t *testing.T) {
	t.Run("when bucket has not enough tokens", func(t *testing.T) {
		clock := &FakeClock{t: time.Now()}
		limiter := NewTokenBucket(1, 2.0, clock)

		res := limiter.Allow() // اضافه شدن پرانتز
		if !res.Allowed {
			t.Fatal("first request should be allowed")
		}

		res = limiter.Allow()
		if res.Allowed {
			t.Error("expected Allowed=false when bucket is exhausted")
		}
		if res.RetryAfter <= 0 {
			t.Errorf("expected RetryAfter > 0, got %v", res.RetryAfter)
		}

		expectedRetry := 500 * time.Millisecond
		if res.RetryAfter != expectedRetry {
			t.Errorf("expected RetryAfter=%v, got %v", expectedRetry, res.RetryAfter)
		}
	})
}

func TestAfterTime(t *testing.T) {
	t.Run("after time passes: tokens replenish", func(t *testing.T) {
		clock := &FakeClock{t: time.Now()}
		limiter := NewTokenBucket(2, 1.0, clock)

		limiter.Allow()
		limiter.Allow()

		if res := limiter.Allow(); res.Allowed {
			t.Fatal("expected request to be rejected immediately after emptying bucket")
		}

		clock.Advance(1 * time.Second)

		res := limiter.Allow()
		if !res.Allowed {
			t.Errorf("expected request to be allowed after token replenishment")
		}
		if res.RetryAfter != 0 {
			t.Errorf("expected RetryAfter to be 0, got %v", res.RetryAfter)
		}

		res = limiter.Allow()
		if res.Allowed {
			t.Errorf("expected request to be rejected as only 1 token was replenished")
		}
	})
}
