package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

type (
	IntroOptions = schema.IntroOptions
	IntroResult  = schema.IntroResult
	FullType     = schema.FullType
	FieldObject  = schema.FieldObject
	TypeRef      = schema.TypeRef
)

var (
	BuildIntrospection = schema.BuildIntrospection
)

func TestIntrospectionIncludesUnderscoreOperators(t *testing.T) {
	// Create a simple in-memory schema for testing
	di := sdata.GetTestDBInfo()
	schema, err := sdata.NewDBSchema(di, nil)
	if err != nil {
		t.Fatal(err)
	}

	result, err := BuildIntrospection(IntroOptions{
		Schemas: []*sdata.DBSchema{schema},
	})
	if err != nil {
		t.Fatal(err)
	}

	var introResult IntroResult
	if err := json.Unmarshal(result, &introResult); err != nil {
		t.Fatal(err)
	}

	// Check if IntExpression type exists and has _eq field
	var intExpressionType *FullType
	for _, typ := range introResult.Schema.Types {
		if typ.Name == "IntExpression" {
			intExpressionType = &typ
			break
		}
	}

	if intExpressionType == nil {
		t.Fatal("IntExpression type not found in schema")
	}

	// Check for _eq field
	hasEq := false
	for _, field := range intExpressionType.InputFields {
		if field.Name == "_eq" {
			hasEq = true
			break
		}
	}

	if !hasEq {
		t.Error("IntExpression type does not have _eq field")
	}

	// Check if any WhereInput type exists and has _or field
	var whereInputType *FullType
	for _, typ := range introResult.Schema.Types {
		if len(typ.Name) > 10 && typ.Name[len(typ.Name)-10:] == "WhereInput" {
			whereInputType = &typ
			break
		}
	}

	if whereInputType == nil {
		t.Fatal("No WhereInput type found in schema")
	}

	// Check for _or field
	hasOr := false
	for _, field := range whereInputType.InputFields {
		if field.Name == "_or" {
			hasOr = true
			break
		}
	}

	if !hasOr {
		t.Errorf("WhereInput type %s does not have _or field", whereInputType.Name)
	}
}

func TestIntrospectionIncludesBothOperatorFormats(t *testing.T) {
	// Create a simple in-memory schema for testing
	di := sdata.GetTestDBInfo()
	schema, err := sdata.NewDBSchema(di, nil)
	if err != nil {
		t.Fatal(err)
	}

	result, err := BuildIntrospection(IntroOptions{
		Schemas: []*sdata.DBSchema{schema},
	})
	if err != nil {
		t.Fatal(err)
	}

	var introResult IntroResult
	if err := json.Unmarshal(result, &introResult); err != nil {
		t.Fatal(err)
	}

	// Find IntExpression type
	var intExpressionType *FullType
	for _, typ := range introResult.Schema.Types {
		if typ.Name == "IntExpression" {
			intExpressionType = &typ
			break
		}
	}

	if intExpressionType == nil {
		t.Fatal("IntExpression type not found in schema")
	}

	// Check that we have both formats of operators
	operatorPairs := []struct {
		camelCase  string
		underscore string
	}{
		{"equals", "_eq"},
		{"notEquals", "_neq"},
		{"greaterThan", "_gt"},
		{"lesserThan", "_lt"},
		{"greaterOrEquals", "_gte"},
		{"lesserOrEquals", "_lte"},
	}

	for _, pair := range operatorPairs {
		hasCamelCase := false
		hasUnderscore := false

		for _, field := range intExpressionType.InputFields {
			if field.Name == pair.camelCase {
				hasCamelCase = true
			}
			if field.Name == pair.underscore {
				hasUnderscore = true
			}
		}

		if !hasCamelCase {
			t.Errorf("IntExpression type missing camelCase operator: %s", pair.camelCase)
		}
		if !hasUnderscore {
			t.Errorf("IntExpression type missing underscore operator: %s", pair.underscore)
		}
	}

	// Check WhereInput boolean operators
	var whereInputType *FullType
	for _, typ := range introResult.Schema.Types {
		if len(typ.Name) > 10 && typ.Name[len(typ.Name)-10:] == "WhereInput" {
			whereInputType = &typ
			break
		}
	}

	if whereInputType == nil {
		t.Fatal("No WhereInput type found in schema")
	}

	boolOperatorPairs := []struct {
		camelCase  string
		underscore string
	}{
		{"and", "_and"},
		{"or", "_or"},
		{"not", "_not"},
	}

	for _, pair := range boolOperatorPairs {
		hasCamelCase := false
		hasUnderscore := false

		for _, field := range whereInputType.InputFields {
			if field.Name == pair.camelCase {
				hasCamelCase = true
			}
			if field.Name == pair.underscore {
				hasUnderscore = true
			}
		}

		if !hasCamelCase {
			t.Errorf("WhereInput type missing camelCase operator: %s", pair.camelCase)
		}
		if !hasUnderscore {
			t.Errorf("WhereInput type missing underscore operator: %s", pair.underscore)
		}
	}
}

