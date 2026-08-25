package scenarios

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "aggregates_full", Fn: aggregatesFull})
	register(Scenario{Name: "aggregates_nested", Fn: aggregatesNested})
}

// aggregatesFull checks every aggregate op against hand-computed values.
func aggregatesFull(h *harness.H) error {
	data, err := h.MustData(`{ products { count_id sum_price avg_price min_price max_price } }`, nil)
	if err != nil {
		return err
	}
	row := data["products"].([]any)[0].(map[string]any)
	checks := map[string]float64{
		"count_id":  4,
		"sum_price": 1688.48,
		"avg_price": 422.12, // 1688.48 / 4 = 422.12 exactly
		"min_price": 39.99,
		"max_price": 999.99,
	}
	for k, want := range checks {
		got, ok := row[k].(float64)
		if !ok {
			return fmt.Errorf("%s missing or non-numeric: %v", k, row[k])
		}
		if diff := got - want; diff < -0.001 || diff > 0.001 {
			return fmt.Errorf("%s = %v, want ≈%v", k, got, want)
		}
	}
	return nil
}

// aggregatesNested requires aggregation INSIDE a relationship — this is
// where the nested JSON machinery earns its keep.
func aggregatesNested(h *harness.H) error {
	data, err := h.MustData(
		`{ users(orderBy: { id: asc }) { id full_name orders { count_id sum_total } } }`, nil)
	if err != nil {
		return fmt.Errorf("nested aggregate query: %w", err)
	}
	users, _ := data["users"].([]any)
	if len(users) != 3 {
		return fmt.Errorf("users = %d, want 3", len(users))
	}

	type want struct {
		count float64
		sum   float64
	}
	expect := map[float64]want{
		1: {2, 1089.48}, // ada: 1049.49 + 39.99
		2: {1, 499.50},  // alan
		3: {0, 0},       // grace: no orders — empty aggregate must still be well-shaped
	}
	for _, u := range users {
		m := u.(map[string]any)
		id := m["id"].(float64)
		orders, ok := m["orders"].([]any)
		if !ok || len(orders) == 0 {
			if expect[id].count == 0 {
				continue // legitimately empty
			}
			return fmt.Errorf("user %v: orders missing: %v", id, m["orders"])
		}
		row := orders[0].(map[string]any)
		w := expect[id]
		if c, _ := row["count_id"].(float64); c != w.count {
			return fmt.Errorf("user %v count = %v, want %v", id, c, w.count)
		}
		if s, _ := row["sum_total"].(float64); s-w.sum > 0.001 || w.sum-s > 0.001 {
			return fmt.Errorf("user %v sum = %v, want ≈%v", id, s, w.sum)
		}
	}
	return nil
}
