package schema_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

func TestMultiSchemaIntroAlias(t *testing.T) {
	cols := []sdata.DBColumn{
		{Schema: "public", Table: "users", Name: "id", Type: "bigint", PrimaryKey: true},
		{Schema: "private", Table: "users", Name: "id", Type: "bigint", PrimaryKey: true},
	}
	di := sdata.NewDBInfo("postgres", 150000, "public", "db", cols, nil, nil)
	s, err := sdata.NewDBSchemaWithConfig(di, nil, sdata.SchemaConfig{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := BuildIntrospection(IntroOptions{Schemas: []*sdata.DBSchema{s}})
	if err != nil {
		t.Fatal(err)
	}
	var intro IntroResult
	if err := json.Unmarshal(result, &intro); err != nil {
		t.Fatal(err)
	}
	foundBare, foundAlias := false, false
	for _, typ := range intro.Schema.Types {
		if typ.Name == "Query" {
			for _, f := range typ.Fields {
				if f.Name == "users" {
					foundBare = true
				}
				if f.Name == "usersOfPrivate" {
					foundAlias = true
				}
			}
		}
	}
	if !foundBare || !foundAlias {
		t.Fatalf("bare=%v alias=%v", foundBare, foundAlias)
	}

	// allow-list hides private
	s2, _ := sdata.NewDBSchemaWithConfig(di, nil, sdata.SchemaConfig{AllowedSchemas: []string{"public"}})
	result2, err := BuildIntrospection(IntroOptions{Schemas: []*sdata.DBSchema{s2}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result2), "usersOfPrivate") {
		t.Fatalf("disallowed alias leaked into introspection")
	}
}
