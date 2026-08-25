package scenarios

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

// The differential battery runs against BOTH variants. Dev reaches the
// engine through ad-hoc GraphQL compilation; prod goes through saved
// queries over REST (allow-listed). After the suite, the test harness
// requires both variants to agree on every key byte-for-byte after JSON
// canonicalization — one oracle for the whole compile path split.

func init() {
	register(Scenario{
		Name:     "differential",
		Variants: []string{"dev", "prod"},
		Fn:       differential,
		Seeds: map[string]harness.SeedQuery{
			"diffUser":      {Query: `query diffUser($id: ID) { users(id: $id, orderBy: { id: asc }) { id full_name email age } }`},
			"diffAllUsers":  {Query: `query diffAllUsers { users(orderBy: { id: asc }) { id full_name email } }`},
			"diffProducts":  {Query: `query diffProducts { products(orderBy: { id: asc }) { id name price } }`},
			"diffUserItems": {Query: `query diffUserItems($id: ID) { users(id: $id) { full_name orders(orderBy: { id: asc }) { status total items { qty product { name } } } } }`},
		},
	})
}

func canonical(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func record(h *harness.H, key string, payload any) {
	harness.RecordDiff(key, h.Variant(), canonical(payload))
}

func differential(h *harness.H) error {
	if h.Prod {
		return differentialProd(h)
	}
	return differentialDev(h)
}

func differentialDev(h *harness.H) error {
	data, err := h.MustData(`{ users(id: 1) { id full_name email age } }`, nil)
	if err != nil {
		return err
	}
	record(h, "user1", data["users"])

	data, err = h.MustData(`{ users(orderBy: { id: asc }) { id full_name email } }`, nil)
	if err != nil {
		return err
	}
	record(h, "allusers", data["users"])

	data, err = h.MustData(`{ products(orderBy: { id: asc }) { id name price } }`, nil)
	if err != nil {
		return err
	}
	record(h, "products", data["products"])

	data, err = h.MustData(
		`{ users(id: 1) { full_name orders(orderBy: { id: asc }) { status total items { qty product { name } } } } }`, nil)
	if err != nil {
		return err
	}
	record(h, "user_items", data["users"])
	return nil
}

func differentialProd(h *harness.H) error {
	code, _, raw, err := h.Rest("GET", "diffUser", urlVals("id", "1"), nil)
	if err != nil || code != 200 {
		return fmt.Errorf("diffUser: code=%d err=%v raw=%s", code, err, raw)
	}
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	payload, _ := harness.Walk(body, "data.users")
	record(h, "user1", payload)

	for _, tc := range []struct {
		key, name string
		q         url.Values
	}{
		{"allusers", "diffAllUsers", nil},
		{"products", "diffProducts", nil},
		{"user_items", "diffUserItems", urlVals("id", "1")},
	} {
		code, _, raw, err := h.Rest("GET", tc.name, tc.q, nil)
		if err != nil || code != 200 {
			return fmt.Errorf("%s: code=%d err=%v raw=%s", tc.name, code, err, raw)
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		rootField := map[string]string{
			"diffAllUsers":  "users",
			"diffProducts":  "products",
			"diffUserItems": "users",
		}[tc.name]
		p, _ := harness.Walk(m, "data."+rootField)
		record(h, tc.key, p)
	}
	return nil
}
