package scenarios

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "conflict_update", Fn: conflictUpdate})
	register(Scenario{Name: "conflict_get", Fn: conflictGet})
}

// conflictUpdate guards the unique constraint: a duplicate insert with no
// on_conflict clause must surface as a clean error envelope and leave the
// table untouched. (slim supports on_conflict: get only; upsert-style
// rewrites are deliberately out of scope.)
func conflictUpdate(h *harness.H) error {
	resp, err := h.GQL(
		`mutation { users(insert: { email: "ada@example.com", full_name: "Impostor Ada" }) { id } }`, nil)
	if err != nil {
		return fmt.Errorf("transport failed on duplicate insert: %w", err)
	}
	if msg := firstErrorLocal(resp); msg == "" {
		return fmt.Errorf("duplicate insert accepted without unique violation: %v", resp)
	}

	// The original row must be exactly as seeded.
	data, err := h.MustData(`{ users(where: { email: { eq: "ada@example.com" } }) { id full_name } }`, nil)
	if err != nil {
		return err
	}
	if rows := listLen(data["users"]); rows != 1 {
		return fmt.Errorf("ada rows = %d, want 1", rows)
	}
	if n, _ := firstRowField(data, "users", "full_name"); n != "Ada Lovelace" {
		return fmt.Errorf("original row mutated to %v", n)
	}
	return nil
}

func firstErrorLocal(m map[string]any) string {
	errs, _ := m["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	if em, ok := errs[0].(map[string]any); ok {
		s, _ := em["message"].(string)
		return s
	}
	return "?"
}

// conflictGet proves on_conflict:get returns the pre-existing row that
// collides on the unique key instead of writing anything.
func conflictGet(h *harness.H) error {
	data, err := h.MustData(
		`mutation { users(insert: { email: "grace@example.com", full_name: "Should Not Exist" }, on_conflict: get) { id full_name } }`, nil)
	if err != nil {
		return fmt.Errorf("get-conflict: %w", err)
	}
	got, err := firstRowField(data, "users", "full_name")
	if err != nil {
		return err
	}
	// The original Grace must come back untouched.
	if got != "Grace Hopper" {
		return fmt.Errorf("on_conflict get returned %q, want the stored row", got)
	}

	// And the poisoned name must not exist anywhere afterwards.
	data, err = h.MustData(`{ users(where: { full_name: { eq: "Should Not Exist" } }) { id } }`, nil)
	if err != nil {
		return err
	}
	if n := listLen(data["users"]); n != 0 {
		return fmt.Errorf("conflict-get wrote a row anyway (%d matches)", n)
	}
	return nil
}
