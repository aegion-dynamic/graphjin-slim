package scenarios

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{
		Name:        "multischema",
		Fn:          multischema,
		Backends:    []string{"postgres"},
		Multischema: true,
	})
}

func multischema(h *harness.H) error {
	// Alias root into the non-default schema, exact values.
	data, err := h.MustData(`{ orders_archiveOfArchive(orderBy: { id: asc }) { order_id note } }`, nil)
	if err != nil {
		return fmt.Errorf("alias root: %w", err)
	}
	rows, err := harness.Walk(data, "orders_archiveOfArchive")
	if err != nil {
		return err
	}
	list, ok := rows.([]any)
	if !ok || len(list) != 2 {
		return fmt.Errorf("archive rows = %v, want 2", rows)
	}
	r0 := list[0].(map[string]any)
	if r0["order_id"] != float64(1) || r0["note"] != "first archive" {
		return fmt.Errorf("row[0] = %v", r0)
	}

	// Nested join across schemas via the archive -> public FK.
	data, err = h.MustData(`{ orders(id: 1) { status orders_archive { note } } }`, nil)
	if err != nil {
		return fmt.Errorf("nested cross-schema: %w", err)
	}
	notes, err := harness.Walk(data, "orders.orders_archive")
	if err != nil {
		return err
	}
	nl, ok := notes.([]any)
	if !ok || len(nl) != 1 || nl[0].(map[string]any)["note"] != "first archive" {
		return fmt.Errorf("nested notes = %v", notes)
	}

	// Aggregate over the alias root.
	data, err = h.MustData(`{ orders_archiveOfArchive { count_id } }`, nil)
	if err != nil {
		return fmt.Errorf("alias aggregate: %w", err)
	}
	n, err := harness.Walk(data, "orders_archiveOfArchive.0.count_id")
	if err != nil {
		return err
	}
	if n != float64(2) {
		return fmt.Errorf("alias count = %v, want 2", n)
	}
	return nil
}
