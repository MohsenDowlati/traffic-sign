package limiter

import (
	"errors"
	"math"
	"sync"
	"time"
)

var _ Limiter = (*TokenBucket)(nil)

type bucketState struct {
	tokens     float64
	lastRefill time.Time
}

type TokenBucket struct {
	mu         sync.Mutex
	capacity   float64
	ratePerSec float64
	timer      Clock
	buckets    map[string]*bucketState
}

func (tb *TokenBucket) Allow(key string, cost int64) (LimiterResult, error) {
	if cost < 0 {
		return LimiterResult{}, errors.New("cost cannot be negative")
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := tb.timer.Now()

	b, exists := tb.buckets[key]
	if !exists {
		b = &bucketState{
			tokens:     tb.capacity,
			lastRefill: now,
		}
		tb.buckets[key] = b
	} else {
		tb.refill(b, now)
	}

	costF := float64(cost)

	if b.tokens >= costF {
		b.tokens -= costF

		tokensNeededForFull := tb.capacity - b.tokens
		resetDuration := time.Duration((tokensNeededForFull / tb.ratePerSec) * float64(time.Second))

		return LimiterResult{
			Allowed:    true,
			Remaining:  int64(math.Floor(b.tokens)),
			RetryAfter: 0,
			ResetAT:    now.Add(resetDuration),
			limit:      int64(tb.capacity),
		}, nil
	}

	missing := costF - b.tokens
	retrySeconds := missing / tb.ratePerSec
	retryAfter := time.Duration(math.Ceil(retrySeconds * float64(time.Second)))

	tokensNeededForFull := tb.capacity - b.tokens
	resetDuration := time.Duration((tokensNeededForFull / tb.ratePerSec) * float64(time.Second))

	return LimiterResult{
		Allowed:    false,
		Remaining:  int64(math.Floor(b.tokens)),
		RetryAfter: retryAfter,
		ResetAT:    now.Add(resetDuration),
		limit:      int64(tb.capacity),
	}, nil
}

func (tb *TokenBucket) refill(b *bucketState, now time.Time) {
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}

	newTokens := elapsed * tb.ratePerSec
	b.tokens = math.Min(tb.capacity, b.tokens+newTokens)
	b.lastRefill = now
}

func NewTokenBucket(capacity float64, ratePerSec float64, timer Clock) *TokenBucket {
	if timer == nil {
		timer = RealClock{}
	}

	return &TokenBucket{
		capacity:   capacity,
		ratePerSec: ratePerSec,
		timer:      timer,
		buckets:    make(map[string]*bucketState),
	}
}
