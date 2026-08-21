package scenarios

import (
	"fmt"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{
		Name:     "prodsec",
		Variants: []string{"prod"},
		Fn:       prodSec,
	})
	register(Scenario{Name: "edge_cases", Fn: edgeCases})
}

const prodGetUser = `query getUser($id = ID) { users(id: $id) { id full_name email } }`

// prodSec runs against a production-mode service whose queries directory
// ships exactly one saved query. Everything else must be rejected.
func prodSec(h *harness.H) error {
	// Ad-hoc GraphQL is refused.
	resp, err := h.GQL(`{ users { id } }`, nil)
	if err != nil {
		return err
	}
	if firstError(resp) == "" {
		return fmt.Errorf("ad-hoc query executed in production mode")
	}

	// The shipped saved query runs with explicit vars…
	status, body, _, err := h.Rest("GET", "getUser", urlVals("id", "2"), nil)
	if err != nil || status != 200 {
		return fmt.Errorf("saved call failed: status=%d err=%v", status, err)
	}
	if name, _ := harness.Walk(body, "data.users.full_name"); name != "Alan Turing" {
		return fmt.Errorf("saved call → %v", body)
	}

	// …and fails loudly without them.
	status, body, _, err = h.Rest("GET", "getUser", nil, nil)
	if err != nil {
		return err
	}
	if body["data"] != nil && firstError(body) == "" {
		return fmt.Errorf("bare prod call should require the variable, got %v", body)
	}

	// Saved queries are also executable through the GraphQL endpoint by
	// operation name — the production contract.
	named := strings.Replace(prodGetUser, "query getUser", "query getUser", 1)
	resp, err = h.GQL(named, map[string]any{"id": 3})
	if err != nil {
		return err
	}
	if firstError(resp) != "" {
		return fmt.Errorf("named prod graphql call: %v", firstError(resp))
	}
	if name, _ := harness.Walk(resp, "data.users.full_name"); name != "Grace Hopper" {
		return fmt.Errorf("named prod call → %v", resp["data"])
	}
	return nil
}

// edgeCases pokes the odd corners: unicode, empty sets, oversized
// payloads, malformed input, unknown fields.
func edgeCases(h *harness.H) error {
	// Unicode round-trip through insert + filter.
	if _, err := h.MustData(
		`mutation { products(insert: { name: "Gráice 🚀 Probe", price: 1.25 }) { id } }`, nil); err != nil {
		return fmt.Errorf("unicode insert: %w", err)
	}
	data, err := h.MustData(
		`{ products(where: { name: { eq: "Gráice 🚀 Probe" } }) { id name } }`, nil)
	if err != nil {
		return fmt.Errorf("unicode read: %w", err)
	}
	if n := listLen(data["products"]); n != 1 {
		return fmt.Errorf("unicode rows = %v, want 1", data["products"])
	}

	// Empty result set keeps list shape.
	data, err = h.MustData(`{ users(where: { email: { eq: "nobody@nowhere.io" } }) { id } }`, nil)
	if err != nil {
		return err
	}
	if rows, _ := data["users"].([]any); rows == nil || len(rows) != 0 {
		return fmt.Errorf("empty filter → %v, want empty list", data["users"])
	}

	// Malformed JSON body is a clean transport-level failure.
	status, raw, err := h.PostRaw("/api/v1/graphql", "{not json")
	if err != nil {
		return err
	}
	parsed := parseJSON(raw)
	if status < 400 && parsed["errors"] == nil {
		return fmt.Errorf("malformed body accepted: status=%d body=%s", status, raw)
	}

	// Unknown field produces an error naming it, not a crash.
	resp, err := h.GQL(`{ users { id nope_not_here } }`, nil)
	if err != nil {
		return err
	}
	if msg := firstError(resp); !strings.Contains(msg, "nope_not_here") {
		return fmt.Errorf("unknown-field error = %q", msg)
	}
	return nil
}
