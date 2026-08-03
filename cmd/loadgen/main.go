package loadgen

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
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

	go func() {
		collectorDone <- collectResults(results)
	}()

	fmt.Printf("Starting load test\n")
	fmt.Printf("URL:         %s\n", *targetURL)
	fmt.Printf("User:        %s\n", *userID)
	fmt.Printf("Rate:        %d requests/second\n", *rate)
	fmt.Printf("Duration:    %s\n", *duration)
	fmt.Printf("Concurrency: %d\n\n", *concurrency)

	testStartedAt := time.Now()

	runLoadTest(
		client,
		*targetURL,
		*userID,
		*rate,
		*duration,
		testStartedAt,
		semaphore,
		results,
		&requestWG,
	)

	// Stop creating requests first, then wait for every request that
	// is already running.
	requestWG.Wait()

	// No more results will be sent after the WaitGroup reaches zero.
	close(results)

	testSummary := <-collectorDone

	printSummary(testSummary, time.Since(testStartedAt))
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

func fireRequest(client *http.Client, targetURL string, userID string, startedAt time.Duration) result {
	request, err := http.NewRequest(http.MethodGet, targetURL, nil)

	if err != nil {
		return result{
			startedAt: startedAt,
			err:       err,
		}
	}

	request.Header.Set("X-User-ID", userID)

	requestStartedAt := time.Now()

	response, err := client.Do(request)

	latency := time.Since(requestStartedAt)

	if err != nil {
		return result{
			latency:   latency,
			startedAt: startedAt,
			err:       err,
		}
	}

	defer response.Body.Close()

	// Reading the body completely allows Go's HTTP client to reuse
	// the underlying TCP connection.

	_, copyErr := io.Copy(io.Discard, response.Body)
	if copyErr != nil {
		return result{
			latency:    latency,
			statusCode: response.StatusCode,
			startedAt:  startedAt,
			err:        copyErr,
		}
	}

	return result{
		statusCode: response.StatusCode,
		startedAt:  startedAt,
		latency:    latency,
	}

}

func runLoadTest(
	client *http.Client,
	targetURL string,
	userID string,
	rate int,
	duration time.Duration,
	testStartedAt time.Time,
	semaphore chan struct{},
	results chan<- result,
	wg *sync.WaitGroup,

) {
	interval := time.Second / time.Duration(rate)

	ticker := time.NewTicker(interval)

	defer ticker.Stop()

	stopTimer := time.NewTimer(duration)

	defer stopTimer.Stop()

	for {
		select {
		case scheduledAt := <-ticker.C:

			// Wait until an in-flight request slot is available.
			//
			// This prevents the program from creating an unlimited
			// number of goroutines.
			/*
				How this Counting Semaphore works:
				-semaphore is a buffered channel (e.g., make(chan struct{}, 50)).

				-Sending a zero-byte empty struct (struct{}{}) into semaphore consumes 1 available slot.

				-If the channel buffer is not full, the send completes instantly, and execution proceeds.

				-If the buffer is 100% full (meaning 50 requests are currently in-flight), semaphore <- struct{}{} BLOCKS. The main generator pauses, waiting for an active request to finish and free up a slot.
			*/

			semaphore <- struct{}{}

			wg.Add(1)

			go func(requestStartedAt time.Time) {
				defer wg.Done()
				/*

					<-semaphore (The Release): When fireRequest() completes, <-semaphore reads an item out of the buffered channel. This frees up 1 buffer slot, which instantly unblocks the main loop if it was waiting on semaphore <- struct{}{} to launch the next tick!

				*/
				defer func() {
					<-semaphore
				}()

				requestResult := fireRequest(client, targetURL, userID, requestStartedAt.Sub(testStartedAt))

				results <- requestResult

			}(scheduledAt)

		case <-stopTimer.C:
			return
		}
	}
}

func collectResults(results <-chan result) summery {
	collected := summery{
		statusCounts: make(map[int]int),
		timeline:     make(map[int]*secondStats),
	}

	for requestResult := range results {
		collected.totalRequest++
		collected.totalLatency += requestResult.latency

		// A request starting at 1.7 seconds belongs to second 1.
		second := int(requestResult.startedAt.Seconds())

		stats, exits := collected.timeline[second]

		if !exits {
			stats = &secondStats{}
			collected.timeline[second] = stats
		}

		if requestResult.err != nil {
			collected.errorCount++
			stats.errors++

			fmt.Fprintf(
				os.Stderr,
				"request error: %v\n",
				requestResult.err,
			)
			continue
		}

		collected.statusCounts[requestResult.statusCode]++

		switch {
		case requestResult.statusCode >= 200 &&
			requestResult.statusCode < 300:
			stats.allowed++

		case requestResult.statusCode == http.StatusTooManyRequests:
			stats.denied++

		default:
			stats.other++
		}
	}
	return collected
}

func printSummary(
	testSummary summery,
	actualDuration time.Duration,
) {
	fmt.Println("\nPer-second timeline")
	fmt.Println("-------------------")

	seconds := make([]int, 0, len(testSummary.timeline))

	for second := range testSummary.timeline {
		seconds = append(seconds, second)
	}

	sort.Ints(seconds)

	for _, second := range seconds {
		stats := testSummary.timeline[second]

		fmt.Printf(
			"sec %-3d allowed=%-4d denied=%-4d other=%-4d errors=%-4d\n",
			second,
			stats.allowed,
			stats.denied,
			stats.other,
			stats.errors,
		)
	}

	fmt.Println("\nSummary")
	fmt.Println("-------")

	fmt.Printf("Total requests: %d\n", testSummary.totalRequest)
	fmt.Printf("Actual time:    %s\n", actualDuration.Round(time.Millisecond))

	statusCodes := make([]int, 0, len(testSummary.statusCounts))

	for statusCode := range testSummary.statusCounts {
		statusCodes = append(statusCodes, statusCode)
	}

	sort.Ints(statusCodes)

	for _, statusCode := range statusCodes {
		fmt.Printf(
			"HTTP %d:       %d\n",
			statusCode,
			testSummary.statusCounts[statusCode],
		)
	}

	fmt.Printf("Errors:         %d\n", testSummary.errorCount)

	if testSummary.totalRequest > 0 {
		averageLatency := testSummary.totalLatency /
			time.Duration(testSummary.totalRequest)

		fmt.Printf(
			"Average latency: %s\n",
			averageLatency.Round(time.Microsecond),
		)
	}
}
