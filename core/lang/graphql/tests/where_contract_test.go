package graphql_test

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/lang/graphql"
	"strings"
	"testing"
)

func TestWhereNotEqualNullNamesIsNullRepair(t *testing.T) {
	compiler, err := graphql.NewCompiler(dbs, graphql.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = compiler.Compile([]byte(`query { users(where: { email: { neq: null } }) { id } }`), nil, "")
	if err == nil {
		t.Fatal("expected neq: null to fail")
	}
	if !strings.Contains(err.Error(), "use `is_null: false`") {
		t.Fatalf("missing executable repair: %v", err)
	}
}

func TestWhereAggregateOperandNamesTwoStepRepair(t *testing.T) {
	compiler, err := graphql.NewCompiler(dbs, graphql.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = compiler.Compile([]byte(`query { users(where: { id: { gte: { max: id } } }) { id } }`), nil, "")
	if err == nil {
		t.Fatal("expected embedded aggregate operand to fail")
	}
	for _, want := range []string{"first query `max_id`", "returned literal"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %q in repair: %v", want, err)
		}
	}
}

func TestWhereLiteralComparisonStillCompiles(t *testing.T) {
	compiler, err := graphql.NewCompiler(dbs, graphql.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compiler.Compile([]byte(`query { users(where: { id: { gte: 10 } }) { id } }`), nil, ""); err != nil {
		t.Fatalf("literal comparison regressed: %v", err)
	}
}
