package qcode_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/sdata"
)

var dbs *sdata.DBSchema

func init() {
	var err error

	dbs, err = sdata.NewDBSchema(sdata.GetTestDBInfo(), nil)
	if err != nil {
		panic(err)
	}
}

func TestCompile1(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})
	err := qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name"},
		},
	})
	if err != nil {
		t.Error(err)
		return
	}

	_, err = qc.Compile([]byte(`
	query { products(id: 15) {
			id
			name
		} }`), nil, "user", "")

	if err != nil {
		t.Fatal(err)
	}
}

func TestCompile2(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})
	err := qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"ID"},
		},
	})
	if err != nil {
		t.Error(err)
		return
	}

	_, err = qc.Compile([]byte(`
	query { product(id: $id) {
			id
			price
		} }`), nil, "user", "")

	if err == nil {
		t.Fatal(errors.New("expected an error: 'products.price' blocked"))
	}
}

func TestCompile3(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})
	err := qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"ID"},
		},
	})
	if err != nil {
		t.Error(err)
		return
	}

	vars := json.RawMessage(`
		{ "data": { "name": "my_name", "description": "my_desc"  } }`)

	vars1 := make(map[string]json.RawMessage)
	if err := json.Unmarshal(vars, &vars1); err != nil {
		t.Error(err)
	}

	_, err = qc.Compile([]byte(`
	mutation {
		products(insert: $data) {
			id
			name
		}
	}`), vars1, "user", "")

	if err != nil {
		t.Fatal(err)
	}
}

func TestCompile4(t *testing.T) {
	gql := `mutation {
		users(insert: { email: $email, full_name: $full_name}) {
			id
		}
	}`

	vars := json.RawMessage(`{
		"email":     "reannagreenholt@orn.com",
		"full_name": "Flo Barton"
	}`)

	vars1 := make(map[string]json.RawMessage)
	if err := json.Unmarshal(vars, &vars1); err != nil {
		t.Error(err)
	}

	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_, err := qc.Compile([]byte(gql), vars1, "user", "")
	if err != nil {
		t.Fatal(err)
	}
}

// TestWhereFKColumnNotMisinterpretedAsRelationship verifies that filtering on a
// foreign key column (e.g. customer_id on purchases) uses a simple column filter,
// not a relationship join to the customers table.
func TestWhereFKColumnNotMisinterpretedAsRelationship(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})

	// purchases.customer_id is an FK to customers.id — filtering on it should
	// produce a column WHERE clause, not a nested EXISTS join to customers.
	_, err := qc.Compile([]byte(`
	query {
		purchases(where: { customer_id: { eq: 5 } }) {
			id
			quantity
		}
	}`), nil, "user", "")

	if err != nil {
		t.Fatalf("expected FK column filter to compile, got: %v", err)
	}
}

// TestWhereFKColumnProduct verifies product_id FK column filter on purchases.
func TestWhereFKColumnProduct(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})

	// purchases.product_id is an FK to products.id
	_, err := qc.Compile([]byte(`
	query {
		purchases(where: { product_id: { eq: 10 } }) {
			id
			sale_type
		}
	}`), nil, "user", "")

	if err != nil {
		t.Fatalf("expected FK column filter to compile, got: %v", err)
	}
}

// TestWhereNestedRelationshipStillWorks ensures genuine nested table where
// clauses continue to work after the FK column disambiguation fix.
func TestWhereNestedRelationshipStillWorks(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})

	// "users" here is NOT a column on products — it should still be treated
	// as a nested relationship filter (products → users via user_id FK).
	_, err := qc.Compile([]byte(`
	query {
		products(where: { users: { email: { eq: "test@test.com" } } }) {
			id
			name
		}
	}`), nil, "user", "")

	if err != nil {
		t.Fatalf("expected nested relationship filter to compile, got: %v", err)
	}
}

