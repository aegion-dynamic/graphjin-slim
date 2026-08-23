package graphql

// Seam bindings: this file is everything core needs to know about the
// graphql frontend, expressed as langadapter capabilities. The frontend
// registers one descriptor at init time; no core package imports this
// module directly.

import (
	"encoding/json"
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/dbjoin"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/langadapter"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	schemapkg "github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// descriptor implements every registered capability for the language:
// compiler factory, schema description, SDL parsing.
type descriptor struct{}

func (descriptor) Name() string { return "graphql" }

// NewCompiler binds a per-database language instance to the given
// schema under the neutral configuration.
func (descriptor) NewCompiler(schema *sdata.DBSchema, cfg langadapter.CompileConfig) (langadapter.Language, error) {
	c := Config{
		Vars:                cfg.Vars,
		TConfig:             cfg.TConfig,
		DefaultLimit:        cfg.DefaultLimit,
		AnalyticsMode:       cfg.AnalyticsMode,
		DisableAgg:          cfg.DisableAgg,
		DisableFuncs:        cfg.DisableFuncs,
		EnableCamelcase:     cfg.EnableCamelcase,
		DBSchema:            cfg.DBSchema,
		EnableCacheTracking: cfg.EnableCacheTracking,
	}
	co, err := NewCompiler(schema, c)
	if err != nil {
		return nil, err
	}
	return Lang{c: co}, nil
}

// DescribeSchema renders the introspection document purely from its
// inputs — no engine back-reference is needed or permitted.
func (descriptor) DescribeSchema(opts langadapter.DescribeOptions) (json.RawMessage, error) {
	return schemapkg.BuildIntrospection(schemapkg.IntroOptions{
		CamelCase:  opts.CamelCase,
		DisableAgg: opts.DisableAgg,
		Schemas:    opts.Schemas,
	})
}

// ParseSchemaSDL parses GraphJin schema-definition text (the .schema
// file format) into neutral database metadata. An empty dbType falls
// back to the type declared in the file, then postgres.
func (descriptor) ParseSchemaSDL(b []byte, dbType string, blocklist []string) (*sdata.DBInfo, error) {
	return parseSchemaSDL(b, dbType, blocklist)
}

func parseSchemaSDL(b []byte, dbType string, blocklist []string) (*sdata.DBInfo, error) {
	ds, err := ParseSchema(b)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}
	if dbType == "" {
		dbType = ds.Type
	}
	if dbType == "" {
		dbType = "postgres"
	}
	schema := ds.Schema
	if schema == "" {
		schema = schemapkg.DefaultSchemaForDBType(dbType)
	}
	di := sdata.NewDBInfo(dbType, ds.Version, schema, "", ds.Columns, ds.Functions, blocklist)
	attachClusteringKeys(di, ds.ClusteringKeys, schema, dbType)
	return di, nil
}

// attachClusteringKeys assigns parsed @cluster directive data to the
// matching DBTable entries (ported from core/schema when DDL tooling
// stopped importing this frontend).
func attachClusteringKeys(di *sdata.DBInfo, clusters []TableCluster, defaultSchema, dbType string) {
	for _, ck := range clusters {
		schema := ck.Schema
		if schema == "" {
			schema = defaultSchema
		}
		if t, err := di.GetTable(schema, ck.Table); err == nil {
			t.ClusteringKeys = ck.Keys
		}
	}
}

// Lang is the per-database Language instance: a bound compiler plus the
// capability implementations the runtime discovers by assertion.
type Lang struct {
	c *Compiler
}

func (l Lang) Name() string { return "graphql" }

// Compile lowers GraphQL source into the qcode IR.
func (l Lang) Compile(query []byte, vars map[string]json.RawMessage,
	opts langadapter.CompileOptions,
) (*qcode.QCode, error) {
	return l.c.Compile(query, vars, opts.Namespace)
}

// FastInfo extracts the operation kind and name without full compilation.
func (l Lang) FastInfo(query []byte) (langadapter.Info, error) {
	h, err := graph.FastParseBytes(query)
	if err != nil {
		return langadapter.Info{}, err
	}
	return langadapter.Info{Operation: h.Operation, Name: h.Name}, nil
}

// BuildChildQuery serializes cross-database child work as GraphQL source,
// the wire protocol today's remote side already consumes.
func (l Lang) BuildChildQuery(sel *qcode.Select, selects []qcode.Select,
	fkCol sdata.DBColumn, parentID []byte,
) ([]byte, error) {
	return dbjoin.BuildChildGraphQLQuery(sel, selects, fkCol, parentID), nil
}

// QueryParameters extracts named input variables ($id = ID!) from query
// source so documentation generators never need the parser.
func (l Lang) QueryParameters(query []byte) ([]langadapter.Parameter, error) {
	op, err := graph.Parse(query)
	if err != nil {
		return nil, err
	}
	var params []langadapter.Parameter
	for _, varDef := range op.VarDef {
		typeName := "String"
		required := false
		if varDef.Val != nil {
			if t := trimSuffix(varDef.Val.Val, "!"); t != "" {
				typeName = t
			} else if varDef.Val.Name != "" {
				typeName = varDef.Val.Name
			}
			required = varDef.Val.Type == graph.NodeLabel &&
				len(varDef.Val.Children) > 0 &&
				varDef.Val.Children[0].Type == graph.NodeLabel
		}
		params = append(params, langadapter.Parameter{
			Name:     varDef.Name,
			Required: required,
			Type:     typeName,
		})
	}
	return params, nil
}

func trimSuffix(s, suf string) string { //nolint: unused when openapi ports land
	if len(s) >= len(suf) && s[len(s)-len(suf):] == suf {
		return s[:len(s)-len(suf)]
	}
	return s
}

func init() { langadapter.Register(descriptor{}) }
