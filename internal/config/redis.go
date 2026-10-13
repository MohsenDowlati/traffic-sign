package config

import (
	"os"
	"strconv"
	"time"
)

type RedisConfig struct {
	Host         string
	Port         string
	Password     string
	DB           int
	PoolSize     int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type LimitConfig struct {
	RateLimitCapacity float64
	RateLimitRate     float64
	RateLimitTTL      time.Duration
	KeyStrategy       string
	KeyHeaderName     string
}

func LoadLimitConfig() LimitConfig {
	return LimitConfig{
		RateLimitCapacity: getEnvAsFloat("RATE_LIMIT_CAPACITY", 10.0),
		RateLimitRate:     getEnvAsFloat("RATE_LIMIT_RATE", 2.0),
		RateLimitTTL:      getEnvAsDuration("RATE_LIMIT_TTL", 10*time.Minute),
		KeyStrategy:       getEnv("RATE_LIMIT_KEY_BY", "api_key"),
		KeyHeaderName:     getEnv("RATE_LIMIT_HEADER_NAME", "X-API-Key"),
	}
}

func LoadRedisConfig() RedisConfig {
	return RedisConfig{
		Host:         getEnv("REDIS_HOST", "127.0.0.1"),
		Port:         getEnv("REDIS_PORT", "6379"),
		Password:     os.Getenv("REDIS_PASSWORD"),
		DB:           getEnvAsInt("REDIS_DB", 0),
		PoolSize:     getEnvAsInt("REDIS_POOL_SIZE", 10),
		DialTimeout:  getEnvAsDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
		ReadTimeout:  getEnvAsDuration("REDIS_READ_TIMEOUT", 3*time.Second),
		WriteTimeout: getEnvAsDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvAsFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
			return floatVal
		}
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}
