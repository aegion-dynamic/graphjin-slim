package schema

import (
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/sdata"
)

func TestComputeDiff_CreateTable(t *testing.T) {
	current := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{}}
	expected := &sdata.DBInfo{
		Type: "postgres",
		Tables: []sdata.DBTable{{
			Name: "users",
			Columns: []sdata.DBColumn{
				{Name: "id", Type: "bigint", PrimaryKey: true, NotNull: true},
				{Name: "name", Type: "text", NotNull: true},
				{Name: "email", Type: "text"},
			},
		}},
	}
	ops := computeDiff(current, expected, DiffOptions{Destructive: false})
	found := false
	for _, op := range ops {
		if op.Type == "create_table" && op.Table == "users" {
			found = true
			if !strings.Contains(op.SQL, "CREATE TABLE") || !strings.Contains(op.SQL, `"id"`) {
				t.Errorf("unexpected create SQL: %s", op.SQL)
			}
		}
	}
	if !found {
		t.Error("expected create_table for users")
	}
}

func TestSupportsSchemaDDLBoundary(t *testing.T) {
	for _, dbType := range []string{"postgres", "postgresql", "sqlite", "Postgres", "POSTGRES", "sqlite"} {
		if !SupportsSchemaDDL(dbType) {
			t.Fatalf("SupportsSchemaDDL(%q) = false, want true", dbType)
		}
	}
	for _, dbType := range []string{"mysql", "mariadb", "mssql", "oracle", "snowflake", "bigquery", "redshift", "cassandra", "mongodb", "codesql", "graphjin"} {
		if SupportsSchemaDDL(dbType) {
			t.Fatalf("SupportsSchemaDDL(%q) = true, want false", dbType)
		}
	}
}

func TestComputeDiff_AddColumn(t *testing.T) {
	current := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{{Name: "users", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true}}}}}
	expected := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{{Name: "users", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true}, {Name: "email", Type: "text", NotNull: true}}}}}
	ops := computeDiff(current, expected, DiffOptions{Destructive: false})
	found := false
	for _, op := range ops {
		if op.Type == "add_column" && op.Column == "email" {
			found = true
			if !strings.Contains(op.SQL, "ADD COLUMN") {
				t.Errorf("expected ADD COLUMN, got %s", op.SQL)
			}
		}
	}
	if !found {
		t.Error("expected add_column for email")
	}
}

func TestComputeDiff_DropColumn(t *testing.T) {
	current := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{{Name: "users", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true}, {Name: "old", Type: "text"}}}}}
	expected := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{{Name: "users", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true}}}}}
	ops := computeDiff(current, expected, DiffOptions{Destructive: false})
	for _, op := range ops {
		if op.Type == "drop_column" {
			t.Error("should not drop without destructive")
		}
	}
	ops = computeDiff(current, expected, DiffOptions{Destructive: true})
	found := false
	for _, op := range ops {
		if op.Type == "drop_column" && op.Column == "old" && op.Danger {
			found = true
		}
	}
	if !found {
		t.Error("expected dangerous drop_column")
	}
}

func TestComputeDiff_DropTable(t *testing.T) {
	current := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{{Name: "old_table", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true}}}}}
	expected := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{}}
	ops := computeDiff(current, expected, DiffOptions{Destructive: true})
	found := false
	for _, op := range ops {
		if op.Type == "drop_table" && op.Table == "old_table" && op.Danger {
			found = true
		}
	}
	if !found {
		t.Error("expected dangerous drop_table")
	}
}

