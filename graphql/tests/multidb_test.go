package graphql_test

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"

	"fmt"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// TestSelectDatabaseField verifies that Select has a Database field.
func TestSelectDatabaseField(t *testing.T) {
	sel := qcode.Select{
		Table:    "users",
		Database: "analytics",
	}

	if sel.Database != "analytics" {
		t.Errorf("Database = %q, want %q", sel.Database, "analytics")
	}
}

// TestSkipTypeDatabaseJoin verifies SkipTypeDatabaseJoin is defined correctly.
func TestSkipTypeDatabaseJoin(t *testing.T) {
	// Verify SkipTypeDatabaseJoin has a reasonable value (not 0 which is SkipTypeNone)
	if qcode.SkipTypeDatabaseJoin == qcode.SkipTypeNone {
		t.Error("SkipTypeDatabaseJoin should not equal SkipTypeNone")
	}

	// Verify it's distinct from other skip types
	types := []qcode.SkipType{
		qcode.SkipTypeNone,
		qcode.SkipTypeDrop,
		qcode.SkipTypeRemote,
	}

	for _, st := range types {
		if st == qcode.SkipTypeDatabaseJoin {
			t.Errorf("SkipTypeDatabaseJoin should be distinct from %v", st)
		}
	}
}

// TestSkipTypeDatabaseJoinString verifies String() for SkipTypeDatabaseJoin.
func TestSkipTypeDatabaseJoinString(t *testing.T) {
	s := qcode.SkipTypeDatabaseJoin.String()
	if s != "SkipTypeDatabaseJoin" {
		t.Errorf("qcode.SkipTypeDatabaseJoin.String() = %q, want %q", s, "SkipTypeDatabaseJoin")
	}
}

// TestQCodeSelectsWithDatabaseField verifies QCode Selects can use Database field.
func TestQCodeSelectsWithDatabaseField(t *testing.T) {
	qc := &qcode.QCode{
		Selects: []qcode.Select{
			{Table: "users", Database: "main"},
			{Table: "orders", Database: "analytics"},
			{Table: "products", Database: ""}, // default
		},
	}

	if qc.Selects[0].Database != "main" {
		t.Errorf("Selects[0].Database = %q, want %q", qc.Selects[0].Database, "main")
	}
	if qc.Selects[1].Database != "analytics" {
		t.Errorf("Selects[1].Database = %q, want %q", qc.Selects[1].Database, "analytics")
	}
	if qc.Selects[2].Database != "" {
		t.Errorf("Selects[2].Database = %q, want empty", qc.Selects[2].Database)
	}
}

// TestSkipRenderWithDatabaseJoin verifies SkipRender can be set to SkipTypeDatabaseJoin.
func TestSkipRenderWithDatabaseJoin(t *testing.T) {
	sel := qcode.Select{
		Field: qcode.Field{
			SkipRender: qcode.SkipTypeDatabaseJoin,
		},
		Table:    "orders",
		Database: "analytics",
	}

	if sel.SkipRender != qcode.SkipTypeDatabaseJoin {
		t.Errorf("SkipRender = %v, want %v", sel.SkipRender, qcode.SkipTypeDatabaseJoin)
	}
}

// TestMixedSkipTypes verifies different skip types can coexist.
func TestMixedSkipTypes(t *testing.T) {
	selects := []qcode.Select{
		{Field: qcode.Field{SkipRender: qcode.SkipTypeNone}},
		{Field: qcode.Field{SkipRender: qcode.SkipTypeRemote}},
		{Field: qcode.Field{SkipRender: qcode.SkipTypeDatabaseJoin}},
	}

	// Count each type
	counts := make(map[qcode.SkipType]int)
	for _, sel := range selects {
		counts[sel.SkipRender]++
	}

	if counts[qcode.SkipTypeNone] != 1 {
		t.Errorf("SkipTypeNone count = %d, want 1", counts[qcode.SkipTypeNone])
	}
	if counts[qcode.SkipTypeRemote] != 1 {
		t.Errorf("SkipTypeRemote count = %d, want 1", counts[qcode.SkipTypeRemote])
	}
	if counts[qcode.SkipTypeDatabaseJoin] != 1 {
		t.Errorf("SkipTypeDatabaseJoin count = %d, want 1", counts[qcode.SkipTypeDatabaseJoin])
	}
}

