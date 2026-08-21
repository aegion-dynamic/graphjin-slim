package scenarios

import (
	"fmt"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "fk_depth", Fn: func(h *harness.H) error {
		return fkDepth(h, h.Budgets.MaxDepth)
	}})
	register(Scenario{Name: "fk_recursion_guard", Fn: func(h *harness.H) error {
		return fkRecursionGuard(h, h.Budgets.MaxDepth)
	}})
}

// fkDepth walks a Chain(MaxDepth) schema at increasing nesting depths and
// asserts the exact stitched row surfaces at every level.
func fkDepth(h *harness.H, maxDepth int) error {
	for _, depth := range []int{2, 4, 8, maxDepth} {
		if depth > maxDepth {
			continue
		}
		q := chainQuery(depth)
		data, err := h.MustData(q, nil)
		if err != nil {
			return fmt.Errorf("depth %d: %w", depth, err)
		}
		got, err := descendChain(data, depth)
		if err != nil {
			return fmt.Errorf("depth %d: %w", depth, err)
		}
		want := harness.ChainTable(depth-1) + " row"
		if got != want {
			return fmt.Errorf("depth %d: end name = %v, want %q", depth, got, want)
		}
	}
	return nil
}

// chainQuery builds `{ ta { tb { … { last { name } } } } }`.
func chainQuery(depth int) string {
	var b strings.Builder
	b.WriteString("{ ")
	for i := 0; i < depth; i++ {
		b.WriteString(harness.ChainTable(i) + " ")
		if i < depth-1 {
			b.WriteString("{ ")
		}
	}
	b.WriteString("{ name }")
	for i := 0; i < depth-1; i++ {
		b.WriteString(" }")
	}
	b.WriteString(" }")
	return b.String()
}

// fkRecursionGuard proves that querying past the seeded schema fails fast
// with a GraphQL error instead of hanging or panicking.
func fkRecursionGuard(h *harness.H, seeded int) error {
	q := chainQuery(seeded + 10) // references tables beyond the schema
	resp, err := h.GQL(q, nil)
	if err != nil {
		return fmt.Errorf("over-deep query transport error: %w", err)
	}
	errs, ok := resp["errors"]
	if !ok || errs == nil {
		return fmt.Errorf("query beyond the schema must fail, got data: %v", resp["data"])
	}
	eb, _ := errs.([]any)
	if len(eb) == 0 {
		return fmt.Errorf("expected non-empty errors array")
	}
	return nil
}

// descendChain walks data through depth nested list-wrapped levels,
// returning the innermost row's name.
func descendChain(data map[string]any, depth int) (string, error) {
	cur := any(data)
	for i := 0; i < depth; i++ {
		key := harness.ChainTable(i)
		level, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("level %d: %T is not an object", i, cur)
		}
		v, ok := level[key]
		if !ok {
			return "", fmt.Errorf("level %d: missing %q", i, key)
		}
		arr, ok := v.([]any)
		if !ok {
			return "", fmt.Errorf("level %d (%s): unexpected %T", i, key, v)
		}
		if len(arr) != 1 {
			return "", fmt.Errorf("level %d (%s): want exactly 1 row, got %d", i, key, len(arr))
		}
		cur = arr[0]
		if i == depth-1 {
			row := cur.(map[string]any)
			name, _ := row["name"].(string)
			return name, nil
		}
	}
	return "", fmt.Errorf("unreachable")
}
