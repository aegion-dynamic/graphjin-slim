package openapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/allow"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// Config controls the generated specification metadata.
type Config struct {
	Title       string // default "GraphJin REST API"
	Description string
	Version     string // default "1.0.0"
	ServerURL   string // default "/api/v1/rest"
}

func (c Config) withDefaults() Config {
	if c.Title == "" {
		c.Title = "GraphJin REST API"
	}
	if c.Version == "" {
		c.Version = "1.0.0"
	}
	if c.ServerURL == "" {
		c.ServerURL = "/api/v1/rest"
	}
	return c
}

// Generate builds an OpenAPI 3.0 document from an engine snapshot. Saved
// queries become REST paths; discovered tables become reusable schema
// components. Queries that fail to compile are skipped.
func Generate(inputs core.OpenAPIInputs, cfg Config) (*Document, error) {
	cfg = cfg.withDefaults()

	dbSchema, err := defaultSchema(inputs)
	if err != nil {
		return nil, err
	}
	qcc, err := qcode.NewCompiler(dbSchema, qcode.Config{})
	if err != nil {
		return nil, fmt.Errorf("openapi: failed to build compiler: %w", err)
	}

	doc := &Document{
		OpenAPI: "3.0.0",
		Info: Info{
			Title:       cfg.Title,
			Description: cfg.Description,
			Version:     cfg.Version,
		},
		Servers: []Server{
			{URL: cfg.ServerURL, Description: "GraphJin REST API Server"},
		},
		Paths: make(map[string]PathItem),
		Components: &Components{
			Schemas: make(map[string]Schema),
		},
	}

	generateComponents(doc.Components, inputs)

	for _, item := range inputs.Queries {
		pathItem, ok := generatePathItem(qcc, item)
		if !ok {
			continue
		}
		doc.Paths["/"+item.Name] = pathItem
	}

	return doc, nil
}

// GenerateJSON renders the specification for the given engine as indented
// JSON, ready to be served or written to disk.
func GenerateJSON(gj *core.GraphJin, ns *string, cfg Config) (json.RawMessage, error) {
	inputs, err := gj.OpenAPISpecInputs(ns)
	if err != nil {
		return nil, fmt.Errorf("openapi: failed to collect engine inputs: %w", err)
	}
	doc, err := Generate(inputs, cfg)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(doc, "", "  ")
}

// Module (defined in module.go) binds this configuration into a service
// module:
//
//	serv.NewGraphJinService(conf,
//	    serv.OptionSetModule(openapi.Module(openapi.Config{Title: "My API"})))

// defaultSchema returns the default database's schema from the snapshot.
func defaultSchema(inputs core.OpenAPIInputs) (*sdata.DBSchema, error) {
	if len(inputs.Databases) == 0 {
		return nil, fmt.Errorf("openapi: no database schemas available")
	}
	if inputs.Databases[core.DefaultDBName] != nil {
		return inputs.Databases[core.DefaultDBName], nil
	}
	names := sortedDBNames(inputs.Databases)
	return inputs.Databases[names[0]], nil
}