// TestWhereNestedFKColumnNotMisinterpreted verifies that filtering through a
// relationship using an FK column on the intermediate table works correctly.
// e.g. purchases → customers where customers.user_id = 5
// user_id is an FK on customers pointing to users — it must be treated as a
// column filter, not navigated further to the users table.
func TestWhereNestedFKColumnNotMisinterpreted(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})

	_, err := qc.Compile([]byte(`
	query {
		purchases(where: { customers: { user_id: { eq: 5 } } }) {
			id
			quantity
		}
	}`), nil, "user", "")

	if err != nil {
		t.Fatalf("expected nested FK column filter to compile, got: %v", err)
	}
}

func TestWhereVariableRejected(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})

	_, err := qc.Compile([]byte(`
	query($where: UsersWhereInput) {
		users(where: $where) {
			id
		}
	}`), nil, "user", "")

	if err == nil {
		t.Fatal("expected where variable to be rejected")
	}
	if !strings.Contains(err.Error(), "where must be an inline object; use variables only inside filter values") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubscriptionWhereVariableRejected(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})

	_, err := qc.Compile([]byte(`
	subscription($where: UsersWhereInput) {
		users(where: $where) {
			id
		}
	}`), nil, "user", "")

	if err == nil {
		t.Fatal("expected subscription where variable to be rejected")
	}
	if !strings.Contains(err.Error(), "where must be an inline object; use variables only inside filter values") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFKColumnRelationNameCollisionUsesTargetTable(t *testing.T) {
	di := sdata.NewDBInfo("postgres", 0, "public", "", nil, nil, nil)
	di.AddTable(sdata.NewDBTable("public", "organization", "", []sdata.DBColumn{
		{Name: "org_key", Type: "bigint", PrimaryKey: true, UniqueKey: true},
		{Name: "name", Type: "varchar"},
	}))
	di.AddTable(sdata.NewDBTable("public", "employees", "", []sdata.DBColumn{
		{Name: "id", Type: "bigint", PrimaryKey: true, UniqueKey: true},
		{Name: "org_key", Type: "bigint", FKeySchema: "public", FKeyTable: "organization", FKeyCol: "org_key"},
	}))
	schema, err := sdata.NewDBSchema(di, nil)
	if err != nil {
		t.Fatal(err)
	}
	qc, _ := qcode.NewCompiler(schema, qcode.Config{})

	_, err = qc.Compile([]byte(`
	query {
		employees {
			org_key
			organization {
				name
			}
		}
	}`), nil, "user", "")
	if err != nil {
		t.Fatalf("expected FK scalar plus target-table relationship to compile, got: %v", err)
	}

	_, err = qc.Compile([]byte(`
	query {
		employees {
			org_key {
				name
			}
		}
	}`), nil, "user", "")
	if err == nil {
		t.Fatal("expected FK column name with a child selection to be rejected")
	}
}

func TestMultipleFKsToSameTableKeepColumnRelationshipNames(t *testing.T) {
	di := sdata.NewDBInfo("postgres", 0, "public", "", nil, nil, nil)
	di.AddTable(sdata.NewDBTable("public", "graph_node", "", []sdata.DBColumn{
		{Name: "id", Type: "varchar", PrimaryKey: true, UniqueKey: true},
	}))
	di.AddTable(sdata.NewDBTable("public", "graph_edge", "", []sdata.DBColumn{
		{Name: "id", Type: "bigint", PrimaryKey: true, UniqueKey: true},
		{Name: "src_node", Type: "varchar", FKeySchema: "public", FKeyTable: "graph_node", FKeyCol: "id"},
		{Name: "dst_node", Type: "varchar", FKeySchema: "public", FKeyTable: "graph_node", FKeyCol: "id"},
	}))
	schema, err := sdata.NewDBSchema(di, nil)
	if err != nil {
		t.Fatal(err)
	}
	qc, _ := qcode.NewCompiler(schema, qcode.Config{})

	_, err = qc.Compile([]byte(`
	query {
		graph_node {
			id
			src_node { id }
			dst_node { id }
		}
	}`), nil, "user", "")
	if err != nil {
		t.Fatalf("expected join-table relationships to compile by FK column names, got: %v", err)
	}
}