func TestIntrospectionIncludesSyntheticAggregateFields(t *testing.T) {
	introResult := introspectTestDB(t, IntroOptions{})
	products := requireIntroType(t, introResult, "products")

	for _, name := range []string{"count_id", "count_name", "sum_price", "avg_price", "min_price", "max_price"} {
		requireIntroField(t, products, name)
	}

	requireNoIntroField(t, products, "sum_name")
	requireNoIntroField(t, products, "avg_name")

	if got := introFieldTypeName(requireIntroField(t, products, "count_id").Type); got != "Int" {
		t.Fatalf("count_id type = %q, want Int", got)
	}
	if got := introFieldTypeName(requireIntroField(t, products, "sum_price").Type); got != "Float" {
		t.Fatalf("sum_price type = %q, want Float", got)
	}
}

func TestIntrospectionSyntheticAggregateCollisionPreservesPhysicalField(t *testing.T) {
	di := sdata.NewDBInfo("postgres", 0, "public", "", []sdata.DBColumn{
		{Schema: "public", Table: "things", Name: "id", Type: "bigint", PrimaryKey: true},
		{Schema: "public", Table: "things", Name: "likes", Type: "integer"},
		{
			Schema:  "public",
			Table:   "things",
			Name:    "count_likes",
			Type:    "text",
			Comment: "physical count column",
		},
	}, nil, nil)

	introResult := introspectDBInfo(t, di, IntroOptions{})
	things := requireIntroType(t, introResult, "things")
	countLikes := requireIntroField(t, things, "count_likes")

	if got := introFieldTypeName(countLikes.Type); got != "String" {
		t.Fatalf("count_likes type = %q, want String from physical column", got)
	}
}

func TestIntrospectionFKColumnRelationNameCollision(t *testing.T) {
	di := sdata.NewDBInfo("postgres", 0, "public", "", nil, nil, nil)
	di.AddTable(sdata.NewDBTable("public", "organization", "", []sdata.DBColumn{
		{Name: "org_key", Type: "bigint", PrimaryKey: true, UniqueKey: true},
		{Name: "name", Type: "varchar"},
	}))
	di.AddTable(sdata.NewDBTable("public", "employees", "", []sdata.DBColumn{
		{Name: "id", Type: "bigint", PrimaryKey: true, UniqueKey: true},
		{Name: "org_key", Type: "bigint", FKeySchema: "public", FKeyTable: "organization", FKeyCol: "org_key"},
	}))

	introResult := introspectDBInfo(t, di, IntroOptions{})
	employees := requireIntroType(t, introResult, "employees")

	orgKeyFields := introFieldsNamed(employees, "org_key")
	if len(orgKeyFields) != 1 {
		t.Fatalf("employees.org_key fields = %d, want exactly one scalar field", len(orgKeyFields))
	}
	if got := introFieldTypeName(orgKeyFields[0].Type); got != "Int" {
		t.Fatalf("employees.org_key type = %q, want Int", got)
	}

	orgRel := requireIntroField(t, employees, "organization")
	if got := introFieldTypeName(orgRel.Type); got != "organization" {
		t.Fatalf("employees.organization type = %q, want organization", got)
	}

	organization := requireIntroType(t, introResult, "organization")
	orgPKFields := introFieldsNamed(organization, "org_key")
	if len(orgPKFields) != 1 {
		t.Fatalf("organization.org_key fields = %d, want exactly one scalar field", len(orgPKFields))
	}
	if got := introFieldTypeName(orgPKFields[0].Type); got != "ID" {
		t.Fatalf("organization.org_key type = %q, want ID", got)
	}
	requireNoIntroField(t, organization, "organization")
}

func TestIntrospectionSkipsSyntheticAggregatesWhenDisabled(t *testing.T) {
	introResult := introspectTestDB(t, IntroOptions{DisableAgg: true})
	products := requireIntroType(t, introResult, "products")

	requireIntroField(t, products, "id")
	requireNoIntroField(t, products, "count_id")
	requireNoIntroField(t, products, "sum_price")
}

