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
