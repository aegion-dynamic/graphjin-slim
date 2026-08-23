package sqlgen_test

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sqlgen"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

var (
	qcompile *qcode.Compiler
	scompile *sqlgen.Compiler
)

func TestMain(m *testing.M) {
	var err error

	schema, err := sdata.GetTestSchema()
	if err != nil {
		log.Fatal(err)
	}

	qcompile, err = qcode.NewCompiler(schema, qcode.Config{DBSchema: schema.DBSchema()})
	if err != nil {
		log.Fatal(err)
	}

	vars := map[string]string{
		"admin_account_id": "5",
		"get_price":        "sql:select price from prices where id = $product_id",
	}

	scompile = sqlgen.NewCompiler(sqlgen.Config{
		Vars: vars,
	})

	os.Exit(m.Run())
}

func compileGQLToPSQL(t *testing.T, gql string,
	vars map[string]json.RawMessage,
	role string,
) {
	var v json.RawMessage
	var err error

	if v, err = json.Marshal(vars); err != nil {
		t.Error(err)
	}

	if err := _compileGQLToPSQL(t, gql, v, role); err != nil {
		t.Error(err)
	}
}

func compileGQLToPSQLExpectErr(t *testing.T, gql string,
	vars map[string]json.RawMessage,
	role string,
) {
	var v json.RawMessage
	var err error

	if v, err = json.Marshal(v); err != nil {
		t.Error(err)
	}

	if err := _compileGQLToPSQL(t, gql, v, role); err == nil {
		t.Error(errors.New("we were expecting an error"))
	}
}

func _compileGQLToPSQL(t *testing.T, gql string, vars json.RawMessage, role string) error {
	v := make(map[string]json.RawMessage)
	if err := json.Unmarshal(vars, &v); err != nil {
		t.Error(err)
	}

	for i := 0; i < 1000; i++ {
		qc, err := qcompile.Compile([]byte(gql), v, "")
		if err != nil {
			return err
		}

		_, _, err = scompile.CompileEx(qc)
		if err != nil {
			return err
		}
	}

	return nil
}

// TestCompositeFK_ThroughColumn_EmitsFullJoinCondition is the end-to-end
// verification for P5: Stage 1b made @through(column:) match any column
// of a composite FK via ExtraPairs. This test confirms the SQL emitter
// then uses ALL composite columns in the join condition (not just the
// primary). If only the primary column appears in the emitted JOIN, the
// composite FK isn't actually being honored downstream — which would be
// a real bug to fix here.
//
// Schema (GetTestCompositeFKSchema):
//
//	enrollment.(term_id, course_id) ─ composite FK ─→ course_offering.(term_id, course_id)
//
// The query names course_id — the SECOND composite column, which lives
// in ExtraPairs not L/R. Pre-Stage-1b this returned "relationship not
// found".
func TestCompositeFK_ThroughColumn_EmitsFullJoinCondition(t *testing.T) {
	schema, err := sdata.GetTestCompositeFKSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	qc, err := qcode.NewCompiler(schema, qcode.Config{DBSchema: schema.DBSchema()})
	if err != nil {
		t.Fatalf("qcode compiler: %v", err)
	}

	pc := sqlgen.NewCompiler(sqlgen.Config{})

	gql := `query {
		enrollment {
			id
			course_offering @through(column: "course_id") {
				term_id
				course_id
				title
			}
		}
	}`

	qcRes, err := qc.Compile([]byte(gql), nil, "")
	if err != nil {
		t.Fatalf("qcode compile: %v", err)
	}

	_, sqlBytes, err := pc.CompileEx(qcRes)
	if err != nil {
		t.Fatalf("sqlgen compile: %v", err)
	}
	sql := string(sqlBytes)

	// Both composite columns must appear in the emitted join condition,
	// not just somewhere in the SELECT. Look for term_id AND course_id
	// after the WHERE keyword that follows the LATERAL course_offering
	// subquery — that's where the join predicate lives.
	loIdx := strings.Index(strings.ToLower(sql), "course_offering")
	if loIdx < 0 {
		t.Fatalf("emitted SQL missing course_offering join:\n%s", sql)
	}
	whereIdx := strings.Index(strings.ToLower(sql[loIdx:]), "where")
	if whereIdx < 0 {
		t.Fatalf("emitted SQL has course_offering but no WHERE clause for the join:\n%s", sql)
	}
	joinClause := sql[loIdx+whereIdx:]
	if !strings.Contains(joinClause, "term_id") {
		t.Errorf("join clause missing term_id (primary composite column).\nfull SQL:\n%s\njoin clause:\n%s", sql, joinClause)
	}
	if !strings.Contains(joinClause, "course_id") {
		t.Errorf("join clause missing course_id (ExtraPairs composite column).\nfull SQL:\n%s\njoin clause:\n%s", sql, joinClause)
	}
	t.Logf("emitted SQL (composite-FK join verified):\n%s", sql)
}
