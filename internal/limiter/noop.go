package limiter

import "context"

// NoopLimiter is a temporary limiter that allows every request.
//
// It lets us test the HTTP and middleware plumbing before implementing
// a real rate-limiting algorithm.
type NoopLimiter struct{}

// Allow always permits the request.
func (NoopLimiter) Allow(ctx context.Context, key string) (Result, error) {
	return Result{
		Allowed:    true,
		Remaining:  -1,
		RetryAfter: 0,
	}, nil
}
