package scenarios

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

// RunLoad is the load tier: mixed read traffic with latency stats.
// Fails on any request error; latency numbers are informational.
func RunLoad(h *harness.H, total int) error {
	c := h.Budgets.Concurrency
	if c == 0 || c > total {
		c = total
	}
	work := make(chan int)
	go func() {
		for i := 0; i < total; i++ {
			work <- i
		}
		close(work)
	}()

	type result struct {
		ms float64
		ok bool
	}
	results := make(chan result, total)

	start := time.Now()
	var wg sync.WaitGroup
	for w := 0; w < c; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				t0 := time.Now()
				var ok bool
				if i%5 < 3 {
					status, body, _, err := h.Rest("GET", "getUser", urlVals("id", fmt.Sprint(1+i%3)), nil)
					ok = err == nil && status == 200 && firstError(body) == ""
				} else {
					resp, err := h.GQL(`{ users(limit: 2, orderBy: { id: desc }) { id full_name } }`, nil)
					ok = err == nil && resp["errors"] == nil && resp["data"] != nil
				}
				results <- result{ms: float64(time.Since(t0).Microseconds()) / 1000.0, ok: ok}
			}
		}()
	}
	wg.Wait()
	close(results)
	elapsed := time.Since(start)

	var lat []float64
	var fails int
	for r := range results {
		if !r.ok {
			fails++
		}
		lat = append(lat, r.ms)
	}
	sort.Float64s(lat)

	pct := func(p float64) float64 {
		idx := int(float64(len(lat)-1) * p)
		return lat[idx]
	}
	rps := float64(total) / elapsed.Seconds()

	fmt.Printf("    %-22s %6d reqs  %4d clients  %8.0f rps  p50=%6.1fms  p95=%6.1fms  max=%7.1fms\n",
		h.BaseURL[strings.LastIndex(h.BaseURL, "/")+1:], total, c, rps, pct(0.50), pct(0.95), lat[len(lat)-1])

	if fails != 0 {
		return fmt.Errorf("%d/%d requests failed", fails, total)
	}
	return nil
}
