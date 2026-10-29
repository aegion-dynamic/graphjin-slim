package graphql_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sqlgen"
	"github.com/aegion-dynamic/graphjin-slim/graphql/v3"
)

func TestMutationAuditCaptureSQL(t *testing.T) {
	dbs, err := sdata.NewDBSchema(sdata.GetTestDBInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"insert":   `mutation { products(insert: { id: 1, name: "a", price: 10 }) { id name } }`,
		"update":   `mutation { products(where: { id: { eq: 1 } }, update: { price: 10 }) { id } }`,
		"delete":   `mutation { products(where: { id: { eq: 1 } }, delete: true) { id } }`,
		"upsert":   `mutation { products(upsert: { id: 1, name: "a", price: 10 }, where: { id: { eq: 1 } }) { id } }`,
		"typename": `mutation { products(insert: { id: 1, name: "a", price: 10 }) { __typename id } }`,
	}

	for name, gql := range cases {
		t.Run(name, func(t *testing.T) {
			qc, err := graphql.NewCompiler(dbs, graphql.Config{})
			if err != nil {
				t.Fatal(err)
			}
			qcode, err := qc.Compile([]byte(gql), nil, "")
			if err != nil {
				t.Fatal(err)
			}
			c := sqlgen.NewCompiler(sqlgen.Config{DBType: "postgres", CaptureMutationChanges: true})
			c.SetSchemaInfo(dbs.GetTables())
			var b bytes.Buffer
			if _, err := c.Compile(&b, qcode); err != nil {
				t.Fatal(err)
			}
			sql := b.String()

			for _, want := range []string{
				`RETURNING`,
				`to_jsonb(old) AS "__gj_old"`,
				`to_jsonb(new) AS "__gj_new"`,
				`"__gj_audit" AS (`,
				`jsonb_build_object('schema', 'public', 'table', 'products'`,
				`(COALESCE("__gj_new", "__gj_old")->>'id')`,
			} {
				if !strings.Contains(sql, want) {
					t.Fatalf("SQL missing %q:\n%s", want, sql)
				}
			}
			if strings.Index(sql, `RETURNING`) > strings.Index(sql, `"__gj_audit" AS (`) {
				t.Fatalf("audit CTE was rendered before RETURNING:\n%s", sql)
			}
			if name == "upsert" {
				if strings.Index(sql, `ON CONFLICT`) > strings.Index(sql, `RETURNING`) {
					t.Fatalf("upsert conflict clause was rendered outside the CTE:\n%s", sql)
				}
			}

		})
	}
}

func TestMutationAuditCaptureConflictGetSQL(t *testing.T) {
	dbs, err := sdata.NewDBSchema(sdata.GetTestDBInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	qc, err := graphql.NewCompiler(dbs, graphql.Config{})
	if err != nil {
		t.Fatal(err)
	}
	qcode, err := qc.Compile([]byte(`mutation { users(insert: { id: 1, email: "ada@example.com" }, on_conflict: get) { id } }`), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	c := sqlgen.NewCompiler(sqlgen.Config{DBType: "postgres", CaptureMutationChanges: true})
	c.SetSchemaInfo(dbs.GetTables())
	var b bytes.Buffer
	if _, err := c.Compile(&b, qcode); err != nil {
		t.Fatal(err)
	}
	sql := b.String()
	if !strings.Contains(sql, `UNION ALL SELECT *, NULL::jsonb AS "__gj_old", NULL::jsonb AS "__gj_new"`) {
		t.Fatalf("conflict-get union does not carry the audit columns:\n%s", sql)
	}
	if !strings.Contains(sql, `WHERE "__gj_old" IS NOT NULL OR "__gj_new" IS NOT NULL`) {
		t.Fatalf("audit CTE does not filter unchanged conflict rows:\n%s", sql)
	}
}

func TestMutationAuditCaptureDisabled(t *testing.T) {
	dbs, err := sdata.NewDBSchema(sdata.GetTestDBInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	qc, err := graphql.NewCompiler(dbs, graphql.Config{})
	if err != nil {
		t.Fatal(err)
	}
	qcode, err := qc.Compile([]byte(`mutation { products(insert: { id: 1, name: "a", price: 10 }) { id } }`), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	c := sqlgen.NewCompiler(sqlgen.Config{DBType: "postgres"})
	c.SetSchemaInfo(dbs.GetTables())
	var b bytes.Buffer
	if _, err := c.Compile(&b, qcode); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), `__gj_audit`) {
		t.Fatalf("audit envelope rendered with capture disabled:\n%s", b.String())
	}
}
