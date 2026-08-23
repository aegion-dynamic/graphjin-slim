package graphql_test

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/lang/graphql"
	"strings"
	"testing"
)

// The `<agg>(column: <col>)` spelling is what models naturally write for the
// prefix form; before it was accepted, `max_renewal_date: max(column:
// renewal_date)` died as `unknown argument 'column'` with no hint, and a
// benchmark run burned whole step budgets on it. These tests pin that the form
// compiles to the exact shape the prefix form produces.

func TestColumnArgAggregate(t *testing.T) {
	qc, _ := graphql.NewCompiler(dbs, graphql.Config{})

	for _, form := range []string{
		`max_price: max(column: price)`,
		`max_price: max(column: "price")`,
	} {
		q, err := qc.Compile([]byte(`query { products { id `+form+` } }`), nil, "")
		if err != nil {
			t.Fatalf("compile %q: %v", form, err)
		}
		found := false
		for _, f := range q.Selects[0].Fields {
			if f.FieldName != "max_price" {
				continue
			}
			found = true
			if f.Type != qcode.FieldTypeFunc {
				t.Errorf("%q: Type = %v, want FieldTypeFunc", form, f.Type)
			}
			if len(f.Args) != 1 || f.Args[0].Type != qcode.ArgTypeCol {
				t.Fatalf("%q: Args = %+v, want one ArgTypeCol", form, f.Args)
			}
			if f.Args[0].Col.Name != "price" {
				t.Errorf("%q: Col.Name = %q, want price", form, f.Args[0].Col.Name)
			}
			if !strings.EqualFold(f.Func.Name, "max") {
				t.Errorf("%q: Func.Name = %q, want max", form, f.Func.Name)
			}
		}
		if !found {
			t.Fatalf("%q: max_price field not found", form)
		}
	}
}

func TestColumnArgNonAggregateStillErrors(t *testing.T) {
	qc, _ := graphql.NewCompiler(dbs, graphql.Config{})

	// A non-aggregate function name keeps its historical failure: the column
	// spelling is deliberately confined to genuine aggregates.
	if _, err := qc.Compile([]byte(`query { products { id lower(column: name) } }`), nil, ""); err == nil {
		t.Fatal("lower(column:) must not compile")
	}

	// A nonexistent column names itself in the error.
	_, err := qc.Compile([]byte(`query { products { id max(column: nope) } }`), nil, "")
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("unknown column must error naming it, got: %v", err)
	}

	// DisableAgg blocks the new spelling like every other aggregate form.
	blocked, _ := graphql.NewCompiler(dbs, graphql.Config{DisableAgg: true})
	_, err = blocked.Compile([]byte(`query { products { id max(column: price) } }`), nil, "")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("DisableAgg must reject the column form, got: %v", err)
	}
}
