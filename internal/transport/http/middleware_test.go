package http_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	limiterHttp "github.com/MohsenDowlati/traffic-sign/internal/http"
	"github.com/MohsenDowlati/traffic-sign/internal/limiter"
)

func TestRateLimitMiddleware(t *testing.T) {
	keyFunc := func(r *http.Request) string {
		return r.Header.Get("X-API-Key")
	}

	t.Run("when allowed: calls handler, returns 200, sets rate limit headers", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		tb := limiter.NewTokenBucket(10, 1.0, clock)

		handlerCalled := false
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		})

		mw := limiterHttp.RateLimitMiddleware(tb, keyFunc)(nextHandler)

		req := httptest.NewRequest(http.MethodGet, "/resource", nil)
		req.Header.Set("X-API-Key", "client-123")
		rec := httptest.NewRecorder()

		mw.ServeHTTP(rec, req)

		if !handlerCalled {
			t.Fatal("expected downstream handler to be called")
		}

		// 2. بررسی وضعیت HTTP
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		// 3. بررسی هدرها
		headers := rec.Header()
		if limit := headers.Get("X-RateLimit-Limit"); limit != "10" {
			t.Errorf("expected X-RateLimit-Limit to be 10, got %s", limit)
		}
		if remaining := headers.Get("X-RateLimit-Remaining"); remaining != "9" {
			t.Errorf("expected X-RateLimit-Remaining to be 9, got %s", remaining)
		}
		if reset := headers.Get("X-RateLimit-Reset"); reset == "" {
			t.Error("expected X-RateLimit-Reset to be present")
		}
		if retryAfter := headers.Get("Retry-After"); retryAfter != "" {
			t.Errorf("expected Retry-After to be empty when allowed, got %s", retryAfter)
		}
	})

	t.Run("when blocked: does not call handler, returns 429, sets Retry-After", func(t *testing.T) {
		clock := &limiter.FakeClock{T: time.Now()}
		// باکت با ظرفیت 1 توکن
		tb := limiter.NewTokenBucket(1, 1.0, clock)

		handlerCallCount := 0
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCallCount++
			w.WriteHeader(http.StatusOK)
		})

		mw := limiterHttp.RateLimitMiddleware(tb, keyFunc)(nextHandler)

		key := "client-456"

		req1 := httptest.NewRequest(http.MethodGet, "/resource", nil)
		req1.Header.Set("X-API-Key", key)
		rec1 := httptest.NewRecorder()
		mw.ServeHTTP(rec1, req1)

		if rec1.Code != http.StatusOK {
			t.Fatalf("first request should pass, got %d", rec1.Code)
		}

		req2 := httptest.NewRequest(http.MethodGet, "/resource", nil)
		req2.Header.Set("X-API-Key", key)
		rec2 := httptest.NewRecorder()
		mw.ServeHTTP(rec2, req2)

		if handlerCallCount != 1 {
			t.Fatalf("expected handler to be called only once, but was called %d times", handlerCallCount)
		}

		// 2. کد وضعیت 429
		if rec2.Code != http.StatusTooManyRequests {
			t.Fatalf("expected status 429 Too Many Requests, got %d", rec2.Code)
		}

		// 3. بررسی هدرها
		headers := rec2.Header()
		if remaining := headers.Get("X-RateLimit-Remaining"); remaining != "0" {
			t.Errorf("expected X-RateLimit-Remaining to be 0, got %s", remaining)
		}

		retryAfterStr := headers.Get("Retry-After")
		if retryAfterStr == "" {
			t.Fatal("expected Retry-After header to be present")
		}

		retrySeconds, err := strconv.Atoi(retryAfterStr)
		if err != nil || retrySeconds <= 0 {
			t.Errorf("expected Retry-After to be a positive integer, got %s", retryAfterStr)
		}

		if reset := headers.Get("X-RateLimit-Reset"); reset == "" {
			t.Error("expected X-RateLimit-Reset to be present")
		}
	})
}
