package scenarios

import (
	"fmt"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "values_hostile_strings", Fn: valuesHostileStrings})
	register(Scenario{Name: "values_type_roundtrip", Fn: valuesTypeRoundtrip})
}

// valuesHostileStrings feeds classic injection payloads through inserts
// and filters and requires literal storage plus an intact table.
func valuesHostileStrings(h *harness.H) error {
	const sqli = `Robert'); DROP TABLE products;--`
	data, err := h.MustData(
		fmt.Sprintf(`mutation { products(insert: { name: %q, price: 1.0 }) { id name } }`, sqli), nil)
	if err != nil {
		return fmt.Errorf("sqli insert rejected outright: %w", err)
	}
	if n, _ := firstRowField(data, "products", "name"); n != sqli {
		return fmt.Errorf("stored = %q, want verbatim payload", n)
	}

	// The table must still be fully functional.
	data, err = h.MustData(`{ products { count_id } }`, nil)
	if err != nil {
		return fmt.Errorf("products table damaged after hostile insert: %w", err)
	}
	if c := data["products"].([]any)[0].(map[string]any)["count_id"]; c != float64(5) {
		return fmt.Errorf("product count = %v, want 5", c)
	}

	// Filtering BY the hostile string uses parameter binding too.
	data, err = h.MustData(
		fmt.Sprintf(`{ products(where: { name: { eq: %q } }) { id } }`, sqli), nil)
	if err != nil {
		return err
	}
	if n := listLen(data["products"]); n != 1 {
		return fmt.Errorf("hostile filter matched %d rows, want 1", n)
	}
	return nil
}

// valuesTypeRoundtrip pushes boundary values of every scalar type.
func valuesTypeRoundtrip(h *harness.H) error {
	// Int32 boundaries (GraphQL Int is 32-bit by spec).
	big := int64(2147483647)
	neg := int64(-2147483648)

	for _, tc := range []struct {
		name  string
		email string
		age   int64
	}{
		{"max-int32", "maxint@x.io", big},
		{"min-int32", "minint@x.io", neg},
	} {
		if _, err := h.MustData(fmt.Sprintf(
			`mutation { users(insert: { full_name: "%s", email: "%s", age: %d }) { id age } }`,
			tc.name, tc.email, tc.age), nil); err != nil {
			return fmt.Errorf("%s insert: %w", tc.name, err)
		}
		data, err := h.MustData(fmt.Sprintf(
			`{ users(where: { email: { eq: "%s" } }) { age } }`, tc.email), nil)
		if err != nil {
			return err
		}
		got, _ := firstRowField(data, "users", "age")
		if got != float64(tc.age) {
			return fmt.Errorf("%s age round-trip = %v, want %d", tc.name, got, tc.age)
		}
	}

	// Float precision survives the JSON envelope.
	if _, err := h.MustData(
		`mutation { products(insert: { name: "Precision Probe", price: 0.1 }) { id } }`, nil); err != nil {
		return err
	}
	data, err := h.MustData(`{ products(where: { name: { eq: "Precision Probe" } }) { price } }`, nil)
	if err != nil {
		return err
	}
	p, _ := firstRowField(data, "products", "price")
	pf, _ := p.(float64)
	if pf < 0.0999 || pf > 0.1001 {
		return fmt.Errorf("0.1 round-trip = %v", p)
	}

	// Unicode beyond BMP + combining marks in a WHERE value.
	const exotic = "日本語 🚀 café"
	if _, err := h.MustData(fmt.Sprintf(
		`mutation { tags(insert: { label: %q }) { id label } }`, exotic), nil); err != nil {
		return fmt.Errorf("exotic tag insert: %w", err)
	}
	data, err = h.MustData(fmt.Sprintf(
		`{ tags(where: { label: { eq: %q } }) { label } }`, exotic), nil)
	if err != nil {
		return err
	}
	lbl, err := firstRowField(data, "tags", "label")
	if err != nil || lbl != exotic {
		return fmt.Errorf("exotic label = %v (%v)", lbl, err)
	}

	// Empty string vs NULL are different facts; both must survive.
	if _, err := h.MustData(
		`mutation { users(insert: { full_name: "", email: "emptyname@x.io" }) { id } }`, nil); err != nil {
		return err
	}
	data, err = h.MustData(`{ users(where: { email: { eq: "emptyname@x.io" } }) { full_name } }`, nil)
	if err != nil {
		return err
	}
	fn, _ := firstRowField(data, "users", "full_name")
	if s, _ := fn.(string); strings.TrimSpace(s) != "" || fn == nil {
		return fmt.Errorf("empty full_name = %#v, want \"\"", fn)
	}
	return nil
}