// sortedDBNames returns database names in deterministic order.
func sortedDBNames(dbs map[string]*sdata.DBSchema) []string {
	names := make([]string, 0, len(dbs))
	for name := range dbs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// generateComponents creates shared OpenAPI components from the discovered
// schemas. Tables from the default database keep their plain names;
// additional databases are prefixed to avoid collisions.
func generateComponents(components *Components, inputs core.OpenAPIInputs) {
	// Base response and error schemas shared by every operation.
	components.Schemas["GraphJinResponse"] = Schema{
		Type: "object",
		Properties: map[string]Schema{
			"data": {Type: "object", Description: "Query result data"},
			"errors": {
				Type: "array",
				Items: &Schema{
					Ref: "#/components/schemas/GraphQLError",
				},
			},
		},
	}
	components.Schemas["GraphQLError"] = Schema{
		Type: "object",
		Properties: map[string]Schema{
			"message": {Type: "string", Description: "Error message"},
			"path":    {Type: "array", Items: &Schema{Type: "string"}},
		},
		Required: []string{"message"},
	}

	prefixNonDefault := len(inputs.Databases) > 1
	for _, dbName := range sortedDBNames(inputs.Databases) {
		sdb := inputs.Databases[dbName]
		if sdb == nil {
			continue
		}
		prefix := ""
		if prefixNonDefault && dbName != core.DefaultDBName {
			prefix = title(dbName)
		}
		for _, table := range sdb.GetTables() {
			if table.Blocked || len(table.Columns) == 0 {
				continue
			}
			name := title(table.Name)
			if prefix != "" && !strings.HasPrefix(name, prefix+"_") {
				name = prefix + "_" + name
			}
			if _, exists := components.Schemas[name]; exists {
				continue
			}
			tableSchema := Schema{
				Type:       "object",
				Properties: make(map[string]Schema),
			}
			for _, col := range table.Columns {
				if col.Blocked {
					continue
				}
				tableSchema.Properties[col.Name] = columnToSchema(col)
			}
			components.Schemas[name] = tableSchema
			components.Schemas[name+"Array"] = Schema{
				Type:  "array",
				Items: &Schema{Ref: "#/components/schemas/" + name},
			}
		}
	}
}

// queryAnalysis is the compiled form of one saved query.
type queryAnalysis struct {
	name           string
	httpMethods    []string
	parameters     []Parameter
	responseSchema Schema
}

// analyzeQuery parses and compiles one saved query to extract HTTP methods,
// parameters, and a response schema.
func analyzeQuery(qcc *qcode.Compiler, item allow.Item) (*queryAnalysis, bool) {
	op, err := graph.Parse(item.Query)
	if err != nil {
		return nil, false
	}
	qc, err := qcc.Compile(item.Query, nil, item.Namespace)
	if err != nil {
		return nil, false
	}
	return &queryAnalysis{
		name:           item.Name,
		httpMethods:    httpMethodsFor(qc.Type, qc.SType),
		parameters:     extractParameters(op.VarDef),
		responseSchema: responseSchemaFromQCode(qc),
	}, true
}

// extractParameters converts GraphQL variable definitions to query parameters.
// Variable declarations use GraphJin's own syntax ($id = ID); the type name
// is carried in the value node's Val field.
func extractParameters(varDefs []graph.VarDef) []Parameter {
	var params []Parameter
	for _, varDef := range varDefs {
		typeName := "String"
		required := false
		if varDef.Val != nil {
			if t := strings.TrimSuffix(varDef.Val.Val, "!"); t != "" {
				typeName = t
			} else if varDef.Val.Name != "" {
				typeName = varDef.Val.Name
			}
			required = varDef.Val.Type == graph.NodeLabel &&
				len(varDef.Val.Children) > 0 &&
				varDef.Val.Children[0].Type == graph.NodeLabel
		}
		params = append(params, Parameter{
			Name:        varDef.Name,
			In:          "query",
			Description: fmt.Sprintf("GraphQL variable: %s", varDef.Name),
			Required:    required,
			Schema:      graphQLTypeToSchema(typeName),
		})
	}
	return params
}

// graphQLTypeToSchema maps a GraphQL scalar type to an OpenAPI schema.
func graphQLTypeToSchema(graphQLType string) Schema {
	gqlType, isList := schema.ColumnGraphQLType(graphQLType)

	baseType, format := "string", ""
	description := ""
	switch gqlType {
	case "String":
		baseType = "string"
	case "ID":
		baseType = "string"
		description = "Unique identifier"
	case "Int":
		baseType, format = "integer", "int32"
	case "Float":
		baseType, format = "number", "float"
	case "Boolean":
		baseType = "boolean"
	case "JSON":
		return Schema{Type: "object", AdditionalProperties: true}
	default:
		description = fmt.Sprintf("Custom type: %s", gqlType)
	}

	if isList {
		inner := Schema{Type: baseType, Format: format}
		return Schema{Type: "array", Items: &inner, Description: description}
	}
	return Schema{Type: baseType, Format: format, Description: description}
}

// responseSchemaFromQCode builds the response schema from compiled IR.
// Root selections reference their table's component schema.
func responseSchemaFromQCode(qc *qcode.QCode) Schema {
	var dataSchema Schema

	switch {
	case len(qc.Roots) == 0:
		dataSchema = Schema{Type: "object"}
	case len(qc.Roots) == 1:
		rootSel := &qc.Selects[qc.Roots[0]]
		if name := rootSel.Ti.Name; name != "" {
			ref := Schema{Ref: "#/components/schemas/" + title(name)}
			if rootSel.Singular {
				dataSchema = ref
			} else {
				dataSchema = Schema{Type: "array", Items: &ref}
			}
		} else {
			dataSchema = Schema{Type: "object", Description: "Query result"}
		}
	default:
		props := make(map[string]Schema, len(qc.Roots))
		for _, rootID := range qc.Roots {
			sel := &qc.Selects[rootID]
			if sel.Ti.Name == "" {
				props[sel.FieldName] = Schema{Type: "object", Description: "Query result"}
				continue
			}
			ref := Schema{Ref: "#/components/schemas/" + title(sel.Ti.Name)}
			if sel.Singular {
				props[sel.FieldName] = ref
			} else {
				props[sel.FieldName] = Schema{Type: "array", Items: &ref}
			}
		}
		dataSchema = Schema{Type: "object", Properties: props}
	}

	return Schema{
		Type: "object",
		Properties: map[string]Schema{
			"data":   dataSchema,
			"errors": {Ref: "#/components/schemas/GraphQLError"},
		},
	}
}

// httpMethodsFor determines appropriate HTTP methods for the operation.
func httpMethodsFor(opType, subType qcode.QType) []string {
	switch opType {
	case qcode.QTQuery:
		return []string{"GET", "POST"}
	case qcode.QTMutation:
		switch subType {
		case qcode.QTInsert:
			return []string{"POST"}
		case qcode.QTUpdate, qcode.QTUpsert:
			return []string{"PUT", "POST"}
		case qcode.QTDelete:
			return []string{"DELETE", "POST"}
		default:
			return []string{"POST"}
		}
	case qcode.QTSubscription:
		return []string{"GET"}
	default:
		return []string{"POST"}
	}
}

// generatePathItem creates the OpenAPI path item for a saved query. The
// second return value is false when the query could not be analyzed.
func generatePathItem(qcc *qcode.Compiler, item allow.Item) (PathItem, bool) {
	a, ok := analyzeQuery(qcc, item)
	if !ok {
		return PathItem{}, false
	}

	pathItem := PathItem{}
	for _, method := range a.httpMethods {
		operation := &Operation{
			Summary:     fmt.Sprintf("Execute %s query", item.Name),
			Description: fmt.Sprintf("Executes the %s GraphQL query via REST", item.Name),
			OperationID: fmt.Sprintf("%s_%s", strings.ToLower(method), item.Name),
			Tags:        []string{title(item.Operation)},
			Responses: map[string]Response{
				"200": {
					Description: "Successful response",
					Content: map[string]MediaType{
						"application/json": {Schema: a.responseSchema},
					},
				},
				"400": {
					Description: "Bad request",
					Content: map[string]MediaType{
						"application/json": {
							Schema: Schema{Ref: "#/components/schemas/GraphJinResponse"},
						},
					},
				},
			},
		}

		if method == "GET" {
			// Typed per-variable parameters only — the same variables the
			// server reads as individual query parameters on GET.
			operation.Parameters = a.parameters
		}

		if (method == "POST" || method == "PUT") && len(a.parameters) > 0 {
			varsSchema := Schema{
				Type:       "object",
				Properties: make(map[string]Schema, len(a.parameters)),
			}
			for _, param := range a.parameters {
				varsSchema.Properties[param.Name] = param.Schema
			}
			operation.RequestBody = &RequestBody{
				Description: "GraphQL variables as JSON object",
				Content: map[string]MediaType{
					"application/json": {Schema: varsSchema},
				},
			}
		}

		switch method {
		case "GET":
			pathItem.Get = operation
		case "POST":
			pathItem.Post = operation
		case "PUT":
			pathItem.Put = operation
		case "DELETE":
			pathItem.Delete = operation
		}
	}

	return pathItem, true
}

// columnToSchema converts a database column to an OpenAPI schema.
func columnToSchema(col sdata.DBColumn) Schema {
	baseType, format := "string", ""
	description := col.Comment

	if col.PrimaryKey {
		format = "uuid"
		description = "Primary key"
	} else {
		gqlType, _ := schema.ColumnGraphQLType(col.Type)
		sqlType := strings.ToLower(col.Type)

		switch gqlType {
		case "String":
			baseType = "string"
			switch {
			case strings.Contains(sqlType, "timestamp") || strings.Contains(sqlType, "date"):
				format = "date-time"
			case strings.Contains(sqlType, "time"):
				format = "time"
			case strings.Contains(sqlType, "uuid"):
				format = "uuid"
			}
		case "Int":
			baseType = "integer"
			if strings.Contains(sqlType, "big") {
				format = "int64"
			} else {
				format = "int32"
			}
		case "Float":
			baseType, format = "number", "float"
		case "Boolean":
			baseType = "boolean"
		case "JSON":
			return Schema{Type: "object", AdditionalProperties: true, Description: description}
		default:
			description = fmt.Sprintf("Unknown SQL type: %s", col.Type)
		}
	}

	out := Schema{Type: baseType, Format: format, Description: description}
	if col.Array {
		inner := Schema{Type: baseType, Format: format}
		return Schema{Type: "array", Items: &inner, Description: description}
	}
	return out
}

// title capitalizes the first rune of s, replacing the old x/text dependency.
func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
