package tokenbucket

import (
	"context"
	"testing"
	"time"
)

// fakeClock lets a test control the current time.
type fakeClock struct {
	current time.Time
}

// Now returns the fake current time.
func (c *fakeClock) Now() time.Time {
	return c.current
}

// Advance moves the fake clock forward.
func (c *fakeClock) Advance(duration time.Duration) {
	c.current = c.current.Add(duration)
}

func TestFullBucketAllowsCapacityThenDenies(t *testing.T) {
	clock := &fakeClock{
		current: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	rateLimiter, err := NewWithClock(3, 1, clock.Now)

	if err != nil {
		t.Fatalf("failed to create limiter %v", err)
	}

	ctx := context.Background()

	// Capacity is three, so exactly three immediate requests
	// should be allowed.

	for requestNumber := 1; requestNumber <= 3; requestNumber += 1 {
		result, err := rateLimiter.Allow(ctx, "subhajit")

		if err != nil {
			t.Fatalf(
				"request %d returned error: %v",
				requestNumber,
				err,
			)
		}

		if !result.Allowed {
			t.Fatalf(
				"request %d should have been allowed",
				requestNumber,
			)
		}

	}

	// No time has passed, so no tokens have been refilled.
	result, err := rateLimiter.Allow(ctx, "subhajit")

	if err != nil {
		t.Fatalf("fourth request returned error: %v", err)
	}

	if result.Allowed {
		t.Fatal("fourth request should have been denied")
	}
}

func TestBucketRefillsOverTime(t *testing.T) {
	clock := &fakeClock{
		current: time.Date(2026,
			time.January,
			1,
			0,
			0,
			0,
			0,
			time.UTC),
	}

	// Capacity: two tokens
	// Refill: two tokens per second
	rateLimiter, err := NewWithClock(2, 2, clock.Now)
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}
	ctx := context.Background()

	// Drain both initial tokens.
	for requestNumber := 1; requestNumber <= 2; requestNumber++ {
		result, err := rateLimiter.Allow(ctx, "subhajit")

		if err != nil {
			t.Fatalf(
				"request %d returned error: %v",
				requestNumber,
				err,
			)
		}

		if !result.Allowed {
			t.Fatalf(
				"request %d should have been allowed",
				requestNumber,
			)
		}
	}

	// Bucket is now empty.
	result, err := rateLimiter.Allow(ctx, "subhajit")

	if err != nil {
		t.Fatalf(
			"request returned error: %v",

			err,
		)
	}

	if result.Allowed {
		t.Fatal("request should have been denied while bucket was empty")
	}

	// Two tokens per second means 500 ms produces one token.

	clock.Advance(500 * time.Millisecond)

	result, err = rateLimiter.Allow(ctx, "subhajit")
	if err != nil {
		t.Fatalf("request after refill returned error: %v", err)
	}

	if !result.Allowed {
		t.Fatal("request should have been allowed after one token refilled")
	}

	// We consumed the newly refilled token, so the bucket is empty again.
	result, err = rateLimiter.Allow(ctx, "subhajit")
	if err != nil {
		t.Fatalf("second request after refill returned error: %v", err)
	}

	if result.Allowed {
		t.Fatal("second request after refill should have been denied")
	}
}
