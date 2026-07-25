package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/dassubhajit99/ratelimitter/internal/limiter"
	"github.com/dassubhajit99/ratelimitter/internal/server"
)

type denyLimiter struct {
}

func (denyLimiter) Allow(ctx context.Context, key string) (limiter.Result, error) {
	return limiter.Result{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: 5 * time.Second,
	}, nil
}

func main() {
	// rateLimiter := limiter.NoopLimiter{}
	rateLimiter := denyLimiter{}

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
