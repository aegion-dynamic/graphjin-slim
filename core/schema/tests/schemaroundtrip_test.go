package schema_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
	"github.com/aegion-dynamic/graphjin-slim/graphql/v3"
)

// TestWriteSchemaTypeRoundTrip pins exact save/load of column types,
// especially schema-qualified Postgres enums. WriteSchema used to silently
// truncate "application.event_status" to "application", so any engine booted
// from the discovery cache generated CASTs against a nonexistent type.
func TestWriteSchemaTypeRoundTrip(t *testing.T) {
	types := []string{
		"application.event_status",
		"accesscontrol.permission_kind_enum",
		"my_enum",
		"character varying(255)",
		"numeric(12,2)",
		"text",
		"text[]",
		"timestamp without time zone",
		"bigint",
		"boolean",
		"uuid",
		"jsonb",
	}
	cols := make([]sdata.DBColumn, len(types))
	for i, typ := range types {
		cols[i] = sdata.DBColumn{
			ID:      int32(i),
			Schema:  "public",
			Table:   "probes",
			Name:    "c" + strings.ReplaceAll(typ, " ", "_"),
			Type:    typ,
			Array:   strings.HasSuffix(typ, "[]"),
			NotNull: true,
		}
	}
	// Column names must be valid SDL names; use positional names.
	for i := range cols {
		cols[i].Name = "c" + string(rune('a'+i))
	}

	di := sdata.NewDBInfo("postgres", 150000, "public", "db", cols, nil, nil)

	var buf bytes.Buffer
	if err := schema.WriteSchema(di, &buf); err != nil {
		t.Fatalf("WriteSchema: %v", err)
	}
	if n := strings.Count(buf.String(), "@dbtype"); n == 0 {
		t.Fatalf("expected @dbtype directives for unlexable types, got none")
	}

	ds, err := graphql.ParseSchema(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	got := map[string]sdata.DBColumn{}
	for _, c := range ds.Columns {
		got[c.Name] = c
	}
	for i, typ := range types {
		name := "c" + string(rune('a'+i))
		c, ok := got[name]
		if !ok {
			t.Fatalf("column %s missing after reload", name)
		}
		if c.Type != typ {
			t.Errorf("column %s type = %q, want %q", name, c.Type, typ)
		}
		if c.Array != strings.HasSuffix(typ, "[]") {
			t.Errorf("column %s array = %v, want %v", name, c.Array, !c.Array)
		}
	}
}
