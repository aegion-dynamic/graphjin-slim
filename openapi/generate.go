package openapi

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/allow"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/langadapter"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// Logger receives the generator's warnings.
type Logger interface {
	Warnf(format string, args ...any)
}

// Config controls the generated specification metadata.
type Config struct {
	Title       string // default "GraphJin REST API"
	Description string
	Version     string // default "1.0.0"
	ServerURL   string // default "/api/v1/rest"

	// ServerVars are the variables the server fills itself, such as a
	// user_id taken from the caller's token. They are left out of the
	// request, because a caller does not send them.
	ServerVars []string

	// Logger, when set, is told about every operation that could not be
	// described. An operation that cannot be described is left out of the
	// specification, and saying so is the difference between a gap in the
	// documentation and a mystery.
	Logger Logger
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
	d, lerr := langadapter.Lookup(langadapter.DefaultLanguageName)
	if lerr != nil {
		return nil, lerr
	}
	cf, ok := d.(langadapter.CompilerFactory)
	if !ok {
		return nil, fmt.Errorf("openapi: language %q provides no compiler factory", langadapter.DefaultLanguageName)
	}
	qcc, err := cf.NewCompiler(dbSchema, langadapter.CompileConfig{DBSchema: dbSchema.DBSchema()})
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
		pathItem, ok := generatePathItem(qcc, item, cfg, inputs)
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

			// The input schema of a table: what a mutation payload may
			// carry. Every column is optional, because which ones a payload
			// has to carry depends on the operation.
			inputSchema := Schema{
				Type:       "object",
				Properties: make(map[string]Schema),
			}
			for _, col := range table.Columns {
				if col.Blocked {
					continue
				}
				inputSchema.Properties[col.Name] = columnToSchema(col)
			}
			components.Schemas[name+"Input"] = inputSchema
		}
	}
}

// componentName returns the component name of a table, the same name
// generateComponents gives it.
func componentName(table string, inputs core.OpenAPIInputs) string {
	prefixNonDefault := len(inputs.Databases) > 1
	for _, dbName := range sortedDBNames(inputs.Databases) {
		sdb := inputs.Databases[dbName]
		if sdb == nil {
			continue
		}
		for _, t := range sdb.GetTables() {
			if t.Name != table {
				continue
			}
			name := title(t.Name)
			if prefixNonDefault && dbName != core.DefaultDBName {
				prefix := title(dbName)
				if !strings.HasPrefix(name, prefix+"_") {
					name = prefix + "_" + name
				}
			}
			return name
		}
	}
	return title(table)
}

// queryAnalysis is the compiled form of one saved query.
type queryAnalysis struct {
	name           string
	httpMethods    []string
	parameters     []Parameter
	responseSchema Schema
	// payloadVar is the variable that carries the mutation payload, when the
	// operation is a mutation.
	payloadVar string
	// payloadTable is the table the payload is written to.
	payloadTable string
	// payloadComponent is the input schema of that table.
	payloadComponent string
	// payloadSchema is the schema of the payload variable.
	payloadSchema Schema
}

