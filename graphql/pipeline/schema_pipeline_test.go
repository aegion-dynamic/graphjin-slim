package pipeline_test

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/engine"
	schemapkg "github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/graphql/v3"
)

// helper mirroring engine.writeSchema
var writeSchema = func(s *sdata.DBInfo, buf *bytes.Buffer) error {
	return schemapkg.WriteSchema(s, buf)
}

func TestParseSchemaWithDatabase(t *testing.T) {
	schema := []byte(`
# dbinfo:postgres,140000,public

type users {
	id:	Bigint!	@id
	name:	CharacterVarying!
}

type events @database(name: analytics) {
	id:	Bigint!	@id
	event_type:	CharacterVarying!
}

type audit_logs @database(name: logs) {
	id:	Bigint!	@id
	action:	CharacterVarying!
}
`)

	ds, err := graphql.ParseSchema(schema)
	if err != nil {
		t.Fatal(err)
	}

	// Build a map of table -> database for verification
	tableDatabase := make(map[string]string)
	for _, col := range ds.Columns {
		// All columns of the same table should have the same database
		if existing, ok := tableDatabase[col.Table]; ok {
			if existing != col.Database {
				t.Errorf("inconsistent Database field for table %s: expected %q, got %q",
					col.Table, existing, col.Database)
			}
		} else {
			tableDatabase[col.Table] = col.Database
		}
	}

	// Verify expected database assignments
	expected := map[string]string{
		"users":      "",
		"events":     "analytics",
		"audit_logs": "logs",
	}

	for table, expectedDB := range expected {
		if actualDB, ok := tableDatabase[table]; !ok {
			t.Errorf("table %s not found in parsed columns", table)
		} else if actualDB != expectedDB {
			t.Errorf("table %s: expected Database=%q, got %q", table, expectedDB, actualDB)
		}
	}
}

