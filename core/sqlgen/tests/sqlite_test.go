package sqlgen_test

import (
	graphql "github.com/aegion-dynamic/graphjin-slim/core/v3/lang/graphql"

	"bytes"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sqlgen"
)

func TestSQLiteGeneration(t *testing.T) {
	schema, err := sdata.GetTestSchema()
	if err != nil {
		t.Fatal(err)
	}

	qcCompiler, err := graphql.NewCompiler(schema, graphql.Config{DBSchema: schema.DBSchema()})
	if err != nil {
		t.Fatal(err)
	}

	gql := `query {
		users {
			id
			products {
				name
			}
		}
	}`

	qc, err := qcCompiler.Compile([]byte(gql), nil, "")
	if err != nil {
		t.Fatal(err)
	}

	conf := sqlgen.Config{
		DBType: "sqlite",
	}
	co := sqlgen.NewCompiler(conf)

	var w bytes.Buffer
	_, err = co.Compile(&w, qc)
	if err != nil {
		t.Fatal(err)
	}

	sql := w.String()
	t.Log(sql)

	if strings.Contains(sql, "LATERAL") {
		t.Error("Generated SQL contains LATERAL join, expected inline rendering for SQLite")
	}

	if !strings.Contains(sql, "json_group_array") {
		t.Error("Generated SQL missing json_group_array")
	}
}

func TestSQLiteEmptySelection(t *testing.T) {
	schema, err := sdata.GetTestSchema()
	if err != nil {
		t.Fatal(err)
	}

	qcCompiler, err := graphql.NewCompiler(schema, graphql.Config{DBSchema: schema.DBSchema()})
	if err != nil {
		t.Fatal(err)
	}

	gql := `query {
		users {
			id @skip(if: $skip_id)
		}
	}`

	qc, err := qcCompiler.Compile([]byte(gql), nil, "")
	if err != nil {
		t.Fatal(err)
	}

	conf := sqlgen.Config{
		DBType: "sqlite",
	}
	co := sqlgen.NewCompiler(conf)

	var w bytes.Buffer
	_, err = co.Compile(&w, qc)
	if err != nil {
		t.Fatal(err)
	}

	sql := w.String()
	// t.Log(sql)

	if strings.Contains(sql, "SELECT FROM") {
		t.Error("Generated SQL contains invalid 'SELECT FROM' syntax for SQLite")
	}
}
