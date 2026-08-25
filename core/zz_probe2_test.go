package core_test

import (
	"bytes"
	"encoding/json"
	"testing"

	graphql "github.com/aegion-dynamic/graphjin-slim/graphql/v3"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sqlgen"
)

func TestProbeMutates(t *testing.T) {
	schema, _ := sdata.NewDBSchema(sdata.GetTestDBInfo(), nil)
	fe, _ := graphql.NewCompiler(schema, graphql.Config{})
	gql := `mutation { users(insert: { full_name: "P", email: "p@x.io", products: { name: "C", price: 1 } }) { id products { id name } } }`
	qc, err := fe.Compile([]byte(gql), map[string]json.RawMessage{}, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Mutates=%d", len(qc.Mutates))

	sc := sqlgen.NewCompiler(sqlgen.Config{DBType: "postgres"})
	var w bytes.Buffer
	if _, err := sc.Compile(&w, qc); err != nil {
		t.Fatal(err)
	}
	t.Logf("PG-SQL=%.600s", w.String())
}
