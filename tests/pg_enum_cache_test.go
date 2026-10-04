package tests

// Regression test for the discovery-cache enum corruption: WriteSchema used
// to truncate "application.event_status" to "application", so an engine
// booted from the DDL cache failed every write touching an enum column with
// `type "application" does not exist`.
//
// Needs a Postgres with the dialect corpus schema (enums included); the
// dialect up.sh runs it after seeding. Skipped without GJ_TEST_PG.

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/engine"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/introspection"
	schemapkg "github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
	postgresmod "github.com/aegion-dynamic/graphjin-slim/postgres/v3"
	_ "github.com/aegion-dynamic/graphjin-slim/graphql/v3" // language registration for ParseSchemaSDL
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPgEnumDiscoveryCacheRoundTrip(t *testing.T) {
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

	live, err := introspection.GetDBInfo(ctx, db, "postgres", nil)
	if err != nil {
		t.Fatal(err)
	}
	var enumCols int
	for _, tb := range live.Tables {
		for _, c := range tb.Columns {
			if strings.Contains(c.Type, ".") {
				enumCols++
			}
		}
	}
	if enumCols == 0 {
		t.Fatal("fixture has no schema-qualified column types; cannot exercise the bug")
	}

	var buf bytes.Buffer
	if err := schemapkg.WriteSchema(live, &buf); err != nil {
		t.Fatalf("WriteSchema: %v", err)
	}
	cached, err := engine.ParseSchemaSDL(buf.Bytes(), "postgres", nil)
	if err != nil {
		t.Fatalf("ParseSchemaSDL: %v", err)
	}
	want := map[string]string{}
	for _, tb := range live.Tables {
		for _, c := range tb.Columns {
			want[tb.Schema+"."+tb.Name+"."+c.Name] = c.Type
		}
	}
	for _, tb := range cached.Tables {
		for _, c := range tb.Columns {
			if want[tb.Schema+"."+tb.Name+"."+c.Name] != c.Type {
				t.Fatalf("type lost in save/load: %v.%v.%v live=%q cached=%q",
					tb.Schema, tb.Name, c.Name, want[tb.Schema+"."+tb.Name+"."+c.Name], c.Type)
			}
		}
	}

	// Boot an engine purely from the DDL cache file and write an enum
	// column: this exact query failed with type "application" does not exist.
	dir := t.TempDir()
	p := filepath.Join(dir, ".graphjin", "schema-ddl", "default.ddl")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	gj, err := engine.NewGraphJinWithFS(
		&engine.Config{DisableAllowList: true},
		db,
		engine.NewOsFS(dir),
		engine.OptionSetRuntimeSchemaCacheFirst(true),
		engine.OptionSetRuntimeSchemaCacheRequired(true),
	)
	if err != nil {
		t.Fatalf("cache-only startup: %v", err)
	}
	defer gj.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	res, err := gj.GraphQL(ctx,
		`mutation { events(update: {status: "draft"}, where: {slug: {eq: "grid-scale-storage-summit"}}) { slug status } }`,
		nil, &engine.RequestConfig{Tx: tx})
	if err != nil {
		t.Fatalf("enum write on cache-booted engine: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("enum write errors: %v", res.Errors[0].Message)
	}
	if !strings.Contains(string(res.Data), `"status": "draft"`) {
		t.Fatalf("unexpected data: %s", res.Data)
	}
}
