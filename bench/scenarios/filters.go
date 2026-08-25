package scenarios

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "filter_matrix", Fn: filterMatrix})
	register(Scenario{Name: "filter_logic", Fn: filterLogic})
}

// idsOf extracts the sorted id list from a rows payload as float keys.
func idsOf(t interface{ Errorf(string, ...any) }, v any) []float64 {
	rows, ok := v.([]any)
	if !ok {
		return nil
	}
	out := []float64{}
	for _, r := range rows {
		if m, ok := r.(map[string]any); ok {
			if id, ok := m["id"].(float64); ok {
				out = append(out, id)
			}
		}
	}
	return out
}

func sameIDs(got []float64, want ...float64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// filterMatrix sweeps the comparison operators against known values.
func filterMatrix(h *harness.H) error {
	cases := []struct {
		name  string
		where string
		want  []float64 // product ids, ascending
	}{
		{"eq", `{ price: { eq: 39.99 } }`, []float64{4}},
		{"neq", `{ price: { neq: 999.99 } }`, []float64{2, 3, 4}},
		{"gt", `{ price: { gt: 149.0 } }`, []float64{1, 2}},
		{"gte", `{ price: { gte: 149.0 } }`, []float64{1, 2, 3}},
		{"lt", `{ price: { lt: 149.0 } }`, []float64{4}},
		{"lte", `{ price: { lte: 149.0 } }`, []float64{3, 4}},
		{"in", `{ id: { in: [1, 3] } }`, []float64{1, 3}},
	}
	for _, tc := range cases {
		data, err := h.MustData(
			fmt.Sprintf(`{ products(where: %s, orderBy: { id: asc }) { id } }`, tc.where), nil)
		if err != nil {
			return fmt.Errorf("%s: %w", tc.name, err)
		}
		got := idsOf(nil, data["products"])
		if !sameIDs(got, tc.want...) {
			return fmt.Errorf("%s → ids %v, want %v", tc.name, got, tc.want)
		}
	}
	return nil
}

// filterLogic exercises boolean composition and null handling.
func filterLogic(h *harness.H) error {
	// AND of two clauses.
	data, err := h.MustData(
		`{ products(where: { and: [{ price: { gte: 100 } }, { price: { lt: 500 } }] }, orderBy: { id: asc }) { id } }`, nil)
	if err != nil {
		return err
	}
	if got := idsOf(nil, data["products"]); !sameIDs(got, 2, 3) {
		return fmt.Errorf("and → %v, want [2 3]", got)
	}

	// OR across fields.
	data, err = h.MustData(
		`{ products(where: { or: [{ price: { lte: 39.99 } }, { price: { gte: 999.99 } }] }, orderBy: { id: asc }) { id } }`, nil)
	if err != nil {
		return err
	}
	if got := idsOf(nil, data["products"]); !sameIDs(got, 1, 4) {
		return fmt.Errorf("or → %v, want [1 4]", got)
	}

	// NULL semantics through a nullable column: only Alan has bio NULL.
	data, err = h.MustData(`{ users(where: { bio: { isNull: true } }) { id } }`, nil)
	if err != nil {
		// isNull may be spelled differently by the schema; try not-equals form.
		data, err = h.MustData(`{ users(where: { bio: { equals: null } }) { id } }`, nil)
		if err != nil {
			return fmt.Errorf("null filter: %w", err)
		}
	}
	if got := idsOf(nil, data["users"]); !sameIDs(got, 2) {
		return fmt.Errorf("null bio → %v, want [2]", got)
	}
	return nil
}
