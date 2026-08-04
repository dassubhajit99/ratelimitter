- Token bucket algo result

```text
./bin/loadgen   -rate=50   -duration=10s   -url=http://localhost:8080/api/ping   -user=alice   -concurrency=100
Starting load test
URL:         http://localhost:8080/api/ping
User:        alice
Rate:        50 requests/second
Duration:    10s
Concurrency: 100


Per-second timeline
-------------------
sec 0   allowed=11   denied=38   other=0    errors=0
sec 1   allowed=2    denied=48   other=0    errors=0
sec 2   allowed=2    denied=48   other=0    errors=0
sec 3   allowed=2    denied=48   other=0    errors=0
sec 4   allowed=2    denied=48   other=0    errors=0
sec 5   allowed=2    denied=48   other=0    errors=0
sec 6   allowed=2    denied=48   other=0    errors=0
sec 7   allowed=2    denied=48   other=0    errors=0
sec 8   allowed=2    denied=48   other=0    errors=0
sec 9   allowed=2    denied=48   other=0    errors=0
sec 10  allowed=0    denied=1    other=0    errors=0

Summary
-------
Total requests: 500
Actual time:    10s
HTTP 200:       29
HTTP 429:       471
Errors:         0
Average latency: 574µs
```

```

capacity = 10
refill   = 2 tokens/second

For a test of:

rate     = 50 requests/second
duration = 10 seconds

Approximately 500 requests will be sent.

The theoretical allowance is approximately:

initial capacity + refill during the test
10 + (2 × 10)
= 30 requests

Therefore, expect approximately:

HTTP 200 = 29–30
HTTP 429 = 470–471

The exact boundary can vary because:

The first request starts after the first ticker interval.
The last request may occur just before the ten-second timer fires.
Tokens refill fractionally between requests.
OS and HTTP scheduling are not perfectly exact.

A typical timeline may look like:

sec 0   allowed=11 denied=38
sec 1   allowed=2  denied=48
sec 2   allowed=2  denied=48
sec 3   allowed=2  denied=48
sec 4   allowed=2  denied=48
sec 5   allowed=2  denied=48
sec 6   allowed=2  denied=48
sec 7   allowed=2  denied=48
sec 8   allowed=2  denied=48
sec 9   allowed=2  denied=48

The first second can show around 11 or 12 allowed rather than exactly 10 because refill begins immediately after the first bucket is created.
```

```
What happens under the load (50 req/s against a single user's bucket, capacity=10, refill=2/s):

  1. Second 0 — the burst drains the initial capacity: The bucket starts with 10 tokens. The loadgen fires 50 requests spaced ~20ms apart (interval = 1s/50),
  so it takes close to a full second to dispatch all of them. While that's happening, the bucket is continuously refilling at 2 tokens/sec. By the time the
  11th request lands, roughly ~0.5s has elapsed since the burst started, which is enough for 0.5 × 2 = 1 extra token — so you get 11 allowed, not exactly 10.
  The remaining 38 of the 49 requests sent in that window (49 arrivals not counting the very first tick landing in sec 0's window) are denied.
  2. Seconds 1–9 — steady state: Once the bucket is drained, it can never accumulate more than what refills between requests. Since requests arrive far faster
  (50/s) than refill (2/s), each second only ever has ~2 tokens available → allowed=2, denied=48, second after second. This is the bucket's steady-state
  throughput exactly equaling refillPerSecond.
  3. Second 10 — trailing request: the 10s timer cuts off mid-flight; one straggler request fires just as the test stops, finds the bucket empty (not enough
  elapsed time for a full token), and gets denied.

```
