package tests

import (
	"encoding/json"
	_ "github.com/aegion-dynamic/graphjin-slim/graphql/v3"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/allow"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/openapi/v3"
)

func testInputs(t *testing.T) core.OpenAPIInputs {
	t.Helper()
	schema, err := sdata.GetTestSchema()
	if err != nil {
		t.Fatalf("GetTestSchema: %v", err)
	}
	return core.OpenAPIInputs{
		Databases: map[string]*sdata.DBSchema{core.DefaultDBName: schema},
		Queries: []allow.Item{
			{
				Name:      "getUsers",
				Namespace: "admin",
				Query:     []byte("query getUsers { users(limit: 5) { id full_name } }"),
			},
			{
				Name:      "getUser",
				Namespace: "admin",
				Query:     []byte("query getUser($id = ID) { users(id: $id) { id email } }"),
			},
			{
				Name:      "broken",
				Namespace: "admin",
				Query:     []byte("query broken { nope_not_real { id } }"),
			},
			{
				// A bulk insert: the payload is a list of input objects, and
				// the operation has no value to give at description time.
				Name:      "insertUsers",
				Namespace: "admin",
				Query: []byte("mutation insertUsers($rows: [usersInput!]!) " +
					"{ users(insert: $rows) { id } }"),
			},
			{
				Name:      "listUsers",
				Namespace: "admin",
				Query:     []byte("query listUsers { users { id email } }"),
			},
		},
	}
}

