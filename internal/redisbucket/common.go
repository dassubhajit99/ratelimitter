package redisbucket

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config contains settings shared by both Redis implementations.
type Config struct {
	Client          *redis.Client
	Capacity        float64
	RefillPerSecond float64

	// TTL removes buckets that have been idle for a long time.
	TTL time.Duration

	// Prefix separates rate-limit keys from other Redis data.
	Prefix string

	// Now is injected so time can be controlled in tests.
	Now func() time.Time

	// NaiveRaceDelay exists only to make the race in the naive
	// implementation easier to reproduce.
	//
	// The Lua implementation ignores this value.

	NaiveRaceDelay time.Duration
}

// DefaultTTL returns a TTL longer than the time needed for an empty
// bucket to become full.
//
// Empty-to-full time:
//
//	capacity / refill rate
//
// We use twice that duration, with a minimum of one minute.

func DefaultTTL(capacity float64, refillPerSecond float64) time.Duration {
	refillDurationSeconds := capacity / refillPerSecond
	ttlSeconds := math.Ceil(refillDurationSeconds)

	return time.Duration(math.Min(ttlSeconds, 60) * float64(time.Second))
}

func normalizeConfig(config Config) (Config, error) {
	if config.Client == nil {
		return Config{}, errors.New("Redis client cannot be nil")
	}

	if config.Capacity <= 0 {
		return Config{}, errors.New(
			"capacity must be greater than zero",
		)
	}

	if config.RefillPerSecond <= 0 {
		return Config{}, errors.New(
			"refill rate must be greater than zero",
		)
	}

	if config.Prefix == "" {
		config.Prefix = "ratelimit:"
	}

	if config.Now == nil {
		config.Now = time.Now
	}

	if config.TTL <= 0 {
		config.TTL = DefaultTTL(
			config.Capacity,
			config.RefillPerSecond,
		)
	}

	return config, nil
}

func (config Config) redisKey(key string) string {
	return config.Prefix + key
}

// unixSeconds returns Unix time with fractional seconds.
// the current Unix timestamp as a floating-point number (float64)
// representing seconds with nanosecond precision (e.g., 1785958073.1920166).
// UnixNano is converted to seconds so a refill rate such as two tokens
// per second can still refill fractions between requests.

func unixSeconds(currentTime time.Time) float64 {
	return float64(currentTime.UnixNano()) / (float64(time.Second))
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// optionalFloat parses a value returned by Redis.
//
// found is false when the Redis hash field did not exist.
