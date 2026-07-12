package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/dassubhajit99/ratelimitter.git/internal/limiter"
)

// RateLimit creates HTTP middleware that checks every request using the
// supplied limiter.

func RateLimit(l limiter.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := extractKey(r)

			log.Printf(
				"rate-limit check: method=%s path=%s key=%s", r.Method, r.URL.Path, key,
			)

			result, err := l.Allow(r.Context(), key)

			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "rate limiter unreachable",
				})
				return
			}

			if result.Remaining >= 0 {
				w.Header().Set(
					"X-RateLimit-Remaining",
					strconv.FormatInt(result.Remaining, 10),
				)
			}

			if !result.Allowed {
				retryAfterSeconds := durationToRetryAfter(result.RetryAfter)

				w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
				writeJSON(w, http.StatusTooManyRequests,
					map[string]any{
						"error":               "rate limit exceeded",
						"retry_after_seconds": retryAfterSeconds,
					},
				)
				return
			}
			next.ServeHTTP(w, r)

		})
	}
}

// extractKey chooses the identity used by the rate limiter.
//
// Priority:
//  1. X-User-ID header
//  2. Client IP address
func extractKey(r *http.Request) string {
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return "user:" + userID
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)

	if err == nil && host != "" {
		return "ip:" + host
	}

	return fmt.Sprintf("ip:%s", r.RemoteAddr)
}

// durationToRetryAfter converts a duration into whole seconds.
//
// Retry-After should not be zero when a request has been denied, so this
// function returns at least one second.
func durationToRetryAfter(duration time.Duration) int64 {
	if duration <= 0 {
		return 1
	}

	seconds := int64(duration / time.Second)

	if duration%time.Second != 0 {
		seconds++
	}
	return seconds
}

// writeJSON writes a JSON response with the provided HTTP status code.

func writeJSON(w http.ResponseWriter, statusCode int, body any) {

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("Failed to encode JSON response : %v", err)
	}
}
