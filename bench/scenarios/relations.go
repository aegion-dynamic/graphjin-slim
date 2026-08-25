package scenarios

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "relations_m2m", Fn: relationsM2M})
	register(Scenario{Name: "relations_deep", Fn: relationsDeep})
	register(Scenario{Name: "relations_shapes", Fn: relationsShapes})
}

// relationsM2M proves many-to-many traversal through the join table.
func relationsM2M(h *harness.H) error {
	data, err := h.MustData(
		`{ products(orderBy: { id: asc }) { id tags { label } } }`, nil)
	if err != nil {
		return fmt.Errorf("m2m query: %w", err)
	}
	rows, _ := data["products"].([]any)
	if len(rows) != 4 {
		return fmt.Errorf("products = %d, want 4", len(rows))
	}

	expect := map[float64][]string{
		1: {"vintage", "computing"},
		2: {"computing"},
		3: {"classic"},
	}
	for _, r := range rows {
		m := r.(map[string]any)
		id := m["id"].(float64)
		want, ok := expect[id]
		if !ok {
			continue // product 4 has no tags; absence must be tolerated
		}
		tags, ok := m["tags"].([]any)
		if !ok || len(tags) != len(want) {
			return fmt.Errorf("product %v tags = %v, want %v", id, m["tags"], want)
		}
		for i, t := range tags {
			if got := t.(map[string]any)["label"]; got != want[i] {
				return fmt.Errorf("product %v tag[%d] = %v, want %v", id, i, got, want[i])
			}
		}
	}
	return nil
}

// relationsDeep walks users → orders → items → products: three levels of
// nesting through two different FK hops with aggregation at the leaf.
func relationsDeep(h *harness.H) error {
	data, err := h.MustData(
		`{ users(id: 1) {
			full_name
			orders(orderBy: { id: asc }) {
				status total
				items { qty product { name price } }
			}
		} }`, nil)
	if err != nil {
		return fmt.Errorf("deep chain: %w", err)
	}
	orders, err := harness.Walk(data, "users.orders")
	if err != nil {
		return err
	}
	list, _ := orders.([]any)
	if len(list) != 2 {
		return fmt.Errorf("ada orders = %d, want 2", len(list))
	}

	o0 := list[0].(map[string]any)
	if o0["status"] != "paid" {
		return fmt.Errorf("order[0].status = %v", o0["status"])
	}
	items, _ := o0["items"].([]any)
	if len(items) != 2 {
		return fmt.Errorf("order[0] items = %d, want 2", len(items))
	}
	first := items[0].(map[string]any)
	if first["qty"] != float64(1) {
		return fmt.Errorf("item qty = %v", first["qty"])
	}
	prod := first["product"].(map[string]any)
	if prod["name"] != "Analytical Engine" || prod["price"] != 999.99 {
		return fmt.Errorf("item product = %v", prod)
	}
	return nil
}

// relationsShapes checks reverse traversal, aliases and __typename —
// the GraphQL-shape features that pure SQL generators tend to forget.
func relationsShapes(h *harness.H) error {
	data, err := h.MustData(
		`{
			ada: users(id: 1) { full_name __typename }
			back: orders(where: { status: { eq: "shipped" } }) { user { full_name email } }
		}`, nil)
	if err != nil {
		return fmt.Errorf("aliases/reverse: %w", err)
	}
	name, _ := harness.Walk(data, "ada.full_name")
	if name != "Ada Lovelace" {
		return fmt.Errorf("aliased root = %v", name)
	}
	tn, _ := harness.Walk(data, "ada.__typename")
	if tn != "users" && tn != "user" && tn != "User" {
		return fmt.Errorf("__typename = %v (unexpected normalization)", tn)
	}
	owner, err := harness.Walk(data, "back.0.user.full_name")
	if err != nil {
		return fmt.Errorf("reverse order→user: %w", err)
	}
	if owner != "Alan Turing" {
		return fmt.Errorf("reverse owner = %v, want Alan Turing", owner)
	}
	return nil
}
