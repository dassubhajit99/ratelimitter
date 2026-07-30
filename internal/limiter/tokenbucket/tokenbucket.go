package tokenbucket

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	corelimiter "github.com/dassubhajit99/ratelimitter/internal/limiter"
)

// bucket contains the rate-limit state for one key.
//
// A key could be:
//   - user:alice
//   - user:bob
//   - ip:127.0.0.1

type bucket struct {
	tokens    float64
	lastRefil time.Time
}

// TokenBucket is an in-memory token-bucket rate limiter.
type TokenBucket struct {
	// mu protects the buckets map and every bucket stored inside it.
	mu sync.Mutex

	// buckets stores one bucket for each user or IP address.
	buckets map[string]*bucket

	// capacity is the maximum number of tokens a bucket can hold.
	capacity float64

	// refillPerSecond is the number of tokens added each second.
	refillPerSecond float64

	// now returns the current time.
	//
	// Production uses time.Now. Tests can provide a fake clock.

	now func() time.Time
}

// Compile-time check that *TokenBucket implements limiter.Limiter.
// The idiom var _ SomeInterface = (*ConcreteType)(nil) doesn't create a variable you'll use (_ discards it) — it just forces the compiler to verify
//
//	*TokenBucket implements SomeInterface. If you later change TokenBucket's methods and break the interface, this line fails to compile immediately instead of
//	the bug surfacing at runtime.
var _ corelimiter.Limiter = (*TokenBucket)(nil)

// New creates a token-bucket limiter using the real system clock.
func New(capacity float64, refillPerSecond float64) (*TokenBucket, error) {
	return NewWithClock(capacity, refillPerSecond, time.Now)
}

// NewWithClock creates a token-bucket limiter with a custom clock.
//
// This is mainly useful for deterministic unit tests.
func NewWithClock(capacity float64, refillPerSecond float64, now func() time.Time) (*TokenBucket, error) {
	if capacity <= 0 {
		return nil, errors.New("capacity must be greater than zero")
	}

	if refillPerSecond <= 0 {
		return nil, errors.New(
			"refill rate must be greater than zero",
		)
	}

	if now == nil {
		return nil, errors.New("clock function cannot be nil")
	}

	return &TokenBucket{

		buckets:         make(map[string]*bucket),
		capacity:        capacity,
		refillPerSecond: refillPerSecond,
		now:             now,
	}, nil
}

// Allow checks whether one request should be allowed for the given key.
func (l *TokenBucket) Allow(_ context.Context, key string) (corelimiter.Result, error) {
	const cost = 1.0
	l.mu.Lock()

	defer l.mu.Unlock()

	now := l.now()

	currentBucket, exists := l.buckets[key]

	if !exists {
		// A new bucket starts full so a new user can immediately use
		// the configured burst capacity.
		currentBucket = &bucket{
			tokens:    l.capacity,
			lastRefil: now,
		}
		l.buckets[key] = currentBucket
	}

	// Calculate how much time has passed since this bucket was last used.
	elapsedSeconds := now.Sub(
		currentBucket.lastRefil,
	).Seconds()

	// Lazy refill:
	// new tokens = old tokens + elapsed time × refill rate

	refilledTokens := currentBucket.tokens + elapsedSeconds*l.refillPerSecond

	// A bucket must never contain more than its capacity.
	currentBucket.tokens = math.Min(refilledTokens, l.capacity)

	currentBucket.lastRefil = now

	if currentBucket.tokens >= cost {
		currentBucket.tokens -= cost

		return corelimiter.Result{
			Allowed: true,
			// The middleware currently exposes remaining tokens as an
			// integer, so fractional tokens are rounded down.
			Remaining:  int64(math.Floor(currentBucket.tokens)),
			RetryAfter: 0,
		}, nil
	}

	// The request was denied. Calculate how many tokens are missing.
	missingTokens := cost - currentBucket.tokens

	// Formula:
	//
	// wait time = missing tokens / tokens added per second
	retryAfterSeconds := missingTokens / l.refillPerSecond

	// Convert seconds into a time.Duration without returning a duration
	retryAfter := time.Duration(
		math.Ceil(
			retryAfterSeconds * float64(time.Second),
		),
	)

	return corelimiter.Result{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: retryAfter,
	}, nil

}
