package limiter

import (
	"context"
	"time"
)

// Result describes the decision made by a rate limiter.
type Result struct {
	// Allowed tells the middleware whether the request may continue.
	Allowed bool

	// Remaining is the number of requests or tokens remaining.
	//
	// A negative value means that the limiter does not provide this
	// information.
	Remaining int64

	// RetryAfter tells the client how long it should wait before retrying.
	//
	// This is used when Allowed is false.
	RetryAfter time.Duration
}

// Limiter defines the behavior required from every rate-limiting algorithm.
//
// Later implementations may include: (will try each one)
//   - in-memory token bucket
//   - Redis token bucket
//   - sliding window

type Limiter interface {
	Allow(ctx context.Context, key string) (Result, error)
}