func TestPostgresDialect_MapType(t *testing.T) {
	d := &postgresDialect{}
	tests := []struct {
		in          string
		notNull, pk bool
		want        string
	}{
		{"bigint", false, true, "BIGSERIAL PRIMARY KEY"},
		{"integer", true, false, "INTEGER NOT NULL"},
		{"text", false, false, "TEXT"},
		{"boolean", true, false, "BOOLEAN NOT NULL"},
		{"timestamptz", false, false, "TIMESTAMPTZ"},
		{"jsonb", false, false, "JSONB"},
		{"uuid", false, true, "UUID PRIMARY KEY"},
	}
	for _, tc := range tests {
		if got := d.MapType(tc.in, tc.notNull, tc.pk); got != tc.want {
			t.Errorf("MapType(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSQLiteDialect_MapType(t *testing.T) {
	d := &sqliteDialect{}
	tests := []struct {
		in          string
		notNull, pk bool
		want        string
	}{
		{"bigint", false, true, "INTEGER PRIMARY KEY AUTOINCREMENT"},
		{"integer", true, false, "INTEGER NOT NULL"},
		{"text", false, false, "TEXT"},
		{"boolean", true, false, "INTEGER NOT NULL"},
		{"json", false, false, "TEXT"},
		{"timestamp", false, false, "TEXT"},
	}
	for _, tc := range tests {
		if got := d.MapType(tc.in, tc.notNull, tc.pk); got != tc.want {
			t.Errorf("MapType(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestPostgresDialect_CreateTable(t *testing.T) {
	d := &postgresDialect{}
	table := sdata.DBTable{Name: "posts", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true, NotNull: true}, {Name: "title", Type: "text", NotNull: true}, {Name: "user_id", Type: "bigint", NotNull: true, FKeyTable: "users", FKeyCol: "id"}}}
	sql := d.CreateTable(table)
	for _, want := range []string{"CREATE TABLE", `"posts"`, `"id"`, "BIGSERIAL PRIMARY KEY", "FOREIGN KEY", `REFERENCES "users"("id")`} {
		if !strings.Contains(sql, want) {
			t.Errorf("CreateTable missing %q in %s", want, sql)
		}
	}
}

func TestSQLiteDialect_CreateTable(t *testing.T) {
	d := &sqliteDialect{}
	table := sdata.DBTable{Name: "posts", Columns: []sdata.DBColumn{{Name: "id", Type: "integer", PrimaryKey: true}, {Name: "title", Type: "text"}}}
	sql := d.CreateTable(table)
	if !strings.Contains(sql, "CREATE TABLE") || !strings.Contains(sql, `"posts"`) {
		t.Errorf("unexpected sqlite CreateTable %s", sql)
	}
}

func TestQuoteIdentifier(t *testing.T) {
	for _, tc := range []struct {
		d    DDLDialect
		want string
	}{{&postgresDialect{}, `"users"`}, {&sqliteDialect{}, `"users"`}} {
		if got := tc.d.QuoteIdentifier("users"); got != tc.want {
			t.Errorf("%s Quote = %q want %q", tc.d.Name(), got, tc.want)
		}
	}
}

func TestGenerateDiffSQL(t *testing.T) {
	ops := []SchemaOperation{{Type: "create_table", Table: "users", SQL: "CREATE TABLE users ();"}, {Type: "add_column", Table: "users", Column: "email", SQL: "ALTER TABLE users ADD COLUMN email TEXT;"}}
	sqls := GenerateDiffSQL(ops)
	if len(sqls) != 2 || sqls[0] != "CREATE TABLE users ();" {
		t.Errorf("GenerateDiffSQL = %v", sqls)
	}
}

func TestComputeDiff_NoChanges(t *testing.T) {
	schema := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{{Name: "users", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true}, {Name: "name", Type: "text"}}}}}
	if ops := computeDiff(schema, schema, DiffOptions{}); len(ops) != 0 {
		t.Errorf("expected 0 ops, got %d", len(ops))
	}
}

func TestComputeDiff_DifferentDBTypes(t *testing.T) {
	current := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{}}
	expected := &sdata.DBInfo{Type: "sqlite", Tables: []sdata.DBTable{{Name: "users", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint"}}}}}
	ops := computeDiff(current, expected, DiffOptions{})
	if len(ops) == 0 || ops[0].Type != "create_table" {
		t.Errorf("expected create_table, got %v", ops)
	}
	if !strings.Contains(ops[0].SQL, `"users"`) {
		t.Errorf("expected quoted table, got %s", ops[0].SQL)
	}
}

func TestAddForeignKey(t *testing.T) {
	d := &postgresDialect{}
	col := sdata.DBColumn{Name: "user_id", Type: "bigint", FKeyTable: "users", FKeyCol: "id"}
	sql := d.AddForeignKey("posts", col)
	for _, want := range []string{"ALTER TABLE", "ADD CONSTRAINT", "FOREIGN KEY", `REFERENCES "users"("id")`} {
		if !strings.Contains(sql, want) {
			t.Errorf("AddForeignKey missing %q in %s", want, sql)
		}
	}
}

func TestCreateUniqueIndex(t *testing.T) {
	d := &postgresDialect{}
	sql := d.CreateUniqueIndex("users", sdata.DBColumn{Name: "email", Type: "text", UniqueKey: true})
	if !strings.Contains(sql, "CREATE UNIQUE INDEX") || !strings.Contains(sql, `ON "users"`) {
		t.Errorf("CreateUniqueIndex bad %s", sql)
	}
}

func TestCreateSearchIndex_Postgres(t *testing.T) {
	d := &postgresDialect{}
	sql := d.CreateSearchIndex("posts", sdata.DBColumn{Name: "content", Type: "text", FullText: true})
	if !strings.Contains(sql, "USING gin") || !strings.Contains(sql, "to_tsvector") {
		t.Errorf("CreateSearchIndex bad %s", sql)
	}
}

func TestCreateIndex_Postgres(t *testing.T) {
	d := &postgresDialect{}
	sql := d.CreateIndex("users", sdata.DBColumn{Name: "email", Type: "text", Index: true})
	if !strings.Contains(sql, "CREATE INDEX") || !strings.Contains(sql, `"idx_users_email"`) {
		t.Errorf("CreateIndex bad %s", sql)
	}
}

func TestCreateIndex_SQLite(t *testing.T) {
	d := &sqliteDialect{}
	sql := d.CreateIndex("users", sdata.DBColumn{Name: "email", Type: "text", Index: true})
	if !strings.Contains(sql, "CREATE INDEX") || !strings.Contains(sql, `"idx_users_email"`) {
		t.Errorf("CreateIndex sqlite bad %s", sql)
	}
}

func TestPostgresDialect_MapDefault(t *testing.T) {
	d := &postgresDialect{}
	for _, tc := range []struct{ in, want string }{{"'active'", "'active'"}, {"now()", "now()"}, {"0", "0"}} {
		if got := d.MapDefault(tc.in); got != tc.want {
			t.Errorf("MapDefault %q = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestCreateTable_WithDefault_Postgres(t *testing.T) {
	d := &postgresDialect{}
	table := sdata.DBTable{Name: "orders", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true, NotNull: true}, {Name: "status", Type: "text", NotNull: true, Default: "'pending'"}, {Name: "created_at", Type: "timestamp", Default: "now()"}}}
	sql := d.CreateTable(table)
	if !strings.Contains(sql, "DEFAULT 'pending'") || !strings.Contains(sql, "DEFAULT now()") {
		t.Errorf("CreateTable with default bad %s", sql)
	}
}

func TestAddColumn_WithDefault_Postgres(t *testing.T) {
	d := &postgresDialect{}
	sql := d.AddColumn("orders", sdata.DBColumn{Name: "status", Type: "text", NotNull: true, Default: "'new'"})
	if !strings.Contains(sql, "ADD COLUMN") || !strings.Contains(sql, "DEFAULT 'new'") {
		t.Errorf("AddColumn bad %s", sql)
	}
}

func TestAddForeignKey_WithCascade(t *testing.T) {
	d := &postgresDialect{}
	sql := d.AddForeignKey("posts", sdata.DBColumn{Name: "user_id", Type: "bigint", FKeyTable: "users", FKeyCol: "id", FKOnDelete: "CASCADE"})
	if !strings.Contains(sql, "ON DELETE CASCADE") {
		t.Errorf("FK cascade missing %s", sql)
	}
}

func TestAddForeignKey_WithSetNull(t *testing.T) {
	d := &postgresDialect{}
	sql := d.AddForeignKey("products", sdata.DBColumn{Name: "category_id", Type: "bigint", FKeyTable: "categories", FKeyCol: "id", FKOnDelete: "SET NULL", FKOnUpdate: "CASCADE"})
	if !strings.Contains(sql, "ON DELETE SET NULL") || !strings.Contains(sql, "ON UPDATE CASCADE") {
		t.Errorf("FK set null bad %s", sql)
	}
}

func TestPostgresDialect_NewTypes(t *testing.T) {
	d := &postgresDialect{}
	for _, tc := range []struct{ in, want string }{{"money", "MONEY"}, {"xml", "XML"}, {"serial", "SERIAL"}, {"bigserial", "BIGSERIAL"}, {"interval", "INTERVAL"}} {
		if got := d.MapType(tc.in, false, false); got != tc.want {
			t.Errorf("MapType %q = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSQLiteDialect_NewTypes(t *testing.T) {
	d := &sqliteDialect{}
	for _, tc := range []struct{ in, want string }{{"money", "REAL"}, {"xml", "TEXT"}, {"serial", "INTEGER"}, {"bigserial", "INTEGER"}, {"interval", "TEXT"}} {
		if got := d.MapType(tc.in, false, false); got != tc.want {
			t.Errorf("SQLite MapType %q = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseTypeWithSize(t *testing.T) {
	for _, tc := range []struct{ in, base, size string }{{"varchar255", "varchar", "255"}, {"char36", "char", "36"}, {"decimal10_2", "decimal", "10,2"}, {"varchar(100)", "varchar", "100"}, {"text", "", ""}} {
		b, s := parseTypeWithSize(tc.in)
		if b != tc.base || s != tc.size {
			t.Errorf("parseTypeWithSize %q = (%q,%q) want (%q,%q)", tc.in, b, s, tc.base, tc.size)
		}
	}
}

func TestTypeAliases_Varchar_Postgres(t *testing.T) {
	d := &postgresDialect{}
	for _, tc := range []struct{ in, want string }{{"varchar255", "VARCHAR(255)"}, {"varchar100", "VARCHAR(100)"}} {
		if got := d.MapType(tc.in, false, false); got != tc.want {
			t.Errorf("MapType %q = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestComputeDiff_WithDefault(t *testing.T) {
	current := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{}}
	expected := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{{Name: "orders", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true, NotNull: true}, {Name: "status", Type: "text", NotNull: true, Default: "'pending'"}}}}}
	ops := computeDiff(current, expected, DiffOptions{})
	found := false
	for _, op := range ops {
		if op.Type == "create_table" && strings.Contains(op.SQL, "DEFAULT 'pending'") {
			found = true
		}
	}
	if !found {
		t.Error("expected DEFAULT in create")
	}
}

func TestComputeDiff_WithIndex(t *testing.T) {
	current := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{}}
	expected := &sdata.DBInfo{Type: "postgres", Tables: []sdata.DBTable{{Name: "users", Columns: []sdata.DBColumn{{Name: "id", Type: "bigint", PrimaryKey: true, NotNull: true}, {Name: "email", Type: "text", Index: true}}}}}
	ops := computeDiff(current, expected, DiffOptions{})
	hasIdx := false
	for _, op := range ops {
		if op.Type == "add_index" && strings.Contains(op.SQL, "CREATE INDEX") {
			hasIdx = true
		}
	}
	if !hasIdx {
		t.Error("expected add_index")
	}
}
