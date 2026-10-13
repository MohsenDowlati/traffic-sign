package limiter_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/MohsenDowlati/traffic-sign/internal/limiter"
)

func BenchmarkTokenBucket(b *testing.B) {
	sizes := []int{1, 16, 64, 128}

	const numKeys = 5000
	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = fmt.Sprintf("tenant-user-%d", i)
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("shards-%d", size), func(b *testing.B) {
			tb := limiter.NewTokenBucketWithShards(1_000_000, 1_000_000, nil, size)

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				r := rand.New(rand.NewSource(time.Now().UnixNano()))
				for pb.Next() {
					key := keys[r.Intn(numKeys)]
					_, _ = tb.Allow(key, 1)
				}
			})
		})
	}
}
