package main

import (
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
	"github.com/aegion-dynamic/graphjin-slim/bench/v3/scenarios"
)

// TestE2EScenarios runs every registered end-to-end scenario against a
// real service for each of its variants. This is the CI entry point:
// `go test ./...` in this module gates merges on the full battery.
func TestE2EScenarios(t *testing.T) {
	budgets := harness.DefaultBudgets()
	for _, sc := range scenarios.All {
		for _, v := range sc.Variants {
			sc, v := sc, v
			t.Run(sc.Name+"/"+v, func(t *testing.T) {
				t.Chdir(t.TempDir())
				h, err := harness.SpinUp(sc.Opts(v, budgets))
				if err != nil {
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

	// Differential oracle: every key recorded by the differential battery
	// must carry identical canonical payloads from both variants.
	var mismatches []string
	for _, key := range diffKeys() {
		got := harness.DiffLog[key]
		if got["dev"] == "" || got["prod"] == "" {
			mismatches = append(mismatches, key+": missing a variant")
			continue
		}
		if got["dev"] != got["prod"] {
			mismatches = append(mismatches,
				fmt.Sprintf("%s\n  dev : %s\n  prod: %s", key, got["dev"], got["prod"]))
		}
	}
	if len(mismatches) > 0 {
		for _, m := range mismatches {
			t.Errorf("differential: %s", m)
		}
	}
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
