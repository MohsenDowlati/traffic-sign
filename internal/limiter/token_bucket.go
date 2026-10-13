package limiter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"hash/fnv"
	"math"
	"sync"
	"time"
)

var _ Limiter = (*TokenBucket)(nil)

const (
	defaultShardCount = 64
	keyLength         = 8
	defaultTTL        = 10 * time.Minute
)

type bucketState struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

type shard struct {
	mu      sync.Mutex
	buckets map[string]*bucketState
}

type TokenBucket struct {
	capacity   float64
	ratePerSec float64
	ttl        time.Duration
	timer      Clock
	shardMask  uint32
	shards     []*shard
}

func fnv32(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}

func (tb *TokenBucket) getShard(key string) *shard {
	idx := fnv32(key) & tb.shardMask
	return tb.shards[idx]
}

func (tb *TokenBucket) Allow(key string, cost int64) (Result, error) {
	if cost < 0 {
		return Result{}, errors.New("cost cannot be negative")
	}

	sh := tb.getShard(key)

	sh.mu.Lock()
	defer sh.mu.Unlock()

	now := tb.timer.Now()

	b, exists := sh.buckets[key]
	if !exists {
		b = &bucketState{
			tokens:     tb.capacity,
			lastRefill: now,
			lastSeen:   now,
		}
		sh.buckets[key] = b
	} else {
		if tb.ttl > 0 && now.Sub(b.lastSeen) > tb.ttl {
			b.tokens = tb.capacity
			b.lastRefill = now
		} else {
			tb.refill(b, now)
		}
		b.lastSeen = now
	}

	costF := float64(cost)

	if b.tokens >= costF {
		b.tokens -= costF

		tokensNeededForFull := tb.capacity - b.tokens
		resetDuration := time.Duration((tokensNeededForFull / tb.ratePerSec) * float64(time.Second))

		return Result{
			Allowed:    true,
			Remaining:  int64(math.Floor(b.tokens)),
			RetryAfter: 0,
			ResetAT:    now.Add(resetDuration),
			Limit:      int64(tb.capacity),
		}, nil
	}

	missing := costF - b.tokens
	retrySeconds := missing / tb.ratePerSec
	retryAfter := time.Duration(math.Ceil(retrySeconds * float64(time.Second)))

	tokensNeededForFull := tb.capacity - b.tokens
	resetDuration := time.Duration((tokensNeededForFull / tb.ratePerSec) * float64(time.Second))

	return Result{
		Allowed:    false,
		Remaining:  int64(math.Floor(b.tokens)),
		RetryAfter: retryAfter,
		ResetAT:    now.Add(resetDuration),
		Limit:      int64(tb.capacity),
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

func (tb *TokenBucket) Cleanup(now time.Time) int {
	if tb.ttl <= 0 {
		return 0
	}

	removedTotal := 0
	for _, sh := range tb.shards {
		sh.mu.Lock()
		for k, b := range sh.buckets {
			if now.Sub(b.lastSeen) > tb.ttl {
				delete(sh.buckets, k)
				removedTotal++
			}
		}
		sh.mu.Unlock()
	}
	return removedTotal
}

func (tb *TokenBucket) TotalKeys() int {
	total := 0
	for _, sh := range tb.shards {
		sh.mu.Lock()
		total += len(sh.buckets)
		sh.mu.Unlock()
	}
	return total
}

func (tb *TokenBucket) StartBackgroundCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				tb.Cleanup(t)
			}
		}
	}()
}

func NewTokenBucket(capacity float64, ratePerSec float64, timer Clock) *TokenBucket {
	return NewTokenBucketWithConfig(capacity, ratePerSec, timer, defaultShardCount, defaultTTL)
}

func NewTokenBucketWithShards(capacity float64, ratePerSec float64, timer Clock, shardCount int) *TokenBucket {
	return NewTokenBucketWithConfig(capacity, ratePerSec, timer, shardCount, defaultTTL)
}

func NewTokenBucketWithConfig(capacity float64, ratePerSec float64, timer Clock, shardCount int, ttl time.Duration) *TokenBucket {
	if timer == nil {
		timer = RealClock{}
	}

	powerOfTwo := 1
	for powerOfTwo < shardCount {
		powerOfTwo <<= 1
	}

	shards := make([]*shard, powerOfTwo)
	for i := range shards {
		shards[i] = &shard{
			buckets: make(map[string]*bucketState),
		}
	}

	return &TokenBucket{
		capacity:   capacity,
		ratePerSec: ratePerSec,
		ttl:        ttl,
		timer:      timer,
		shardMask:  uint32(powerOfTwo - 1),
		shards:     shards,
	}
}

func GenerateRandomKey() (string, error) {
	bytes := make([]byte, keyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "session_" + hex.EncodeToString(bytes), nil
}