// TestSelectFieldsForDatabaseJoin verifies a Select configured for DB join.
func TestSelectFieldsForDatabaseJoin(t *testing.T) {
	// A typical cross-database child select
	sel := qcode.Select{
		Field: qcode.Field{
			ID:         1,
			ParentID:   0,
			FieldName:  "orders",
			SkipRender: qcode.SkipTypeDatabaseJoin,
		},
		Table:    "orders",
		Database: "analytics",
	}

	if sel.ParentID != 0 {
		t.Errorf("ParentID = %d, want 0", sel.ParentID)
	}
	if sel.Database != "analytics" {
		t.Errorf("Database = %q, want %q", sel.Database, "analytics")
	}
	if sel.SkipRender != qcode.SkipTypeDatabaseJoin {
		t.Errorf("SkipRender = %v, want %v", sel.SkipRender, qcode.SkipTypeDatabaseJoin)
	}
}

// TestSelectTiDatabaseField verifies Ti.Database is accessible.
func TestSelectTiDatabaseField(t *testing.T) {
	sel := qcode.Select{
		Table:    "orders",
		Database: "analytics",
		Ti: sdata.DBTable{
			Name:     "orders",
			Database: "analytics",
		},
	}

	if sel.Ti.Database != "analytics" {
		t.Errorf("Ti.Database = %q, want %q", sel.Ti.Database, "analytics")
	}
}

// TestAddRelColumnsForDatabaseJoin verifies that addRelColumns handles
// RelDatabaseJoin correctly: adds a placeholder field to the parent select,
// sets SkipRender to SkipTypeDatabaseJoin, and sets the Database field.
func TestAddRelColumnsForDatabaseJoin(t *testing.T) {
	// Create a minimal compiler (addRelColumns doesn't use the compiler's fields)
	co := &Compiler{}

	// Set up parent and child selects
	parentSel := qcode.Select{
		Field:  qcode.Field{ID: 0, FieldName: "users"},
		Table:  "users",
		Fields: []qcode.Field{},
		BCols:  []qcode.Column{},
	}
	childSel := qcode.Select{
		Field:  qcode.Field{ID: 1, ParentID: 0, FieldName: "orders"},
		Table:  "orders",
		Ti:     sdata.DBTable{Name: "orders", Database: "analytics"},
		Fields: []qcode.Field{},
		BCols:  []qcode.Column{},
		Rel: sdata.DBRel{
			Type: sdata.RelDatabaseJoin,
			Right: sdata.DBRelRight{
				Col: sdata.DBColumn{Name: "user_id", Table: "users", Schema: "public"},
			},
		},
	}

	qc := &qcode.QCode{
		Selects: []qcode.Select{parentSel, childSel},
	}

	err := co.AddRelColumns(qc, &qc.Selects[1], qc.Selects[1].Rel)
	if err != nil {
		t.Fatalf("AddRelColumns() error: %v", err)
	}

	// Verify parent select got a placeholder field with the right name
	expectedPlaceholder := fmt.Sprintf("__%s_db_join", "orders")
	foundPlaceholder := false
	for _, f := range qc.Selects[0].Fields {
		if f.FieldName == expectedPlaceholder {
			foundPlaceholder = true
			if f.Col.Name != "user_id" {
				t.Errorf("placeholder field Col.Name = %q, want %q", f.Col.Name, "user_id")
			}
			break
		}
	}
	if !foundPlaceholder {
		t.Errorf("parent select missing placeholder field %q; fields: %v",
			expectedPlaceholder, qc.Selects[0].Fields)
	}

	// Verify child select has SkipRender set to SkipTypeDatabaseJoin
	if qc.Selects[1].SkipRender != qcode.SkipTypeDatabaseJoin {
		t.Errorf("child SkipRender = %v, want %v", qc.Selects[1].SkipRender, qcode.SkipTypeDatabaseJoin)
	}

	// Verify child select has Database set
	if qc.Selects[1].Database != "analytics" {
		t.Errorf("child Database = %q, want %q", qc.Selects[1].Database, "analytics")
	}
}
