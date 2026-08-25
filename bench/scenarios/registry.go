// Package scenarios holds every end-to-end scenario. Each file registers
// itself via init(); the runner in the root main.go executes them.
package scenarios

import "github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"

// Scenario is one self-contained end-to-end check against a live service.
type Scenario struct {
	Name     string
	Variants []string // subset of {"dev", "prod"}; empty means dev only
	Fn       func(h *harness.H) error
	Schema   string                       // "shop" (default) | "chain" | "blob"
	ChainN   int                          // table count for Schema=="chain"
	Seeds    map[string]harness.SeedQuery // saved queries written pre-start
}

// Opts renders the spin-up options for one variant on one backend.
func (sc Scenario) Opts(variant string, backend string, b harness.Budgets) harness.Opts {
	schema := sc.Schema
	if schema == "" {
		schema = "shop"
	}
	return harness.Opts{
		Name:    sc.Name + "-" + variant,
		Prod:    variant == "prod",
		Backend: backend,
		Schema:  schema,
		ChainN:  sc.ChainN,
		Seeds:   sc.Seeds,
		Budgets: b,
	}
}

// All is populated by init() calls across this package.
var All []Scenario

func register(s Scenario) {
	if len(s.Variants) == 0 {
		s.Variants = []string{"dev"}
	}
	All = append(All, s)
}

// optsFor maps a scenario onto harness options for a given variant,
// applying budgets and depth caps.
func optsFor(name string, variant string, b harness.Budgets, schema string, chainN int) harness.Opts {
	return harness.Opts{
		Name:    name + "-" + variant,
		Prod:    variant == "prod",
		Schema:  schema,
		ChainN:  chainN,
		Budgets: b,
	}
}
