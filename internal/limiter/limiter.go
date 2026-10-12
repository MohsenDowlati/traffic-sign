package limiter

import "time"

type LimiterResult struct {
	Allowed bool
	Remaining int64
	RetryAfter time.Duration // 0 if allowed
	ResetAT time.Time
	limit int64
}

type Limiter interface {
	Allow(key string, cost int64) (LimiterResult, error)
}