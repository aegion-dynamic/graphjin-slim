package tests

// Regression test for nested list inserts that wrote the first request's
// child rows for every later request: the rows of a nested list bound to a
// variable were compiled into the statement, and the statement is compiled
// once in production and reused.
//
// Needs a Postgres with the dialect corpus schema and seed; the dialect
// up.sh runs it after seeding. Skipped without GJ_TEST_PG.

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/engine"
	_ "github.com/aegion-dynamic/graphjin-slim/graphql/v3"
	postgresmod "github.com/aegion-dynamic/graphjin-slim/postgres/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPgNestedListVarsBoundPerRequest(t *testing.T) {
	dsn := os.Getenv("GJ_TEST_PG")
	if dsn == "" {
		t.Skip("GJ_TEST_PG not set")
	}
	ctx := context.Background()
	db, err := sql.Open(postgresmod.DriverPostgres, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("postgres not reachable: %v", err)
	}

	gj, err := engine.NewGraphJin(&engine.Config{Production: true}, db,
		engine.OptionSetFS(engine.NewOsFS(t.TempDir())))
	if err != nil {
		t.Fatal(err)
	}
	defer gj.Close()

	run := func(name, query, vars string) string {
		t.Helper()
		if err := gj.SaveQuery(&engine.SavedQueryDetails{Name: name, Query: query}); err != nil {
			t.Fatal(err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback() //nolint:errcheck
		res, err := gj.GraphQLByName(ctx, name, json.RawMessage(vars), &engine.RequestConfig{Tx: tx})
		if err != nil {
			t.Fatalf("%s %s: %v", name, vars, err)
		}
		return string(res.Data)
	}

	nested := `mutation NestedLists($slug: String!, $regs: [registrationsInput!]!, $ff: [form_fieldsInput!]!) {
  events(insert: { slug: $slug, title: "Nested lists", host_user_id: "11111111-1111-1111-1111-000000000001",
    status: "draft", format: "in_person", participation_type: "open", registrations: $regs, form_fields: $ff }) {
    slug
    registrations { name email phone }
    form_fields { label }
  }
}`
	steps := []struct{ vars, want, not string }{
		// Empty lists first: the shape of a later request must not come
		// from this one.
		{`{"slug": "nl-0", "regs": [], "ff": []}`, `"registrations": []`, ``},
		{`{"slug": "nl-1", "regs": [{"name": "First", "email": "first@example.org"}], "ff": []}`, `"First"`, ``},
		{`{"slug": "nl-2", "regs": [{"name": "Second", "email": "second@example.org"}, {"name": "Third", "email": "third@example.org", "phone": "555"}], "ff": [{"label": "L", "field_type": "short_text", "position": 1}]}`, `"phone": "555"`, `First`},
	}
	for _, s := range steps {
		got := run("NestedLists", nested, s.vars)
		if !strings.Contains(got, s.want) || (s.not != "" && strings.Contains(got, s.not)) {
			t.Fatalf("vars %s: got %s, want %q and not %q", s.vars, got, s.want, s.not)
		}
	}

	multiRoot := `mutation TwoRoots($a: form_fieldsInput!, $b: form_fieldsInput!) {
  a: form_fields(insert: $a) { label }
  b: form_fields(insert: $b) { label }
}`
	for _, label := range []string{"one", "two"} {
		vars := `{"a": {"event_id": "44444444-4444-4444-4444-000000000001", "label": "a-` + label + `", "field_type": "short_text", "position": 1},
		          "b": {"event_id": "44444444-4444-4444-4444-000000000001", "label": "b-` + label + `", "field_type": "short_text", "position": 2}}`
		got := run("TwoRoots", multiRoot, vars)
		if !strings.Contains(got, `"a-`+label+`"`) || !strings.Contains(got, `"b-`+label+`"`) {
			t.Fatalf("vars %s: got %s", vars, got)
		}
	}
}
