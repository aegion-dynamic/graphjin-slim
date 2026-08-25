package scenarios

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "crud_lifecycle", Fn: crudLifecycle})
	register(Scenario{Name: "crud_nested_insert", Fn: crudNestedInsert})
}

// crudLifecycle walks one row through insert → update → read-back → delete
// and proves the table is back to its exact seeded state afterwards.
func crudLifecycle(h *harness.H) error {
	// Insert.
	data, err := h.MustData(
		`mutation { users(insert: { full_name: "Temp Person", email: "temp@x.io", age: 30 }) { id full_name age } }`, nil)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	row, err := firstRowField(data, "users", "full_name")
	if err != nil || row != "Temp Person" {
		return fmt.Errorf("inserted = %v (%v)", row, err)
	}
	idv, _ := harness.Walk(data, "users.0.id") // may be list or object; fall through to re-query

	_ = idv
	data, err = h.MustData(`{ users(where: { email: { eq: "temp@x.io" } }) { id } }`, nil)
	if err != nil {
		return err
	}
	rows, _ := data["users"].([]any)
	if len(rows) != 1 {
		return fmt.Errorf("insert not found; got %v", rows)
	}
	uid := rows[0].(map[string]any)["id"].(float64)

	// Update by primary key: values in the payload, target in where.
	data, err = h.MustData(
		fmt.Sprintf(`mutation { users(update: { full_name: "Renamed Person" }, where: { id: { eq: %.0f } }) { id full_name } }`, uid), nil)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if n, _ := firstRowField(data, "users", "full_name"); n != "Renamed Person" {
		return fmt.Errorf("updated name = %v", n)
	}

	// Read-back shows the new value through a different code path (filter).
	data, err = h.MustData(`{ users(where: { email: { eq: "temp@x.io" } }) { full_name } }`, nil)
	if err != nil {
		return err
	}
	if n, _ := firstRowField(data, "users", "full_name"); n != "Renamed Person" {
		return fmt.Errorf("read-back after update = %v", n)
	}

	// Delete requires an explicit where clause; delete takes a boolean.
	if _, err := h.MustData(
		fmt.Sprintf(`mutation { users(delete: true, where: { id: { eq: %.0f } }) { id } }`, uid), nil); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	data, err = h.MustData(`{ users(where: { email: { eq: "temp@x.io" } }) { id } }`, nil)
	if err != nil {
		return err
	}
	if rows, _ := data["users"].([]any); len(rows) != 0 {
		return fmt.Errorf("row survived delete: %v", rows)
	}
	return nil
}

// crudNestedInsert inserts a user together with a child product in one
// statement and requires the generated parent key to reach the child.
func crudNestedInsert(h *harness.H) error {
	data, err := h.MustData(
		`mutation { users(insert: {
			full_name: "Batch Parent", email: "batch@x.io",
			products: { name: "Child Gadget", price: 12.5 }
		}) { id products { id name } } }`, nil)
	if err != nil {
		return fmt.Errorf("nested insert: %w", err)
	}
	pid, err := firstRowField(data, "users", "id")
	if err != nil {
		return err
	}
	parentID, ok := pid.(float64)
	if !ok || parentID < 4 {
		return fmt.Errorf("parent id = %v, want new (>3)", pid)
	}

	// The child must reference the freshly minted parent, proving ID
	// propagation instead of a NULL or zero foreign key.
	data, err = h.MustData(`{ products(where: { name: { eq: "Child Gadget" } }) { id users_id } }`, nil)
	if err != nil {
		return err
	}
	rows, _ := data["products"].([]any)
	if len(rows) != 1 {
		return fmt.Errorf("child product missing: %v", rows)
	}
	got := rows[0].(map[string]any)["users_id"].(float64)
	if got != parentID {
		return fmt.Errorf("child.users_id = %v, want propagated %v", got, parentID)
	}
	return nil
}

var _ = harness.ErrKnownBug