func TestIntrospectionIncludesSyntheticCursorFields(t *testing.T) {
	introResult := introspectTestDB(t, IntroOptions{})

	query := requireIntroType(t, introResult, "Query")
	productsCursor := requireIntroField(t, query, "products_cursor")
	if got := introFieldTypeName(productsCursor.Type); got != "Cursor" {
		t.Fatalf("products_cursor type = %q, want Cursor", got)
	}
	requireNoIntroField(t, query, "productsByID_cursor")

	// Slim build: subscriptions are not supported, so the introspected
	// schema exposes no subscription root or Subscription type at all.
	for i := range introResult.Schema.Types {
		if introResult.Schema.Types[i].Name == "Subscription" {
			t.Fatalf("introspection must not include a %q type in the slim build", "Subscription")
		}
	}
	if introResult.Schema.SubscriptionType != nil {
		t.Fatalf("schema.subscriptionType = %q, want nil in the slim build",
			introResult.Schema.SubscriptionType.Name)
	}

	mutation := requireIntroType(t, introResult, "Mutation")
	requireNoIntroField(t, mutation, "products_cursor")

	users := requireIntroType(t, introResult, "users")
	requireIntroField(t, users, "comments_cursor")
}

func TestIntrospectionSyntheticFieldsRespectCamelcase(t *testing.T) {
	introResult := introspectTestDB(t, IntroOptions{CamelCase: true})

	products := requireIntroType(t, introResult, "products")
	requireIntroField(t, products, "countId")
	requireIntroField(t, products, "sumPrice")
	requireNoIntroField(t, products, "count_id")
	requireNoIntroField(t, products, "sum_price")

	query := requireIntroType(t, introResult, "Query")
	requireIntroField(t, query, "products_cursor")
}

func introspectTestDB(t *testing.T, opts IntroOptions) IntroResult {
	t.Helper()
	return introspectDBInfo(t, sdata.GetTestDBInfo(), opts)
}

func introspectDBInfo(t *testing.T, di *sdata.DBInfo, opts IntroOptions) IntroResult {
	t.Helper()

	schema, err := sdata.NewDBSchema(di, nil)
	if err != nil {
		t.Fatal(err)
	}
	opts.Schemas = []*sdata.DBSchema{schema}

	result, err := BuildIntrospection(opts)
	if err != nil {
		t.Fatal(err)
	}

	var introResult IntroResult
	if err := json.Unmarshal(result, &introResult); err != nil {
		t.Fatal(err)
	}
	return introResult
}

func requireIntroType(t *testing.T, introResult IntroResult, name string) *FullType {
	t.Helper()
	for i := range introResult.Schema.Types {
		if introResult.Schema.Types[i].Name == name {
			return &introResult.Schema.Types[i]
		}
	}
	t.Fatalf("introspection type %q not found", name)
	return nil
}

func requireIntroField(t *testing.T, ft *FullType, name string) FieldObject {
	t.Helper()
	for _, field := range ft.Fields {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("field %q not found on type %q; fields: %v", name, ft.Name, introFieldNames(ft))
	return FieldObject{}
}

func requireNoIntroField(t *testing.T, ft *FullType, name string) {
	t.Helper()
	for _, field := range ft.Fields {
		if field.Name == name {
			t.Fatalf("field %q unexpectedly found on type %q", name, ft.Name)
		}
	}
}

func introFieldTypeName(tr *TypeRef) string {
	for tr != nil {
		if tr.Name != nil {
			return *tr.Name
		}
		tr = tr.OfType
	}
	return ""
}

func introFieldNames(ft *FullType) []string {
	names := make([]string, len(ft.Fields))
	for i, field := range ft.Fields {
		names[i] = field.Name
	}
	return names
}

func introFieldsNamed(ft *FullType, name string) []FieldObject {
	fields := []FieldObject{}
	for _, field := range ft.Fields {
		if field.Name == name {
			fields = append(fields, field)
		}
	}
	return fields
}

func TestIntrospectionReverseRelationsOrderIndependent(t *testing.T) {
	introResult := introspectTestDB(t, IntroOptions{})

	// Both directions of the products<->users relationship must appear
	// no matter which table the builder processes first.
	users := requireIntroType(t, introResult, "users")
	requireIntroField(t, users, "products")

	products := requireIntroType(t, introResult, "products")
	requireIntroField(t, products, "user")
}
