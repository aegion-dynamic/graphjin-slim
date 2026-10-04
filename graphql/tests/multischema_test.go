package graphql_test

import (
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/graphql/v3"
)

func twoSchema(t *testing.T, cfg sdata.SchemaConfig) *sdata.DBSchema {
	t.Helper()
	cols := []sdata.DBColumn{
		{Schema: "public", Table: "users", Name: "id", Type: "bigint", PrimaryKey: true},
		{Schema: "private", Table: "users", Name: "id", Type: "bigint", PrimaryKey: true},
	}
	di := sdata.NewDBInfo("postgres", 150000, "public", "db", cols, nil, nil)
	s, err := sdata.NewDBSchemaWithConfig(di, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMultiSchemaFindAlias(t *testing.T) {
	s := twoSchema(t, sdata.SchemaConfig{})
	co, err := graphql.NewCompiler(s, graphql.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := co.Find("", "usersOfPrivate")
	if err != nil {
		t.Fatal(err)
	}
	if tbl.Schema != "private" || tbl.Name != "users" {
		t.Fatalf("got %+v", tbl)
	}
	tbl, err = co.Find("", "users")
	if err != nil || tbl.Schema != "public" {
		t.Fatalf("bare users should hit default public: %+v %v", tbl, err)
	}
	// default schema passed explicitly must not veto the alias
	tbl, err = co.Find("public", "usersOfPrivate")
	if err != nil || tbl.Schema != "private" {
		t.Fatalf("explicit default schema vetoed alias: %+v %v", tbl, err)
	}

	// camelCase path: raw camel alias survives ParseName
	co2, _ := graphql.NewCompiler(s, graphql.Config{EnableCamelcase: true})
	if got := co2.ParseName("usersOfPrivate"); got != "usersOfprivate" {
		t.Fatalf("ParseName = %q", got)
	}
	tbl, err = co2.Find("", co2.ParseName("usersOfPrivate"))
	if err != nil || tbl.Schema != "private" {
		t.Fatalf("camel alias failed: %+v %v", tbl, err)
	}

	// disallowed schema rejected
	sLocked := twoSchema(t, sdata.SchemaConfig{AllowedSchemas: []string{"public"}})
	co3, _ := graphql.NewCompiler(sLocked, graphql.Config{})
	if _, err := co3.Find("", "usersOfPrivate"); err == nil {
		t.Fatalf("expected not-allowed error")
	}
}
