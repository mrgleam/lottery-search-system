// Command loadgen drives the API and reports a latency distribution.
//
//	go run ./cmd/loadgen -c 200 -d 30s -pattern '****23'
//
// It reports percentiles rather than an average, because an average hides the
// tail and the tail is what users actually complain about.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type result struct {
	latency time.Duration
	got     int
	status  int
	err     bool
}

func main() {
	var (
		url         = flag.String("url", "http://localhost:8080/search", "search endpoint")
		concurrency = flag.Int("c", 50, "concurrent workers")
		duration    = flag.Duration("d", 30*time.Second, "how long to run")
		pattern     = flag.String("pattern", "****23", "search pattern, or 'mixed' for a realistic spread")
		count       = flag.Int("count", 5, "tickets per request")
		warmup      = flag.Duration("warmup", 3*time.Second, "discard results from this opening period")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()

	var (
		mu      sync.Mutex
		results []result
		sent    atomic.Int64
	)
	started := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			// One client per worker, each with its own connection pool, so
			// workers do not queue behind a shared transport limit.
			client := &http.Client{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					MaxIdleConnsPerHost: 2,
					DisableCompression:  true,
				},
			}
			rng := rand.New(rand.NewSource(int64(w)))
			holder := fmt.Sprintf("load-%d", w)

			for ctx.Err() == nil {
				p := *pattern
				if p == "mixed" {
					p = mixedPattern(rng)
				}
				r := fire(ctx, client, *url, p, *count, holder)
				sent.Add(1)
				if time.Since(started) < *warmup {
					continue // discard warmup
				}
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()

	report(results, time.Since(started)-*warmup, *concurrency, sent.Load())
}

// mixedPattern produces a spread of pattern widths, because a benchmark that
// only fires one shape will not find the case that hurts.
func mixedPattern(rng *rand.Rand) string {
	shapes := []string{"******", "1*****", "12****", "***456", "****23", "1*3*5*", "123456"}
	s := []byte(shapes[rng.Intn(len(shapes))])
	for i := range s {
		if s[i] != '*' {
			s[i] = byte('0' + rng.Intn(10))
		}
	}
	return string(s)
}

func fire(ctx context.Context, c *http.Client, url, pattern string, count int, holder string) result {
	body, _ := json.Marshal(map[string]any{
		"pattern": pattern, "count": count, "holder": holder,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return result{err: true}
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.Do(req)
	if err != nil {
		return result{latency: time.Since(start), err: true}
	}
	defer resp.Body.Close()

	var parsed struct {
		Tickets []struct{} `json:"tickets"`
	}
	raw, _ := io.ReadAll(resp.Body)
	json.Unmarshal(raw, &parsed)

	return result{
		latency: time.Since(start),
		got:     len(parsed.Tickets),
		status:  resp.StatusCode,
	}
}

func report(rs []result, elapsed time.Duration, concurrency int, sent int64) {
	if len(rs) == 0 {
		fmt.Println("no results collected")
		return
	}
	lat := make([]time.Duration, 0, len(rs))
	var errs, empty, tickets int
	codes := map[int]int{}
	for _, r := range rs {
		if r.err {
			errs++
			continue
		}
		lat = append(lat, r.latency)
		codes[r.status]++
		tickets += r.got
		if r.got == 0 {
			empty++
		}
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })

	fmt.Printf("\nworkers          %d\n", concurrency)
	fmt.Printf("duration         %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("requests         %d measured (%d sent including warmup)\n", len(rs), sent)
	fmt.Printf("throughput       %.0f req/sec\n", float64(len(rs))/elapsed.Seconds())
	fmt.Printf("tickets issued   %d\n", tickets)
	fmt.Printf("empty responses  %d (%.1f%%)\n", empty, 100*float64(empty)/float64(len(rs)))
	fmt.Printf("errors           %d\n", errs)
	fmt.Printf("status codes     %v\n\n", codes)

	fmt.Printf("latency p50      %s\n", pct(lat, 0.50))
	fmt.Printf("latency p90      %s\n", pct(lat, 0.90))
	fmt.Printf("latency p99      %s\n", pct(lat, 0.99))
	fmt.Printf("latency max      %s\n", lat[len(lat)-1].Round(time.Microsecond))
}

func pct(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(float64(len(sorted)) * p)
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i].Round(time.Microsecond)
}
