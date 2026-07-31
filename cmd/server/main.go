package main

import (
	"log"
	"net/http"
	"time"

	"github.com/dassubhajit99/ratelimitter/internal/limiter/tokenbucket"
	"github.com/dassubhajit99/ratelimitter/internal/server"
)

func main() {
	// Allow a burst of 10 requests.
	// After the burst, refill 2 tokens every second.
	rateLimiter, err := tokenbucket.New(10, 2)

	if err != nil {
		log.Fatalf("failed to create rate limiter: %v", err)
	}

	router := server.NewRouter(rateLimiter)

	httpServer := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("server listening on http://localhost%s", httpServer.Addr)

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed :%v", err)
	}
}
