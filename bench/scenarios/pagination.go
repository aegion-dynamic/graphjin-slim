package scenarios

import (
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "pagination_cursor", Fn: paginationCursor})
	register(Scenario{Name: "pagination_offset", Fn: paginationOffset})
}

// paginationCursor walks all users two-at-a-time with opaque cursors and
// requires: exact page contents, no duplicates, and a clean end signal.
func paginationCursor(h *harness.H) error {
	seen := map[float64]bool{}
	cursor := ""
	total := 0

	for page := 0; page < 5; page++ { // hard bound; 3 rows should finish in 2 pages
		q := `query walk($users_cursor: String) { users(first: 2, after: $users_cursor, orderBy: { id: asc }) { id full_name users_cursor } }`
		// The variable must always be bound; page zero carries "".
		vars := map[string]any{"users_cursor": cursor}
		data, err := h.MustData(q, vars)
		if err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}
		rows, ok := data["users"].([]any)
		if !ok {
			return fmt.Errorf("page %d: users missing: %v", page, data)
		}

		for _, r := range rows {
			m := r.(map[string]any)
			id, _ := m["id"].(float64)
			if seen[id] {
				return fmt.Errorf("page %d: duplicate id %v", page, id)
			}
			seen[id] = true
			total++
		}

		// The next-page cursor is a top-level sibling of the rows.
		next, _ := data["users_cursor"].(string)

		if len(rows) == 0 {
			// Terminal page: no rows left beyond the cursor.
			if total != 3 {
				return fmt.Errorf("walked %d rows, want exactly 3", total)
			}
			return nil
		}
		if next == "" {
			return fmt.Errorf("stuck: only %d rows but cursor exhausted", total)
		}
		cursor = next
	}
	return fmt.Errorf("cursor walk did not terminate within 5 pages")
}

// paginationOffset proves limit/offset windows slice deterministically.
func paginationOffset(h *harness.H) error {
	data, err := h.MustData(`{ products(limit: 2, offset: 1, orderBy: { id: asc }) { id } }`, nil)
	if err != nil {
		return err
	}
	rows, _ := data["products"].([]any)
	if len(rows) != 2 {
		return fmt.Errorf("window size = %d, want 2", len(rows))
	}
	want := []float64{2, 3}
	for i, r := range rows {
		id := r.(map[string]any)["id"].(float64)
		if id != want[i] {
			return fmt.Errorf("window[%d].id = %v, want %v", i, id, want[i])
		}
	}
	return nil
}
