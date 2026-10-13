package limiter

import "time"

type Result struct {
	Allowed    bool
	Remaining  int64
	RetryAfter time.Duration // 0 if allowed
	ResetAT    time.Time
	Limit      int64
}

type Limiter interface {
	Allow(key string, cost int64) (Result, error)
}
