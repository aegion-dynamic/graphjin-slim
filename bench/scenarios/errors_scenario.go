package scenarios

import (
	"fmt"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "error_envelopes", Fn: errorEnvelopes})
}

// errorEnvelopes requires every failure class to surface as a clean
// GraphQL errors entry — never a crash, stack trace or leaked internals.
func errorEnvelopes(h *harness.H) error {
	cases := []struct {
		name  string
		query string
	}{
		{"syntax", `{ users { id `},
		{"unknown-root", `{ nosuchtable { id } }`},
		{"unknown-field", `{ users { no_such_column } }`},
		{"unknown-directive", `{ users @nosuchdirective { id } }`},
	}
	for _, tc := range cases {
		resp, err := h.GQL(tc.query, nil)
		if err != nil {
			return fmt.Errorf("%s: transport failed instead of erroring: %w", tc.name, err)
		}
		msg := firstError(resp)
		if msg == "" {
			return fmt.Errorf("%s: accepted without errors[]: %v", tc.name, resp)
		}
		for _, leak := range []string{"goroutine", "panic:", "/home/", ".go:"} {
			if strings.Contains(msg, leak) {
				return fmt.Errorf("%s: error leaks internals (%q): %s", tc.name, leak, msg)
			}
		}
	}

	// Variable type mismatch is rejected server-side.
	resp, err := h.GQL(
		`query q($id: Int) { users(id: $id) { id } }`, map[string]any{"id": "not-an-int"})
	if err != nil {
		return err
	}
	// Accepted coercion paths differ; what must never happen is executing
	// against an unintended row.
	if firstError(resp) == "" {
		if d, _ := resp["data"].(map[string]any); d != nil && d["users"] != nil {
			return fmt.Errorf("string-bound Int variable produced data: %v", resp)
		}
	}

	// Unknown variable reference.
	resp, err = h.GQL(`{ users(id: $neverdeclared) { id } }`, nil)
	if err != nil {
		return err
	}
	if firstError(resp) == "" {
		return fmt.Errorf("undeclared variable accepted")
	}

	// In production mode the same failures must not get chattier than in
	// dev — compare message classes across variants via a sibling check:
	// prod-specific behavior lives in the prodsec scenario; here we only
	// require that dev-mode messages name the offending field.
	if !h.Prod {
		resp, err = h.GQL(`{ users { no_such_column } }`, nil)
		if err != nil {
			return err
		}
		if msg := firstError(resp); !strings.Contains(msg, "no_such_column") {
			return fmt.Errorf("field name missing from validation error: %q", msg)
		}
	}
	return nil
}
