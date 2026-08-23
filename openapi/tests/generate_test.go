package tests

import (
	"encoding/json"
	_ "github.com/aegion-dynamic/graphjin-slim/core/v3/lang/graphql"
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
