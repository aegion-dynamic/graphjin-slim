package graphql_test

import (
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/graphql/v3"
)

// joinsTo returns the joins of the EXISTS filter that ends on table name.
func joinsTo(ex *qcode.Exp, name string) []qcode.Join {
	if ex == nil {
		return nil
	}
	if ex.Op == qcode.OpSelectExists && len(ex.Joins) != 0 &&
		ex.Joins[len(ex.Joins)-1].Rel.Left.Ti.Name == name {
		return ex.Joins
	}
	for _, c := range ex.Children {
		if j := joinsTo(c, name); j != nil {
			return j
		}
	}
	return nil
}

// A relation inside an and/or list under a related table must join from that
// related table, not from the root table of the query.
func TestWhereNestedRelationInListJoinsFromParentRelation(t *testing.T) {
	qc, err := graphql.NewCompiler(dbs, graphql.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for name, q := range map[string]string{
		"and":       `query { purchases(where: { customers: { and: [{ vip: { eq: true } }, { users: { email: { eq: "a@b.c" } } }] } }) { id } }`,
		"or":        `query { purchases(where: { customers: { or: [{ vip: { eq: true } }, { users: { email: { eq: "a@b.c" } } }] } }) { id } }`,
		"multi-key": `query { purchases(where: { customers: { vip: { eq: true }, users: { email: { eq: "a@b.c" } } } }) { id } }`,
	} {
		t.Run(name, func(t *testing.T) {
			res, err := qc.Compile([]byte(q), nil, "")
			if err != nil {
				t.Fatal(err)
			}
			joins := joinsTo(res.Selects[0].Where.Exp, "users")
			if len(joins) != 1 {
				t.Fatalf("expected one join users -> customers, got %d", len(joins))
			}
			if r := joins[0].Rel.Right.Ti.Name; r != "customers" {
				t.Fatalf("expected users to join from customers, got %s", r)
			}
		})
	}
}