func TestInvalidCompile1(t *testing.T) {
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_, err := qcompile.Compile([]byte(`#`), nil, "user", "")

	if err == nil {
		t.Fatal(errors.New("expecting an error"))
	}
}

func TestInvalidCompile2(t *testing.T) {
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_, err := qcompile.Compile([]byte(`{u(where:{not:0})}`), nil, "user", "")

	if err == nil {
		t.Fatal(errors.New("expecting an error"))
	}
}

func TestEmptyCompile(t *testing.T) {
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_, err := qcompile.Compile([]byte(``), nil, "user", "")

	if err == nil {
		t.Fatal(errors.New("expecting an error"))
	}
}

func TestInvalidPostfixCompile(t *testing.T) {
	gql := `mutation
updateThread {
  thread(update: $data, where: { slug: { eq: $slug } }) {
    slug
    title
    published
    createdAt : created_at
    totalVotes : cached_votes_total
    totalPosts : cached_posts_total
    vote : thread_vote(where: { user_id: { eq: $user_id } }) {
     id
    }
    topics {
      slug
      name
    }
	}
}
}}`
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_, err := qcompile.Compile([]byte(gql), nil, "anon", "")

	if err == nil {
		t.Fatal(errors.New("expecting an error"))
	}
}

func TestFragmentsCompile1(t *testing.T) {
	gql := `
	fragment userFields1 on user {
		id
		email
	}

	query {
		users {
			...userFields2

			created_at
			...userFields1
		}
	}

	fragment userFields2 on user {
		full_name
		phone
	}
	`
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_, err := qcompile.Compile([]byte(gql), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestFragmentsCompile2(t *testing.T) {
	gql := `
	query {
		users {
			...userFields2

			created_at
			...userFields1
		}
	}

	fragment userFields1 on user {
		id
		email
	}

	fragment userFields2 on user {
		full_name
		phone
	}`
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_, err := qcompile.Compile([]byte(gql), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestFragmentsCompile3(t *testing.T) {
	gql := `
	fragment userFields1 on user {
		id
		email
	}

	fragment userFields2 on user {
		full_name
		phone
	}

	query {
		users {
			...userFields2

			created_at
			...userFields1
		}
	}

	`
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_, err := qcompile.Compile([]byte(gql), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}
}

var gql = []byte(`
	{products(
		# returns only 30 items
		limit: 30,

		# starts from item 10, commented out for now
		# offset: 10,

		# orders the response items by highest price
		order_by: { price: desc },

		# no duplicate prices returned
		distinct: [ price ],

		# only items with an id >= 30 and < 30 are returned
		where: { id: { and: { greater_or_equals: 20, lt: 28 } } }) {
		id
		name
		price
	}}`)

var gqlWithFragments = []byte(`
fragment userFields1 on user {
	id
	email
	__typename
}

query {
	users {
		...userFields2

		created_at
		...userFields1
		__typename
	}
}

fragment userFields2 on user {
	full_name
	__typename
}`)

func BenchmarkQCompile(b *testing.B) {
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		_, err := qcompile.Compile(gql, nil, "user", "")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQCompileP(b *testing.B) {
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := qcompile.Compile(gql, nil, "user", "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestClusteringKeysNotUsedForPostgres(t *testing.T) {
	// The default test DB is postgres — clustering keys should be ignored
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name", "price", "created_at"},
		},
	})

	result, err := qc.Compile([]byte(`
		query {
			products(first: 20, after: $cursor) {
				id
				name
			}
		}`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	sel := result.Selects[0]

	// For postgres, should only have PK tie-breaker
	if len(sel.OrderBy) != 1 {
		t.Fatalf("expected 1 ORDER BY column for postgres, got %d: %v",
			len(sel.OrderBy), orderByNames(sel.OrderBy))
	}
	if sel.OrderBy[0].Col.Name != "id" {
		t.Errorf("ORDER BY[0]: expected %q, got %q", "id", sel.OrderBy[0].Col.Name)
	}
}

func TestPartitionFilterInjected(t *testing.T) {
	// Table with partition key and default range of 30 days
	pSchema, err := sdata.GetTestPartitionedSchema()
	if err != nil {
		t.Fatal(err)
	}

	qc, err := qcode.NewCompiler(pSchema, qcode.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name", "price", "created_at"},
		},
	})

	// Query without any filter on the partition column
	result, err := qc.Compile([]byte(`
		query {
			products {
				id
				name
			}
		}`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	sel := result.Selects[0]

	// The WHERE clause should have a partition filter injected
	if sel.Where.Exp == nil {
		t.Fatal("expected WHERE clause to have partition filter, got nil")
	}

	// Should have no warnings (filter was injected, not just warned)
	if len(result.Warnings) > 0 {
		t.Errorf("expected no warnings when default range is set, got: %v", result.Warnings)
	}

	// Verify the injected filter references the partition column
	if !qcode.HasFilterOnColumn(sel.Where.Exp, "created_at") {
		t.Error("expected injected filter to reference partition column 'created_at'")
	}
}

func TestPartitionFilterNotInjectedWhenUserFilters(t *testing.T) {
	pSchema, err := sdata.GetTestPartitionedSchema()
	if err != nil {
		t.Fatal(err)
	}

	qc, err := qcode.NewCompiler(pSchema, qcode.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name", "price", "created_at"},
		},
	})

	// Query WITH a filter on the partition column
	result, err := qc.Compile([]byte(`
		query {
			products(where: { created_at: { gte: $start_date } }) {
				id
				name
			}
		}`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	// No warnings — user already filtered on the partition column
	if len(result.Warnings) > 0 {
		t.Errorf("expected no warnings when user filters on partition column, got: %v",
			result.Warnings)
	}
}

func TestPartitionWarningWhenNoFilter(t *testing.T) {
	// Table with partition key but NO default range (warn only)
	pSchema, err := sdata.GetTestPartitionedWarnOnlySchema()
	if err != nil {
		t.Fatal(err)
	}

	qc, err := qcode.NewCompiler(pSchema, qcode.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name", "price", "created_at"},
		},
	})

	// Query without filter on partition column, no default range
	result, err := qc.Compile([]byte(`
		query {
			products {
				id
				name
			}
		}`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	// Should have a warning
	if len(result.Warnings) == 0 {
		t.Fatal("expected a warning about missing partition filter")
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "partition column") && strings.Contains(w, "created_at") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about partition column 'created_at', got: %v", result.Warnings)
	}
}

func TestPostgresOrderByAlwaysProjected(t *testing.T) {
	// ORDER BY columns remain projected for the Postgres/SQLite compiler path.
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name", "price"},
		},
	})

	result, err := qc.Compile([]byte(`
		query {
			products(order_by: { price: desc }) {
				id
				name
			}
		}`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	sel := result.Selects[0]

	found := false
	for _, bc := range sel.BCols {
		if bc.Col.Name == "price" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Postgres: ORDER BY column 'price' SHOULD still be in BCols (no projection optimization)")
	}
}

func TestPartitionFilterRequiredInOLAPWhenNoUserFilter(t *testing.T) {
	// analytics_mode=true + partition configured + no user filter on partition column
	// → compile populates PartitionFilterRequired; no filter injected.
	pSchema, err := sdata.GetTestPartitionedSchema()
	if err != nil {
		t.Fatal(err)
	}

	qc, err := qcode.NewCompiler(pSchema, qcode.Config{AnalyticsMode: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name", "price", "created_at"},
		},
	})

	result, err := qc.Compile([]byte(`
		query {
			products {
				id
				name
			}
		}`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	sel := result.Selects[0]
	if sel.PartitionFilterRequired == "" {
		t.Fatal("expected PartitionFilterRequired to be set in analytics_mode when user omits filter")
	}
	for _, needle := range []string{"products", "created_at"} {
		if !strings.Contains(sel.PartitionFilterRequired, needle) {
			t.Errorf("expected error to mention %q, got: %s", needle, sel.PartitionFilterRequired)
		}
	}

	// Must NOT silently inject the default time-range filter in OLAP mode.
	if qcode.HasFilterOnColumn(sel.Where.Exp, "created_at") {
		t.Error("analytics_mode must not inject a default partition filter")
	}
}

func TestPartitionFilterNotRequiredInOLAPWhenUserFilters(t *testing.T) {
	// analytics_mode=true + user already filters on partition column
	// → no error, no injection.
	pSchema, err := sdata.GetTestPartitionedSchema()
	if err != nil {
		t.Fatal(err)
	}

	qc, err := qcode.NewCompiler(pSchema, qcode.Config{AnalyticsMode: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name", "price", "created_at"},
		},
	})

	result, err := qc.Compile([]byte(`
		query {
			products(where: { created_at: { gte: $start_date } }) {
				id
				name
			}
		}`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	sel := result.Selects[0]
	if sel.PartitionFilterRequired != "" {
		t.Errorf("expected PartitionFilterRequired to be empty when user filters on partition column, got: %s",
			sel.PartitionFilterRequired)
	}
}

func TestPartitionFilterRequiredInOLAPFromImplicitTemporalColumn(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{AnalyticsMode: true})
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{Columns: []string{"id", "name", "price", "created_at"}},
	})

	result, err := qc.Compile([]byte(`query { products { id name } }`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	sel := result.Selects[0]
	if sel.PartitionFilterRequired == "" {
		t.Fatal("expected implicit partition detection to require a filter in analytics_mode")
	}
	for _, needle := range []string{"products", "created_at", "unrestricted"} {
		if !strings.Contains(sel.PartitionFilterRequired, needle) {
			t.Errorf("expected error to mention %q, got: %s", needle, sel.PartitionFilterRequired)
		}
	}
}

func TestPartitionFilterSatisfiedByFilterOnImplicitKey(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{AnalyticsMode: true})
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{Columns: []string{"id", "name", "price", "created_at"}},
	})

	result, err := qc.Compile([]byte(`
		query { products(where: { created_at: { gt: $cutoff } }) { id name } }`),
		nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}
	if s := result.Selects[0].PartitionFilterRequired; s != "" {
		t.Errorf("expected empty, got: %s", s)
	}
}

func currenciesOnlyDBInfo(partitionNone bool) *sdata.DBInfo {
	cols := []sdata.DBColumn{
		{Schema: "public", Table: "currencies", Name: "code", Type: "varchar", NotNull: true, PrimaryKey: true, UniqueKey: true},
		{Schema: "public", Table: "currencies", Name: "name", Type: "varchar"},
	}
	di := sdata.NewDBInfo("postgres", 110000, "public", "db", cols, nil, nil)
	if partitionNone {
		for i := range di.Tables {
			if di.Tables[i].Name == "currencies" {
				di.Tables[i].PartitionNone = true
			}
		}
	}
	return di
}

func TestPartitionFilterUnrestrictedBypassesCheckWhenNoColumn(t *testing.T) {
	schema, err := sdata.NewDBSchema(currenciesOnlyDBInfo(false), nil)
	if err != nil {
		t.Fatal(err)
	}

	qc, _ := qcode.NewCompiler(schema, qcode.Config{AnalyticsMode: true})
	_ = qc.AddRole("user", "public", "currencies", qcode.TRConfig{
		Query: qcode.QueryConfig{Columns: []string{"code", "name"}},
	})

	result, err := qc.Compile([]byte(`query { currencies(unrestricted: true) { code name } }`),
		nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}
	if s := result.Selects[0].PartitionFilterRequired; s != "" {
		t.Errorf("expected unrestricted:true to bypass partition check, got: %s", s)
	}
}

func TestPartitionFilterUnrestrictedBypassesDetectedColumn(t *testing.T) {
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{AnalyticsMode: true})
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{Columns: []string{"id", "name", "price", "created_at"}},
	})

	result, err := qc.Compile([]byte(`query { products(unrestricted: true) { id name } }`),
		nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}
	if s := result.Selects[0].PartitionFilterRequired; s != "" {
		t.Errorf("unrestricted:true should bypass the implicit partition check, got: %s", s)
	}
}