func TestSchemaDatabaseRoundtrip(t *testing.T) {
	// Create DBInfo with mixed tables (some with Database, some without)
	di1 := sdata.GetTestDBInfoWithDatabase()

	// Write schema
	var buf bytes.Buffer
	if err := writeSchema(di1, &buf); err != nil {
		t.Fatal(err)
	}

	// Parse it back
	ds, err := graphql.ParseSchema(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	// Create new DBInfo from parsed data
	di2 := sdata.NewDBInfo(ds.Type,
		ds.Version,
		ds.Schema,
		"",
		ds.Columns,
		ds.Functions,
		nil)

	// Build maps of table -> database for both DBInfos
	getTableDatabases := func(di *sdata.DBInfo) map[string]string {
		result := make(map[string]string)
		for _, t := range di.Tables {
			result[t.Name] = t.Database
		}
		return result
	}

	db1 := getTableDatabases(di1)
	db2 := getTableDatabases(di2)

	// Verify database assignments are preserved
	for table, expectedDB := range db1 {
		if actualDB, ok := db2[table]; !ok {
			t.Errorf("table %s not found after roundtrip", table)
		} else if actualDB != expectedDB {
			t.Errorf("table %s: Database not preserved, expected %q, got %q",
				table, expectedDB, actualDB)
		}
	}
}

func TestSchemaBackwardCompatibility(t *testing.T) {
	// Parse existing schema without any @database directives
	schema := []byte(`
# dbinfo:postgres,140000,public

type users {
	id:	Bigint!	@id
	name:	CharacterVarying!
	email:	CharacterVarying!
}

type products {
	id:	Bigint!	@id
	name:	CharacterVarying
	price:	Numeric
}

type orders {
	id:	Bigint!	@id
	user_id:	Bigint	@relation(type: users, field: id)
	product_id:	Bigint	@relation(type: products, field: id)
}
`)

	ds, err := graphql.ParseSchema(schema)
	if err != nil {
		t.Fatalf("failed to parse schema without @database directives: %v", err)
	}

	// Verify no errors and all tables have empty Database field
	for _, col := range ds.Columns {
		if col.Database != "" {
			t.Errorf("table %s, column %s: expected empty Database, got %q",
				col.Table, col.Name, col.Database)
		}
	}

	// Verify we can create a DBInfo from the parsed schema
	di := sdata.NewDBInfo(ds.Type,
		ds.Version,
		ds.Schema,
		"",
		ds.Columns,
		ds.Functions,
		nil)

	// Verify all tables in DBInfo have empty Database field
	for _, tbl := range di.Tables {
		if tbl.Database != "" {
			t.Errorf("table %s: expected empty Database in DBTable, got %q",
				tbl.Name, tbl.Database)
		}
	}
}

func TestSchemaDiffMultiDB_RequiresDatabaseDirective(t *testing.T) {
	schema := []byte(`
# dbinfo:postgres,140000,public

type users {
	id:	Bigint!	@id
	name:	CharacterVarying!
}

type events @database(name: analytics) {
	id:	Bigint!	@id
	event_type:	CharacterVarying!
}

type audit_logs {
	id:	Bigint!	@id
	action:	CharacterVarying!
}
`)

	connections := map[string]*sql.DB{
		"analytics": nil,
		"logs":      nil,
	}
	dbTypes := map[string]string{
		"analytics": "postgres",
		"logs":      "postgres",
	}

	di, perr := engine.ParseSchemaSDL(schema, "", nil)
	if perr != nil {
		t.Fatalf("parse schema: %v", perr)
	}

	_, err := engine.SchemaDiffMultiDB(connections, dbTypes, di, nil, engine.DiffOptions{})
	if err == nil {
		t.Fatal("expected error for tables missing @database directive, got nil")
	}

	errMsg := err.Error()

	// Verify the error mentions the missing tables (sorted order)
	if !strings.Contains(errMsg, "audit_logs") {
		t.Errorf("error should mention 'audit_logs', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "users") {
		t.Errorf("error should mention 'users', got: %s", errMsg)
	}

	// Verify the error mentions available databases
	if !strings.Contains(errMsg, "analytics") {
		t.Errorf("error should mention available database 'analytics', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "logs") {
		t.Errorf("error should mention available database 'logs', got: %s", errMsg)
	}

	// Verify it mentions the @database directive
	if !strings.Contains(errMsg, "@database") {
		t.Errorf("error should mention '@database' directive, got: %s", errMsg)
	}
}

func TestSchemaDiffMultiDB_AllTablesHaveDatabase(t *testing.T) {
	schema := []byte(`
# dbinfo:postgres,140000,public

type users @database(name: analytics) {
	id:	Bigint!	@id
	name:	CharacterVarying!
}

type events @database(name: analytics) {
	id:	Bigint!	@id
	event_type:	CharacterVarying!
}

type audit_logs @database(name: logs) {
	id:	Bigint!	@id
	action:	CharacterVarying!
}
`)

	// Use empty connections — validation only checks the schema columns,
	// so the loop over connections doesn't need to execute.
	connections := map[string]*sql.DB{}
	dbTypes := map[string]string{}

	di, perr := engine.ParseSchemaSDL(schema, "", nil)
	if perr != nil {
		t.Fatalf("parse schema: %v", perr)
	}

	_, err := engine.SchemaDiffMultiDB(connections, dbTypes, di, nil, engine.DiffOptions{})
	if err != nil {
		t.Fatalf("expected no validation error when all tables have @database, got: %v", err)
	}
}

func TestSchemaRoundTrip_ClusteringKeys(t *testing.T) {
	schema := []byte(`# dbinfo:snowflake,1,main

type events @cluster(columns: ["created_at", "region"]) {
	id:	BigInt!	@id
	created_at:	Timestamp!
	region:	Varchar!
}
`)

	ds, err := graphql.ParseSchema(schema)
	if err != nil {
		t.Fatalf("ParseSchema failed: %v", err)
	}

	if len(ds.ClusteringKeys) != 1 {
		t.Fatalf("expected 1 clustering key entry, got %d", len(ds.ClusteringKeys))
	}

	ck := ds.ClusteringKeys[0]
	if ck.Table != "events" {
		t.Errorf("table = %q, want %q", ck.Table, "events")
	}
	if len(ck.Keys) != 2 || ck.Keys[0] != "created_at" || ck.Keys[1] != "region" {
		t.Errorf("keys = %v, want [created_at, region]", ck.Keys)
	}
}
