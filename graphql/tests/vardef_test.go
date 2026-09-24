package graphql_test

// Variable-definition capture for OpenAPI contracts.
//
// The REST bridge projects each allow-listed .gql operation into a typed
// endpoint; the request body comes from QueryParameters, which reads the
// parser's VarDef list. Standard GraphQL declarations ($var: Type!) must
// therefore survive parsing — this test pins that with .gql fixtures.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/graphql/v3"
)

type wantParam struct {
	name     string
	typ      string
	required bool
}

func TestVarDefCapture(t *testing.T) {
	cases := []struct {
		file string
		want []wantParam
	}{
		{"01_scalar_vars.gql", []wantParam{{"eventId", "String", true}}},
		{"02_mixed_vars.gql", []wantParam{
			{"user_id", "String", true},
			{"title", "String", true},
			{"notes", "String", false},
			{"priority", "String", false},
		}},
		{"03_input_object.gql", []wantParam{{"input", "insertusersInput", true}}},
		{"04_legacy_default.gql", []wantParam{{"q", "String", false}}},
		{"05_no_vars.gql", nil},
	}

	var l graphql.Lang
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "vardef", tc.file))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			got, err := l.QueryParameters(raw)
			if err != nil {
				t.Fatalf("QueryParameters: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d params (%+v), want %d (%+v)", len(got), got, len(tc.want), tc.want)
			}
			for i, w := range tc.want {
				if got[i].Name != w.name || got[i].Type != w.typ || got[i].Required != w.required {
					t.Errorf("param %d = %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}