func TestPartitionFilterPassesWhenNoColumnDetectable(t *testing.T) {
	schema, err := sdata.NewDBSchema(currenciesOnlyDBInfo(false), nil)
	if err != nil {
		t.Fatal(err)
	}

	qc, _ := qcode.NewCompiler(schema, qcode.Config{AnalyticsMode: true})
	_ = qc.AddRole("user", "public", "currencies", qcode.TRConfig{
		Query: qcode.QueryConfig{Columns: []string{"code", "name"}},
	})

	result, err := qc.Compile([]byte(`query { currencies { code name } }`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}
	if s := result.Selects[0].PartitionFilterRequired; s != "" {
		t.Errorf("no detectable partition column should not produce an error, got: %s", s)
	}
}

func TestPartitionFilterNoneConfigBypassesCheck(t *testing.T) {
	schema, err := sdata.NewDBSchema(currenciesOnlyDBInfo(true), nil)
	if err != nil {
		t.Fatal(err)
	}

	qc, _ := qcode.NewCompiler(schema, qcode.Config{AnalyticsMode: true})
	_ = qc.AddRole("user", "public", "currencies", qcode.TRConfig{
		Query: qcode.QueryConfig{Columns: []string{"code", "name"}},
	})

	result, err := qc.Compile([]byte(`query { currencies { code name } }`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}
	if s := result.Selects[0].PartitionFilterRequired; s != "" {
		t.Errorf("expected PartitionNone to bypass check, got: %s", s)
	}
}

func TestPartitionNoWarningForNonPartitionedTable(t *testing.T) {
	// Standard postgres schema — no partition keys
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name", "price"},
		},
	})

	result, err := qc.Compile([]byte(`
		query {
			products {
				id
				name
			}
		}`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Warnings) > 0 {
		t.Errorf("expected no warnings for non-partitioned table, got: %v", result.Warnings)
	}
}

