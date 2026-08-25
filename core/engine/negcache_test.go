package engine

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// TestCompileFailureNegativeCache verifies that a failed compilation is
// served from the negative cache while fresh, but retried once the
// entry goes stale instead of being remembered forever.
func TestCompileFailureNegativeCache(t *testing.T) {
	gj := &graphjinEngine{
		conf:      &Config{DBType: "postgres"},
		defaultDB: "main",
	}

	s := gstate{gj: gj, r: GraphqlReq{name: "broken"}}

	failures := 0
	s.compileFn = func() error {
		failures++
		return errors.New("boom")
	}

	if err := s.compileQueryOnce(); err == nil {
		t.Fatal("expected first call to fail")
	}
	if err := s.compileQueryOnce(); err == nil {
		t.Fatal("expected cached failure on second call")
	}
	if failures != 1 {
		t.Fatalf("compilation ran %d times within TTL, want 1", failures)
	}

	// Age the failure past the negative-cache window.
	val, _ := gj.queries.Load(s.key())
	cs := val.(*cstate)
	cs.failedAt = time.Now().Add(-2 * negCacheTTL)

	if err := s.compileQueryOnce(); err == nil {
		t.Fatal("expected retry to fail again (stub always fails)")
	}
	if failures != 2 {
		t.Fatalf("compilation ran %d times after expiry, want 2", failures)
	}
}

// TestConcurrentCompileSingleFlight ensures exactly one compilation runs
// when many requests with the same key arrive at once.
func TestConcurrentCompileSingleFlight(t *testing.T) {
	gj := &graphjinEngine{
		conf:      &Config{DBType: "postgres"},
		defaultDB: "main",
	}

	var runs int32
	var mu sync.Mutex

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := gstate{gj: gj, r: GraphqlReq{name: "hot"}}
			s.compileFn = func() error {
				mu.Lock()
				runs++
				mu.Unlock()
				<-start // hold the winner until everyone has arrived
				return nil
			}
			if err := s.compileQueryOnce(); err != nil {
				t.Error(err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if runs != 1 {
		t.Fatalf("compile ran %d times under concurrency, want 1", runs)
	}
}