// analyzeQuery parses and compiles one saved query to extract HTTP methods,
// parameters, and a response schema.
//
// The compile is a shape only compile: a mutation whose payload is a variable
// needs the value of that variable to know its columns, and a description has
// no value to give. Compiling for shape keeps the operation in the
// specification instead of dropping it.
func analyzeQuery(qcc langadapter.Language, item allow.Item, cfg Config,
	inputs core.OpenAPIInputs,
) (*queryAnalysis, bool) {
	logger := cfg.Logger
	qc, err := qcc.Compile(item.Query, nil, langadapter.CompileOptions{
		Namespace: item.Namespace,
		ShapeOnly: true,
	})
	if err != nil {
		if logger != nil {
			logger.Warnf("openapi: operation %q is not in the specification: %s", item.Name, err)
		}
		return nil, false
	}
	var params []Parameter
	var ps []langadapter.Parameter
	if pd, ok := qcc.(langadapter.ParameterDescriber); ok {
		listed, perr := pd.QueryParameters(item.Query)
		if perr == nil {
			for _, p := range listed {
				if !slices.Contains(cfg.ServerVars, p.Name) {
					ps = append(ps, p)
				}
			}
			params = parametersFrom(ps, inputs)
		}
	}
	a := &queryAnalysis{
		name:           item.Name,
		httpMethods:    httpMethodsFor(qc.Type, qc.SType),
		parameters:     params,
		responseSchema: responseSchemaFromQCode(qc),
	}
	// A mutation carries its payload in a variable. The variable is typed
	// from the table it is written to, which is the root of the operation.
	if qc.ActionVar != "" && len(qc.Roots) > 0 {
		a.payloadVar = qc.ActionVar
		a.payloadTable = qc.Selects[qc.Roots[0]].Ti.Name
		a.payloadComponent = componentName(a.payloadTable, inputs) + "Input"
		for i := range ps {
			if ps[i].Name != a.payloadVar {
				continue
			}
			a.payloadSchema = payloadSchema(ps[i].Type, a.payloadComponent)
			a.payloadSchema.Nullable = !ps[i].Required
		}
		for i := range a.parameters {
			if a.parameters[i].Name == a.payloadVar {
				a.parameters[i].Schema = a.payloadSchema
			}
		}
	}
	return a, true
}

// payloadSchema types the variable that carries a mutation payload. The
// variable's own type says whether it carries one row or a list of them.
func payloadSchema(varType, component string) Schema {
	ref := Schema{Ref: "#/components/schemas/" + component}
	if strings.HasPrefix(varType, "[") {
		return Schema{Type: "array", Items: &ref}
	}
	return ref
}

// parametersFrom maps seam-level parameter metadata onto OpenAPI
// parameters. Variable declarations use the frontend's own syntax; the
// type label arrives already normalized by the language.
func parametersFrom(ps []langadapter.Parameter, inputs core.OpenAPIInputs) []Parameter {
	var params []Parameter
	for _, p := range ps {
		typeName := p.Type
		if typeName == "" {
			typeName = "String"
		}
		s := graphQLTypeToSchema(typeName, inputs)
		// A variable that is not required may be sent as null.
		s.Nullable = !p.Required
		params = append(params, Parameter{
			Name:        p.Name,
			In:          "query",
			Description: fmt.Sprintf("Variable: %s", p.Name),
			Required:    p.Required,
			Schema:      s,
		})
	}
	return params
}

// graphQLTypeToSchema maps the declared GraphQL type of a variable to an
// OpenAPI schema: a scalar, an input object of a table, or a list of either.
//
// The type is a GraphQL type and not a column type, so it is read as one:
// reading Int as a column type made every variable a string.
func graphQLTypeToSchema(graphQLType string, inputs core.OpenAPIInputs) Schema {
	t := strings.TrimSuffix(strings.TrimSpace(graphQLType), "!")
	if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
		inner := strings.TrimSpace(t[1 : len(t)-1])
		items := graphQLTypeToSchema(inner, inputs)
		items.Nullable = !strings.HasSuffix(inner, "!")
		return Schema{Type: "array", Items: &items}
	}

	switch t {
	case "String":
		return Schema{Type: "string"}
	case "ID":
		return Schema{Type: "string", Description: "Unique identifier"}
	case "UUID":
		return Schema{Type: "string", Format: "uuid"}
	case "Time", "DateTime", "Timestamp":
		return Schema{Type: "string", Format: "date-time"}
	case "Date":
		return Schema{Type: "string", Format: "date"}
	case "Int":
		return Schema{Type: "integer", Format: "int32"}
	case "Float":
		return Schema{Type: "number", Format: "float"}
	case "Boolean":
		return Schema{Type: "boolean"}
	case "JSON":
		// Any JSON value: an object, a list, a string, a number or a bool.
		return Schema{Description: "Any JSON value"}
	}
	if table, ok := strings.CutSuffix(t, "Input"); ok {
		return Schema{Ref: "#/components/schemas/" + componentName(table, inputs) + "Input"}
	}
	return Schema{Type: "string", Description: fmt.Sprintf("Custom type: %s", t)}
}

