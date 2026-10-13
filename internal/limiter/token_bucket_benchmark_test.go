package limiter_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/MohsenDowlati/traffic-sign/internal/limiter"
)

func benchmarkLimiter(b *testing.B, shardCount int) {
	const numKeys = 10_000
	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = fmt.Sprintf("user-%d", i)
	}

	clock := limiter.RealClock{}
	tb := limiter.NewTokenBucketWithShards(100, 50.0, clock, shardCount)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			key := keys[r.Intn(numKeys)]
			_, _ = tb.Allow(key, 1)
		}
	})
}

func BenchmarkTokenBucket_SingleLock(b *testing.B) {
	benchmarkLimiter(b, 1)
}

func BenchmarkTokenBucket_Sharded64(b *testing.B) {
	benchmarkLimiter(b, 64)
}

func BenchmarkTokenBucket_Sharded256(b *testing.B) {
	benchmarkLimiter(b, 256)
}
