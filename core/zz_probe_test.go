package core_test

import (
	"testing"

	graphql "github.com/aegion-dynamic/graphjin-slim/graphql/v3"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

func TestProbe(t *testing.T) {
	schema, _ := sdata.NewDBSchema(sdata.GetTestDBInfo(), nil)
	co, _ := graphql.NewCompiler(schema, graphql.Config{})
	for _, q := range []string{
		"mutation { users(insert: { full_name: \"a'b\", email: \"x@y.z\" }) { id } }",
		"mutation { users(insert: { full_name: \"Robert'); DROP TABLE x;--\", email: \"x2@y.z\" }) { id } }",
	} {
		if _, err := co.Compile([]byte(q), nil, ""); err != nil {
			t.Logf("ERR  %.45s -> %v", q, err)
		} else {
			t.Logf("OK   %.45s", q)
		}
	}
}
