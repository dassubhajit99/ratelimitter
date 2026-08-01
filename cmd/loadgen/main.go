package loadgen

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// result represents the outcome of one HTTP request.
type result struct {
	statusCode int
	latency    time.Duration
	startedAt  time.Duration
	err        error
}

// secondStats stores results for one second of the test.
type secondStats struct {
	allowed int
	denied  int
	other   int
	errors  int
}

// summary stores the final result of the load test.
//
// Only the collector goroutine modifies this value, so it does not
// require a mutex.
type summery struct {
	totalRequest int
	statusCounts map[int]int
	errorCount   int
	totalLatency time.Duration
	timeline     map[int]*secondStats
}

func main() {
	rate := flag.Int("rate", 50, "number of requests to start per second")

	duration := flag.Duration("duration", 10*time.Second, "duration of the load test")

	targetURL := flag.String(
		"url",
		"http://localhost:8080/api/ping",
		"URL to request",
	)

	userID := flag.String(
		"user",
		"subhajit",
		"value for the X-User-ID header",
	)

	concurrency := flag.Int(
		"concurrency",
		100,
		"maximum number of requests allowed in flight",
	)

	requestTimeout := flag.Duration(
		"timeout",
		5*time.Second,
		"timeout for each HTTP request",
	)

	flag.Parse()

	if err := validateFlags(*rate, *duration, *concurrency, *requestTimeout); err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		Timeout: *requestTimeout,
	}

	results := make(chan result, *concurrency)

	// A buffered channel works as a semaphore.
	//
	// Adding a value acquires a concurrency slot.
	// Removing a value releases that slot.
	semaphore := make(chan struct{}, *concurrency)

	var requestWG sync.WaitGroup

	collectorDone := make(chan summery, 1)

	// go func() {
	// 	collectorDone <- coll
	// }()
}

func validateFlags(rate int,
	duration time.Duration,
	concurrency int,
	requestTimeout time.Duration) error {
	if rate <= 0 {
		return fmt.Errorf("rate must be greater than zero")
	}

	if duration <= 0 {
		return fmt.Errorf("duration must be greater than zero")
	}

	if concurrency <= 0 {
		return fmt.Errorf("concurrency must be greater than zero")
	}

	if requestTimeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}

	interval := time.Second / time.Duration(rate)
	if interval <= 0 {
		return fmt.Errorf("rate is too high to represent with a ticker")
	}

	return nil
}