func TestGeneratePathsAndComponents(t *testing.T) {
	doc, err := openapi.Generate(testInputs(t), openapi.Config{Title: "Test API"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if doc.OpenAPI != "3.0.0" {
		t.Errorf("openapi version = %q, want 3.0.0", doc.OpenAPI)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("title = %q, want Test API", doc.Info.Title)
	}

	for _, path := range []string{"/getUsers", "/getUser"} {
		if _, ok := doc.Paths[path]; !ok {
			t.Errorf("missing path %q (have %v)", path, doc.Paths)
		}
	}
	// Broken query is skipped, not fatal.
	if _, ok := doc.Paths["/broken"]; ok {
		t.Errorf("broken query should be skipped")
	}

	for _, comp := range []string{"GraphJinResponse", "GraphQLError"} {
		if _, ok := doc.Components.Schemas[comp]; !ok {
			t.Errorf("missing component %q", comp)
		}
	}
}

func TestGenerateHTTPMethodsAndParams(t *testing.T) {
	doc, err := openapi.Generate(testInputs(t), openapi.Config{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	item, ok := doc.Paths["/getUser"]
	if !ok {
		t.Fatalf("missing /getUser")
	}
	if item.Get == nil {
		t.Fatalf("expected GET operation for /getUser")
	}
	foundIDParam := false
	for _, p := range item.Get.Parameters {
		if p.Name == "id" && p.In == "query" {
			foundIDParam = true
			if p.Schema.Type != "string" || p.Schema.Format != "uuid" && p.Schema.Description == "" {
				t.Logf("id param schema: %+v", p.Schema)
			}
		}
	}
	if !foundIDParam {
		t.Errorf("expected query param id on GET /getUser, got %+v", item.Get.Parameters)
	}

	if item.Post == nil || item.Post.RequestBody == nil {
		t.Errorf("expected POST with request body for /getUser")
	}
}

func TestGenerateJSONNilEngine(t *testing.T) {
	// A nil engine must fail cleanly, not panic.
	raw, err := openapi.GenerateJSON(nil, nil, openapi.Config{})
	if err == nil {
		t.Fatalf("expected error for nil engine, got raw=%v", raw)
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateJSONOutput(t *testing.T) {
	doc, err := openapi.Generate(testInputs(t), openapi.Config{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(b)
	for _, want := range []string{`"openapi": "3.0.0"`, `"/getUsers"`, `"GraphQLError"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json output missing %s", want)
		}
	}
}

// TestBulkInsertOperationIsInTheSpec pins the operation whose payload is a
// list of input objects.
//
// The operation used to be dropped from the specification, because compiling
// a mutation whose payload is a variable needs the value of that variable,
// and the generator has none. The compile for a description does without it.
func TestBulkInsertOperationIsInTheSpec(t *testing.T) {
	doc, err := openapi.Generate(testInputs(t), openapi.Config{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	item, ok := doc.Paths["/insertUsers"]
	if !ok {
		t.Fatalf("bulk insert operation missing from the spec (have %v)", doc.Paths)
	}
	if item.Post == nil || item.Post.RequestBody == nil {
		t.Fatalf("expected a POST with a request body")
	}
	body := item.Post.RequestBody.Content["application/json"].Schema
	rows, ok := body.Properties["rows"]
	if !ok {
		t.Fatalf("request body has no rows property: %+v", body.Properties)
	}
	if rows.Type != "array" {
		t.Errorf("rows schema type = %q, want array", rows.Type)
	}
	if rows.Items == nil || rows.Items.Ref != "#/components/schemas/UsersInput" {
		t.Errorf("rows items = %+v, want a ref to UsersInput", rows.Items)
	}
	if _, ok := doc.Components.Schemas["UsersInput"]; !ok {
		t.Errorf("missing the UsersInput component")
	}
}

// TestResponseSchemaMatchesTheBridge pins the shape of the answer.
//
// The bridge answers with an object keyed by the root field, so data holds
// one property per root and each is typed from the operation's own selection
// set, not from the whole table. Errors are a list.
func TestResponseSchemaMatchesTheBridge(t *testing.T) {
	doc, err := openapi.Generate(testInputs(t), openapi.Config{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	item, ok := doc.Paths["/listUsers"]
	if !ok {
		t.Fatalf("missing /listUsers")
	}
	resp := item.Get.Responses["200"]
	schema := resp.Content["application/json"].Schema

	data, ok := schema.Properties["data"]
	if !ok {
		t.Fatalf("response has no data property: %+v", schema.Properties)
	}
	if data.Type != "object" {
		t.Errorf("data type = %q, want object", data.Type)
	}
	users, ok := data.Properties["users"]
	if !ok {
		t.Fatalf("data has no users property: %+v", data.Properties)
	}
	if users.Type != "array" {
		t.Errorf("users type = %q, want array", users.Type)
	}
	if users.Items == nil {
		t.Fatalf("users has no items schema")
	}
	// The selection set, not the whole table: full_name is not selected.
	if _, ok := users.Items.Properties["email"]; !ok {
		t.Errorf("users item is missing the email property: %+v", users.Items.Properties)
	}
	if _, ok := users.Items.Properties["full_name"]; ok {
		t.Errorf("users item should not have the full_name property: %+v", users.Items.Properties)
	}

	errs, ok := schema.Properties["errors"]
	if !ok {
		t.Fatalf("response has no errors property")
	}
	if errs.Type != "array" {
		t.Errorf("errors type = %q, want array", errs.Type)
	}
}

// TestTypesAreTightEnoughForGeneratedClients pins the types a generated
// client is built from: variables typed from their declared GraphQL type,
// required variables listed, nullable ones allowed to be null, and response
// fields required, nullable only where the column may be null.
func TestTypesAreTightEnoughForGeneratedClients(t *testing.T) {
	inputs := testInputs(t)
	inputs.Queries = append(inputs.Queries,
		allow.Item{
			Name: "updateUser",
			Query: []byte(`mutation updateUser($id: UUID!, $user_id: UUID!, $n: Int!, $j: JSON!, $at: Time!, $phone: String) {
				users(update: { phone: $phone }, where: { id: { eq: $id } }) { id phone }
			}`),
		},
		allow.Item{
			Name:  "countUsers",
			Query: []byte(`query countUsers { users { count_id } }`),
		},
	)
	doc, err := openapi.Generate(inputs, openapi.Config{ServerVars: []string{"user_id"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	body := doc.Paths["/updateUser"].Post.RequestBody.Content["application/json"].Schema
	want := map[string]string{"id": "string", "n": "integer", "at": "string", "phone": "string"}
	for name, typ := range want {
		if got := body.Properties[name].Type; got != typ {
			t.Errorf("variable %s type = %q, want %q", name, got, typ)
		}
	}
	if s := body.Properties["j"]; s.Type != "" {
		t.Errorf("JSON variable should accept any value, got type %q", s.Type)
	}
	if f := body.Properties["id"].Format; f != "uuid" {
		t.Errorf("UUID variable format = %q, want uuid", f)
	}
	if !body.Properties["phone"].Nullable || body.Properties["id"].Nullable {
		t.Errorf("only the optional variable is nullable: %+v", body.Properties)
	}
	if _, ok := body.Properties["user_id"]; ok {
		t.Errorf("a server variable is not part of the request")
	}
	if got := strings.Join(body.Required, ","); got != "at,id,j,n" {
		t.Errorf("required = %q, want at,id,j,n", got)
	}

	row := doc.Paths["/updateUser"].Post.Responses["200"].Content["application/json"].Schema.
		Properties["data"].Properties["users"].Items
	if got := strings.Join(row.Required, ","); got != "id,phone" {
		t.Errorf("response required = %q, want id,phone", got)
	}
	if row.Properties["id"].Nullable || !row.Properties["phone"].Nullable {
		t.Errorf("only the nullable column is nullable: %+v", row.Properties)
	}
	if typ := row.Properties["id"].Type; typ != "integer" {
		t.Errorf("bigint primary key type = %q, want integer", typ)
	}

	agg := doc.Paths["/countUsers"].Get.Responses["200"].Content["application/json"].Schema.
		Properties["data"].Properties["users"].Items.Properties["count_id"]
	if agg.Type != "integer" {
		t.Errorf("count_id type = %q, want integer", agg.Type)
	}
}
