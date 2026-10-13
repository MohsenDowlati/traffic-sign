package limiter

import (
	"errors"
	"hash/fnv"
	"math"
	"sync"
	"time"
)

var _ Limiter = (*TokenBucket)(nil)

const defaultShardCount = 64

type bucketState struct {
	tokens     float64
	lastRefill time.Time
}

type shard struct {
	mu      sync.Mutex
	buckets map[string]*bucketState
}

type TokenBucket struct {
	capacity   float64
	ratePerSec float64
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

func (tb *TokenBucket) Allow(key string, cost int64) (LimiterResult, error) {
	if cost < 0 {
		return LimiterResult{}, errors.New("cost cannot be negative")
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
		}
		sh.buckets[key] = b
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
	return NewTokenBucketWithShards(capacity, ratePerSec, timer, defaultShardCount)
}

func NewTokenBucketWithShards(capacity float64, ratePerSec float64, timer Clock, shardCount int) *TokenBucket {
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
		timer:      timer,
		shardMask:  uint32(powerOfTwo - 1),
		shards:     shards,
	}
}