// --- Dangerous query warning regression test ---

func TestPostgresNoDangerousQueryWarnings(t *testing.T) {
	// Postgres should never get dangerous query warnings
	qc, _ := qcode.NewCompiler(dbs, qcode.Config{})
	_ = qc.AddRole("user", "public", "products", qcode.TRConfig{
		Query: qcode.QueryConfig{
			Columns: []string{"id", "name", "price"},
		},
	})

	result, err := qc.Compile([]byte(`
		query {
			products {
				count_id
			}
		}`), nil, "user", "")
	if err != nil {
		t.Fatal(err)
	}

	for _, w := range result.Warnings {
		if strings.Contains(w, "aggregation") || strings.Contains(w, "clustering") || strings.Contains(w, "full table scan") {
			t.Errorf("Postgres should not get dangerous query warnings, got: %s", w)
		}
	}
}

func orderByNames(obs []qcode.OrderBy) []string {
	names := make([]string, len(obs))
	for i, ob := range obs {
		names[i] = ob.Col.Name
	}
	return names
}

func BenchmarkQCompileFragment(b *testing.B) {
	qcompile, _ := qcode.NewCompiler(dbs, qcode.Config{})

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		_, err := qcompile.Compile(gqlWithFragments, nil, "user", "")
		if err != nil {
			b.Fatal(err)
		}
	}
}
