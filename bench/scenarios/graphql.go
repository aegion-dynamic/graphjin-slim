package scenarios

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "graphql_basic", Fn: graphqlBasic})
	register(Scenario{Name: "graphql_mutations", Fn: graphqlMutations})
	register(Scenario{Name: "introspection", Fn: introspection})
}

const shopUsersByOrderAsc = `{ users(orderBy: { id: asc }) { id full_name email } }`

func graphqlBasic(h *harness.H) error {
	data, err := h.MustData(`{ users(orderBy: { id: asc }) { id full_name email } }`, nil)
	if err != nil {
		return err
	}
	users, err := harness.Walk(data, "users")
	if err != nil {
		return err
	}
	list, ok := users.([]any)
	if !ok || len(list) != 3 {
		return fmt.Errorf("want 3 users, got %v", users)
	}
	wantNames := []string{"Ada Lovelace", "Alan Turing", "Grace Hopper"}
	for i, u := range list {
		name, _ := harness.Walk(u.(map[string]any), "full_name")
		if name != wantNames[i] {
			return fmt.Errorf("user[%d] = %v, want %s", i, name, wantNames[i])
		}
	}

	// FK nesting one level deep, with exact value checks.
	data, err = h.MustData(`{ users(id: 1) { full_name products { name price } } }`, nil)
	if err != nil {
		return fmt.Errorf("nested users->products: %w", err)
	}
	prods, err := harness.Walk(data, "users.products")
	if err != nil {
		return err
	}
	pl, ok := prods.([]any)
	if !ok || len(pl) != 1 {
		return fmt.Errorf("ada products = %v, want exactly 1", prods)
	}
	p0 := pl[0].(map[string]any)
	if p0["name"] != "Analytical Engine" || p0["price"] != 999.99 {
		return fmt.Errorf("product = %v", p0)
	}

	// Aggregates.
	data, err = h.MustData(`{ products { count_id sum_price } }`, nil)
	if err != nil {
		return err
	}
	row := data["products"].([]any)[0].(map[string]any)
	if row["count_id"] != float64(4) {
		return fmt.Errorf("count_id = %v, want 4", row["count_id"])
	}
	if row["sum_price"] != 1688.48 {
		return fmt.Errorf("sum_price = %v, want 1688.48", row["sum_price"])
	}

	// Singular root by primary key.
	data, err = h.MustData(`{ usersByID(id: 2) { full_name } }`, nil)
	if err != nil {
		return fmt.Errorf("usersByID: %w", err)
	}
	name, _ := harness.Walk(data, "usersByID.full_name")
	if name != "Alan Turing" {
		return fmt.Errorf("usersByID.full_name = %v", name)
	}
	return nil
}

func graphqlMutations(h *harness.H) error {
	data, err := h.MustData(
		`mutation { products(insert: { name: "Gráice 🚀 Probe", price: 42.0 }) { id name } }`, nil)
	if err != nil {
		return err
	}
	inserted, err := firstRowField(data, "products", "name")
	if err != nil || inserted != "Gráice 🚀 Probe" {
		return fmt.Errorf("inserted name = %v (%v)", inserted, err)
	}

	// Read it back filtered, proving unicode round-trips the compiler.
	data, err = h.MustData(
		`{ products(where: { name: { eq: "Gráice 🚀 Probe" } }) { id name price } }`, nil)
	if err != nil {
		return fmt.Errorf("unicode read-back: %w", err)
	}
	if n := listLen(data["products"]); n != 1 {
		return fmt.Errorf("unicode read-back = %v, want 1 row", data["products"])
	}
	return nil
}

func introspection(h *harness.H) error {
	resp, err := h.GQL(`query IntrospectionQuery {
		__schema {
			queryType { name }
			mutationType { name }
			subscriptionType { name }
			types { kind name fields { name type { kind name ofType { kind name } } } }
		}
	}`, nil)
	if err != nil {
		return err
	}
	schema, err := harness.Walk(resp, "data.__schema")
	if err != nil {
		return err
	}
	sm := schema.(map[string]any)

	if sm["subscriptionType"] != nil {
		return fmt.Errorf("subscriptionType = %v, want null in slim build", sm["subscriptionType"])
	}
	types, _ := sm["types"].([]any)
	for _, tv := range types {
		tm := tv.(map[string]any)
		if tm["name"] == "Subscription" {
			return fmt.Errorf("introspection must not include a Subscription type")
		}
	}

	// Reverse relations must exist in both directions.
	users, err := findType(types, "users")
	if err != nil {
		return err
	}
	if !hasField(users, "products") {
		return fmt.Errorf("users type lacks reverse-relation field products")
	}
	products, err := findType(types, "products")
	if err != nil {
		return err
	}
	if !hasField(products, "users") {
		return fmt.Errorf("products type lacks reverse-relation field users")
	}
	return nil
}

func findType(types []any, name string) (map[string]any, error) {
	for _, tv := range types {
		tm := tv.(map[string]any)
		if tm["name"] == name {
			return tm, nil
		}
	}
	return nil, fmt.Errorf("type %q not found in introspection", name)
}

func hasField(t map[string]any, field string) bool {
	fields, _ := t["fields"].([]any)
	for _, fv := range fields {
		if fm := fv.(map[string]any); fm["name"] == field {
			return true
		}
	}
	return false
}