// responseSchemaFromQCode builds the response schema from compiled IR.
//
// The bridge answers with an object keyed by the root field, so data holds
// one property per root, each typed from that root's own selection set
// rather than from the whole table. The shape follows what the operation
// selects: the columns it names, and the relations it nests.
func responseSchemaFromQCode(qc *qcode.QCode) Schema {
	data := Schema{Type: "object", Properties: map[string]Schema{}}
	for _, rootID := range qc.Roots {
		sel := &qc.Selects[rootID]
		if sel.FieldName == "" {
			continue
		}
		ss := selectSchema(qc, sel)
		if sel.Singular {
			// A single row may not be found.
			ss.Nullable = true
			data.Properties[sel.FieldName] = ss
		} else {
			data.Properties[sel.FieldName] = Schema{Type: "array", Items: &ss}
		}
		data.Required = append(data.Required, sel.FieldName)
	}
	sort.Strings(data.Required)

	return Schema{
		Type: "object",
		Properties: map[string]Schema{
			"data":   data,
			"errors": {Type: "array", Items: &Schema{Ref: "#/components/schemas/GraphQLError"}},
		},
	}
}

// selectSchema types one selection: the columns it selects and the relations
// it nests. A relation is a list unless the selection asks for a single row.
func selectSchema(qc *qcode.QCode, sel *qcode.Select) Schema {
	obj := Schema{Type: "object", Properties: map[string]Schema{}}

	for _, f := range sel.Fields {
		name := f.FieldName
		if name == "" || strings.HasPrefix(name, "__") {
			// The engine's own tracking columns are not part of the answer.
			continue
		}
		// Every field the operation selects is in the answer. A column
		// that may be null says so.
		obj.Required = append(obj.Required, name)
		switch f.Type {
		case qcode.FieldTypeCol:
			cs := columnToSchema(f.Col)
			cs.Nullable = !f.Col.NotNull && !f.Col.PrimaryKey
			obj.Properties[name] = cs
		case qcode.FieldTypeFunc:
			obj.Properties[name] = funcSchema(f.Func)
		default:
			obj.Properties[name] = Schema{Description: "Computed value"}
		}
	}

	for _, childID := range sel.Children {
		child := &qc.Selects[childID]
		if child.FieldName == "" {
			continue
		}
		cs := selectSchema(qc, child)
		if child.Singular {
			// A related row is there when the foreign key that points to
			// it cannot be null.
			cs.Nullable = !relationRequired(sel, child)
			obj.Properties[child.FieldName] = cs
		} else {
			// A list is always there, and empty when nothing is related.
			obj.Properties[child.FieldName] = Schema{Type: "array", Items: &cs}
		}
		obj.Required = append(obj.Required, child.FieldName)
	}

	sort.Strings(obj.Required)
	return obj
}

// relationRequired reports whether the selection always has the related row:
// when the selection holds the foreign key to it and the key is not null.
func relationRequired(parent, child *qcode.Select) bool {
	for _, c := range []sdata.DBColumn{child.Rel.Left.Col, child.Rel.Right.Col} {
		if c.Table == parent.Ti.Name && c.FKeyTable != "" && c.NotNull {
			return true
		}
	}
	return false
}

// funcSchema types the value of a function field, such as an aggregate. A
// count is a whole number; other functions are typed from what they return.
func funcSchema(fn sdata.DBFunction) Schema {
	if fn.Name == "count" {
		return Schema{Type: "integer", Format: "int64"}
	}
	switch gqlType, _ := schema.ColumnScalarType(fn.Type); gqlType {
	case "Int":
		return Schema{Type: "integer", Format: "int64", Nullable: true}
	case "Float":
		return Schema{Type: "number", Nullable: true}
	}
	return Schema{Description: "Computed value", Nullable: true}
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
func generatePathItem(qcc langadapter.Language, item allow.Item, cfg Config,
	inputs core.OpenAPIInputs,
) (PathItem, bool) {
	a, ok := analyzeQuery(qcc, item, cfg, inputs)
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
				if param.Required {
					varsSchema.Required = append(varsSchema.Required, param.Name)
				}
			}
			sort.Strings(varsSchema.Required)
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
		description = "Primary key"
	}
	gqlType, _ := schema.ColumnScalarType(col.Type)
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
		// A json column holds any JSON value, a list as much as an
		// object.
		return Schema{Description: description}
	default:
		if description == "" {
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
