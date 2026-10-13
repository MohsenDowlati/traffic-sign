package http

import (
	"math"
	"net/http"
	"strconv"

	"github.com/MohsenDowlati/traffic-sign/internal/limiter"
)

type Middleware func(http.Handler) http.Handler

func RateLimitMiddleware(l limiter.Limiter, keyFn KeyFunc) Middleware {
	if keyFn == nil {
		keyFn = KeyByIP()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if key == "" {
				key = KeyByIP()(r)
			}

			res, err := l.Allow(key, 1)
			if err != nil {
				http.Error(w, "internal rate limiter error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(res.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(res.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(res.ResetAT.Unix(), 10))

			if !res.Allowed {
				retrySeconds := int(math.Ceil(res.RetryAfter.Seconds()))
				if retrySeconds < 1 {
					retrySeconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))

				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte("429 Too Many Requests\n"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
