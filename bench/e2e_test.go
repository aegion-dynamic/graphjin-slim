package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
	"github.com/aegion-dynamic/graphjin-slim/bench/v3/scenarios"
	postgresmod "github.com/aegion-dynamic/graphjin-slim/postgres/v3"
)

// backends lists every database engine the battery covers. The postgres
// tier runs only when its container (see bench/dockerfile) is reachable;
// locally absent docker degrades to a visible skip, never a failure.
var backends = []string{"sqlite", "postgres"}

func TestE2EScenarios(t *testing.T) {
	budgets := harness.DefaultBudgets()

	pgOK := backendAvailable("postgres")

	for _, backend := range backends {
		if backend == "postgres" && !pgOK {
			t.Run("postgres/unavailable", func(t *testing.T) {
				t.Skipf("postgres not reachable at %s — start it with: docker run --rm -p 5432:5432 $(docker build -q -f dockerfile .)", harness.PostgresDSN())
			})
			continue
		}

		for _, sc := range scenarios.All {
			if backend == "postgres" && sc.Schema != "" && sc.Schema != "shop" {
				continue // chain/blob fixtures are sqlite-specific today
			}
			for _, v := range sc.Variants {
				sc, v, backend := sc, v, backend
				t.Run(sc.Name+"/"+v+"/"+backend, func(t *testing.T) {
					t.Chdir(t.TempDir())
					h, err := harness.SpinUp(sc.Opts(v, backend, budgets))
					if err != nil {
						if errors.Is(err, harness.ErrBackendUnavailable) && backend == "postgres" {
							t.Skipf("postgres unavailable: %v", err)
						}
						t.Fatalf("spin-up: %v", err)
					}
					defer h.Stop()

					if err := harness.Guarded(h, budgets, func() error { return sc.Fn(h) }); err != nil {
						if errors.Is(err, harness.ErrKnownBug) {
							t.Skip(err.Error())
						}
						t.Fatal(err)
					}
				})
			}
		}
	}

	// Differential oracles over everything the battery recorded.
	var mismatches []string

	perBackend := map[string][]string{} // "sqlite","postgres" → dimension keys
	for key, dims := range harness.DiffLog {
		for dim := range dims {
			backend := "sqlite"
			if i := strings.Index(dim, "@"); i >= 0 {
				backend = dim[i+1:]
			}
			perBackend[backend] = append(perBackend[backend], key+"@"+dim)
		}
	}

	for backend, keys := range perBackend {
		sort.Strings(keys)
		for _, k := range keys {
			got := harness.DiffLog[strings.SplitN(k, "@", 2)[0]]
			dv, pv := got["dev@"+backend], got["prod@"+backend]
			if dv == "" || pv == "" {
				mismatches = append(mismatches, fmt.Sprintf("%s[%s]: missing a variant", k, backend))
				continue
			}
			if dv != pv {
				mismatches = append(mismatches,
					fmt.Sprintf("%s[%s]\n  dev : %s\n  prod: %s", k, backend, dv, pv))
			}
		}
	}

	// Cross-dialect oracle: when both engines ran, identical logical reads
	// must produce identical canonical results everywhere. Every key in
	// the differential battery is therefore required to be deterministic
	// (no timestamps, no server-side randomness).
	sd, pd := harness.DiffLog["user1"]["dev@sqlite"], harness.DiffLog["user1"]["dev@postgres"]
	allD := harness.DiffLog["allusers"]
	if sd != "" && pd != "" && sd != pd {
		mismatches = append(mismatches, "cross-dialect user1: sqlite and postgres disagree")
	}
	if allD["dev@sqlite"] != "" && allD["dev@postgres"] != "" && allD["dev@sqlite"] != allD["dev@postgres"] {
		mismatches = append(mismatches, "cross-dialect allusers: sqlite and postgres disagree")
	}
	sp := harness.DiffLog["products"]
	if sp["dev@sqlite"] != "" && sp["dev@postgres"] != "" && sp["dev@sqlite"] != sp["dev@postgres"] {
		mismatches = append(mismatches, "cross-dialect products: sqlite and postgres disagree")
	}

	if len(mismatches) > 0 {
		for _, m := range mismatches {
			t.Errorf("differential: %s", m)
		}
	}
}

// backendAvailable probes whether an engine can be reached right now.
func backendAvailable(backend string) bool {
	if backend != "postgres" {
		return true
	}
	db, err := sql.Open(postgresmod.DriverPostgres, harness.PostgresDSN())
	if err != nil {
		return false
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return db.PingContext(ctx) == nil
}

func diffKeys() []string {
	keys := make([]string, 0, len(harness.DiffLog))
	for k := range harness.DiffLog {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestLoadSmoke runs a small load tier so traffic-shape regressions (e.g.
// a change that serializes all requests) surface in CI, not production.
func TestLoadSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("load smoke skipped in -short mode")
	}
	t.Chdir(t.TempDir())
	h, err := harness.SpinUp(harness.Opts{
		Name:   "load-smoke",
		Schema: "shop",
		Seeds: map[string]harness.SeedQuery{
			"getUser": {Query: `query getUser($id = ID) { users(id: $id) { id full_name } }`},
		},
		Budgets: harness.Budgets{
			MaxDepth: 8, MaxRows: 500, Concurrency: 10,
			Timeout: 15 * 1000 * 1000 * 1000, HealthTimeout: 15 * 1000 * 1000 * 1000,
		},
	})
	if err != nil {
		t.Fatalf("spin-up: %v", err)
	}
	defer h.Stop()

	if err := scenarios.RunLoad(h, 200); err != nil {
		t.Fatal(err)
	}
}
