// Command bench runs end-to-end scenarios against real GraphJin services
// backed by plain SQLite files.
//
//	Usage:
//	  go run . e2e   [-run name] [-variant dev|prod|all] [-extreme]
//	  go run . load  [-total N]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
	"github.com/aegion-dynamic/graphjin-slim/bench/v3/scenarios"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 || (os.Args[1] != "e2e" && os.Args[1] != "load") {
		fmt.Println("usage: bench e2e|load [-run name] [-variant dev|prod|all] [-extreme] [-total N]")
		return 2
	}
	mode := os.Args[1]

	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	runFilter := fs.String("run", "", "only scenarios whose name contains this")
	variant := fs.String("variant", "all", "dev, prod, or all")
	extreme := fs.Bool("extreme", false, "raise depth/rows/concurrency budgets")
	total := fs.Int("total", 2000, "load tier request count")
	fs.Parse(os.Args[2:])

	budgets := harness.DefaultBudgets()
	if *extreme {
		budgets = harness.ExtremeBudgets()
	}

	switch mode {
	case "e2e":
		return runE2E(*runFilter, *variant, budgets)
	case "load":
		return runLoad(*runFilter, budgets, *total)
	}
	return 2
}

func runE2E(filter, variant string, budgets harness.Budgets) int {
	origWd, _ := os.Getwd()

	var pass, fail int
	for _, sc := range scenarios.All {
		if filter != "" && !contains(sc.Name, filter) {
			continue
		}
		for _, v := range sc.Variants {
			if variant != "all" && v != variant {
				continue
			}

			root, err := os.MkdirTemp("", "gjbench-")
			if err != nil {
				fmt.Printf("FAIL %s [%s]: tempdir: %v\n", sc.Name, v, err)
				fail++
				continue
			}
			if err := os.Chdir(root); err != nil {
				fmt.Printf("FAIL %s [%s]: chdir: %v\n", sc.Name, v, err)
				fail++
				continue
			}

			opts := harness.Opts{Name: sc.Name, Prod: v == "prod", Budgets: budgets, Schema: "shop"}
			if sc.Name == "rest_saved_queries" {
				opts.Seeds = map[string]harness.SeedQuery{
					"getUser": {Query: `query getUser($id = ID) { users(id: $id) { id full_name email } }`, Vars: map[string]any{"id": 1}},
				}
			}
			if sc.Name == "openapi_spec" {
				opts.Seeds = map[string]harness.SeedQuery{
					"getUser": {Query: `query getUser($id = ID) { users(id: $id) { id full_name email } }`, Vars: map[string]any{"id": 1}},
				}
			}
			if sc.Name == "prodsec" {
				opts.Seeds = map[string]harness.SeedQuery{
					"getUser": {Query: `query getUser($id = ID) { users(id: $id) { id full_name email } }`},
				}
			}
			if sc.Name == "fk_depth" || sc.Name == "fk_recursion_guard" {
				opts.Schema = "chain"
			}
			if sc.Name == "edge_cases" {
				opts.Schema = "shop"
			}
			h, err := harness.SpinUp(opts)
			if err != nil {
				fmt.Printf("FAIL %s [%s]: spin-up: %v\n", sc.Name, v, err)
				cleanup(origWd, root)
				fail++
				continue
			}

			t0 := time.Now()
			err = guarded(h, budgets, func() error { return sc.Fn(h) })
			dur := time.Since(t0)

			h.Stop()
			cleanup(origWd, root)

			if err != nil {
				fmt.Printf("FAIL %-24s [%s] %6.1fs  %v\n", sc.Name, v, dur.Seconds(), err)
				fail++
			} else {
				fmt.Printf("PASS %-24s [%s] %6.1fs\n", sc.Name, v, dur.Seconds())
				pass++
			}
		}
	}

	fmt.Printf("\n%d passed, %d failed\n", pass, fail)
	if fail != 0 {
		return 1
	}
	return 0
}

func runLoad(filter string, budgets harness.Budgets, total int) int {
	origWd, _ := os.Getwd()
	root, _ := os.MkdirTemp("", "gjbench-load")
	defer cleanup(origWd, root)
	os.Chdir(root)

	h, err := harness.SpinUp(harness.Opts{
		Name: "load", Schema: "shop", Budgets: budgets,
		Seeds: map[string]harness.SeedQuery{
			"getUser": {
				Query: `query getUser($id = ID) { users(id: $id) { id full_name } }`,
			},
		},
	})
	if err != nil {
		fmt.Printf("FAIL load: spin-up: %v\n", err)
		return 1
	}
	defer h.Stop()

	if err := scenarios.RunLoad(h, total); err != nil {
		fmt.Printf("FAIL load: %v\n", err)
		return 1
	}
	return 0
}

// guarded runs fn under the scenario timeout so a hung request surfaces as
// a failure instead of freezing the whole suite.
func guarded(h *harness.H, b harness.Budgets, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(b.Timeout + 10*time.Second):
		return fmt.Errorf("scenario exceeded %s deadline", b.Timeout+10*time.Second)
	}
}

func cleanup(origWd, root string) {
	os.Chdir(origWd)
	os.RemoveAll(root)
}

func contains(name, substr string) bool {
	return substr == "" || len(name) >= len(substr) && (name == substr ||
		len(substr) > 0 && indexOf(name, substr) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

var _ = filepath.Join
