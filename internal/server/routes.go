package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/dassubhajit99/ratelimitter/internal/limiter"
	"github.com/dassubhajit99/ratelimitter/internal/middleware"
)

// NewRouter creates the application's HTTP routes and applies middleware.
func NewRouter(l limiter.Limiter) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/ping", pingHandler)
	mux.HandleFunc("GET /api/work", workHandler)

	return middleware.RateLimit(l)(mux)
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK,
		map[string]string{
			"message": "pong",
		},
	)
}

func workHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate an expensive operation such as a database query,
	// external API request, or CPU-heavy operation.
	time.Sleep(50 * time.Millisecond) // 50 * 10^6 =  50ms
	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"message": "work completed",
		},
	)
}

func writeJSON(
	w http.ResponseWriter,
	statusCode int,
	body any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("failed to encode JSON response: %v", err)
	}
}
