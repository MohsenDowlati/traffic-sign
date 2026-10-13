package main

import (
	"context"
	"errors"
	"github.com/MohsenDowlati/traffic-sign/internal/config"
	"github.com/MohsenDowlati/traffic-sign/internal/limiter"
	http2 "github.com/MohsenDowlati/traffic-sign/internal/transport/http"
	"log"
	"net/http"
	"os"
	"os/signal"

	"syscall"
	"time"
)

func main() {
	cfg := config.LoadRedisConfig()

	rdb, err := limiter.NewRedisClient(cfg)

	if err != nil {
		log.Fatalf("Failed to connect to Redis at %s:%s: %v", cfg.Host, cfg.Port, err)
	}
	log.Printf("Connected to Redis successfully (%s:%s)", cfg.Host, cfg.Port)

	limitConfig := config.LoadLimitConfig()

	redisLimiter := limiter.NewRedisTokenBucket(
		rdb,
		limitConfig.RateLimitCapacity,
		limitConfig.RateLimitRate,
		limitConfig.RateLimitTTL,
		limiter.RealClock{},
	)

	keyFunc := http2.KeyFuncFromConfig(limitConfig.KeyStrategy, limitConfig.KeyHeaderName)
	rateLimiterMw := http2.RateLimitMiddleware(redisLimiter, keyFunc)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "success", "status": "approved"}`))
	})

	handlerWithMiddleware := rateLimiterMw(mux)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handlerWithMiddleware,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Server listening on port %s (KeyStrategy: %s)...", cfg.Port, limitConfig.KeyStrategy)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Printf("Server forced shutdown: %v", err)
	}

	if err := rdb.Close(); err != nil {
		log.Printf("Error closing Redis connection: %v", err)
	}

	log.Println("Server gracefully stopped.")
}
