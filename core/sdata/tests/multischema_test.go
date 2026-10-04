package sdata

import (
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

func twoSchemaDBInfo() *sdata.DBInfo {
	cols := []sdata.DBColumn{
		{Schema: "public", Table: "users", Name: "id", Type: "bigint", PrimaryKey: true},
		{Schema: "private", Table: "users", Name: "id", Type: "bigint", PrimaryKey: true},
		{Schema: "public", Table: "proofs", Name: "id", Type: "bigint", PrimaryKey: true},
	}
	return sdata.NewDBInfo("postgres", 150000, "public", "db", cols, nil, nil)
}

func TestMultiSchemaSeparatorDefault(t *testing.T) {
	di := twoSchemaDBInfo()
	s, err := sdata.NewDBSchemaWithConfig(di, nil, sdata.SchemaConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.GetCrossSchemaSeparator(); got != "Of" {
		t.Fatalf("default separator = %q, want Of", got)
	}
	if got := s.DefaultSchema(); got != "public" {
		t.Fatalf("default schema = %q", got)
	}
	if !s.IsAllowedSchema("private") {
		t.Fatalf("empty allow-list should allow all")
	}
	if alias := s.CrossSchemaAlias("users", "public"); alias != "users" {
		t.Fatalf("default alias = %q, want bare users", alias)
	}
	if alias := s.CrossSchemaAlias("users", "private"); alias != "usersOfPrivate" {
		t.Fatalf("alias = %q, want usersOfPrivate", alias)
	}
	tbl, ok := s.FindByAlias("usersOfPrivate")
	if !ok || tbl.Schema != "private" || tbl.Name != "users" {
		t.Fatalf("FindByAlias usersOfPrivate = %+v,%v", tbl, ok)
	}
	if _, ok := s.FindByAlias("proofs"); ok {
		// proofs exists as a real table; must resolve to public.proofs, not split on "of"
		if tbl, _ := s.FindByAlias("proofs"); tbl.Name != "proofs" {
			t.Fatalf("proofs misresolved: %+v", tbl)
		}
	}
	if _, ok := s.FindByAlias("nosuchOfPrivate"); ok {
		t.Fatalf("nosuchOfPrivate should not resolve")
	}

	// allow-list filtering
	s2, err := sdata.NewDBSchemaWithConfig(twoSchemaDBInfo(), nil, sdata.SchemaConfig{AllowedSchemas: []string{"public"}})
	if err != nil {
		t.Fatal(err)
	}
	if !s2.IsAllowedSchema("public") || s2.IsAllowedSchema("private") {
		t.Fatalf("allow-list not enforced")
	}

	// custom separator
	s3, err := sdata.NewDBSchemaWithConfig(twoSchemaDBInfo(), nil, sdata.SchemaConfig{CrossSchemaSeparator: "__"})
	if err != nil {
		t.Fatal(err)
	}
	if alias := s3.CrossSchemaAlias("users", "private"); alias != "users__Private" {
		t.Fatalf("custom alias = %q", alias)
	}
	if tbl, ok := s3.FindByAlias("users__Private"); !ok || tbl.Schema != "private" {
		t.Fatalf("custom sep FindByAlias failed: %+v,%v", tbl, ok)
	}
	tn, sn := s3.ParseCrossSchemaTableName("users")
	if tn != "users" || sn != "public" {
		t.Fatalf("parse bare = %q.%q", sn, tn)
	}
	if got := s.RealTableName("usersOfPrivate"); got != "users" {
		t.Fatalf("RealTableName alias = %q", got)
	}
	if got := s.RealTableName("users"); got != "users" {
		t.Fatalf("RealTableName bare = %q", got)
	}
	if got := s.RealTableName("proofs"); got != "proofs" {
		t.Fatalf("RealTableName proofs = %q", got)
	}
}
