package limiter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

var _ Limiter = (*RedisTokenBucket)(nil)

const tokenBucketLuaScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

local data = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(data[1])
local last_refill = tonumber(data[2])

if not tokens or not last_refill then
    tokens = capacity
    last_refill = now
else
    local elapsed = (now - last_refill) / 1000.0
    if elapsed > 0 then
        local delta = elapsed * rate
        tokens = math.min(capacity, tokens + delta)
        last_refill = now
    end
end

local allowed = 0
local retry_after_ms = 0

if tokens >= cost then
    allowed = 1
    tokens = tokens - cost
else
    local missing = cost - tokens
    retry_after_ms = math.ceil((missing / rate) * 1000)
end

local needed_for_full = capacity - tokens
local reset_after_ms = math.ceil((needed_for_full / rate) * 1000)

redis.call("HMSET", key, "tokens", tostring(tokens), "last_refill", tostring(last_refill))
redis.call("EXPIRE", key, ttl)

return {allowed, math.floor(tokens), retry_after_ms, reset_after_ms}
`

type RedisTokenBucket struct {
	client     *redis.Client
	capacity   float64
	ratePerSec float64
	ttl        time.Duration
	keyPrefix  string
	script     *redis.Script
	timer      Clock
}

func NewRedisTokenBucket(
	client *redis.Client,
	capacity float64,
	ratePerSec float64,
	ttl time.Duration,
	timer Clock,
) *RedisTokenBucket {
	if timer == nil {
		timer = RealClock{}
	}
	if ttl <= 0 {
		fullFillSec := math.Ceil(capacity / ratePerSec)
		ttl = time.Duration(fullFillSec*2) * time.Second
	}

	return &RedisTokenBucket{
		client:     client,
		capacity:   capacity,
		ratePerSec: ratePerSec,
		ttl:        ttl,
		keyPrefix:  "rl:",
		script:     redis.NewScript(tokenBucketLuaScript),
		timer:      timer,
	}
}

func (r *RedisTokenBucket) Allow(key string, cost int64) (Result, error) {
	if cost < 0 {
		return Result{}, errors.New("cost cannot be negative")
	}

	redisKey := r.keyPrefix + key
	now := r.timer.Now()
	nowMs := now.UnixNano() / int64(time.Millisecond)
	ttlSec := int64(math.Ceil(r.ttl.Seconds()))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := r.script.Run(ctx, r.client, []string{redisKey},
		r.capacity,
		r.ratePerSec,
		nowMs,
		cost,
		ttlSec,
	).Result()

	if err != nil {
		return Result{}, fmt.Errorf("redis script execution failed: %w", err)
	}

	return parseLuaResult(res, now, int64(r.capacity))
}

func parseLuaResult(raw interface{}, now time.Time, capacityLimit int64) (Result, error) {
	slice, ok := raw.([]interface{})
	if !ok || len(slice) < 4 {
		return Result{}, fmt.Errorf("unexpected script response format: %v", raw)
	}

	allowedInt, ok1 := slice[0].(int64)
	remainingInt, ok2 := slice[1].(int64)
	retryAfterMs, ok3 := slice[2].(int64)
	resetAfterMs, ok4 := slice[3].(int64)

	if !ok1 || !ok2 || !ok3 || !ok4 {
		return Result{}, fmt.Errorf("failed to cast script response types: %v", slice)
	}

	return Result{
		Allowed:    allowedInt == 1,
		Remaining:  remainingInt,
		RetryAfter: time.Duration(retryAfterMs) * time.Millisecond,
		ResetAT:    now.Add(time.Duration(resetAfterMs) * time.Millisecond),
		Limit:      capacityLimit,
	}, nil
}
