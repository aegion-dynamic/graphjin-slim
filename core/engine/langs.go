package engine

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/langadapter"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// graphqlLang adapts the current fused qcode compiler to the
// langadapter.Language contract. The Phase 4 extraction moves this into
// core/lang/graphql together with the parser; nothing else changes.
type graphqlLang struct {
	c  *qcode.Compiler
	gj *graphjinEngine // engine-level services (schema description)
}

// graphqlFactory registers the built-in language with the global
// registry so discovery and configuration can resolve "graphql".
type graphqlFactory struct{}

func (graphqlFactory) Name() string { return "graphql" }

func (graphqlFactory) New(c *qcode.Compiler) (langadapter.Language, error) {
	return graphqlLang{c: c}, nil
}

func (l graphqlLang) DescribeSchema(ns string) (json.RawMessage, error) {
	if l.gj == nil {
		return nil, fmt.Errorf("introspection unavailable: no engine bound")
	}
	return l.gj.getIntroResult()
}

// BuildChildQuery serializes cross-database child work as GraphQL source,
// the wire protocol today's remote side already consumes.
func (l graphqlLang) BuildChildQuery(sel *qcode.Select, selects []qcode.Select,
	fkCol sdata.DBColumn, parentID []byte,
) ([]byte, error) {
	return buildChildGraphQLQuery(sel, selects, fkCol, parentID), nil
}

func init() { langadapter.Register(graphqlFactory{}) }

func (l graphqlLang) Name() string { return "graphql" }

func (l graphqlLang) Compile(query []byte, vars map[string]json.RawMessage,
	opts langadapter.CompileOptions,
) (*qcode.QCode, error) {
	return l.c.Compile(query, vars, opts.Namespace)
}

// FastInfo extracts the operation kind and name without full compilation.
func (l graphqlLang) FastInfo(query []byte) (langadapter.Info, error) {
	h, err := graph.FastParseBytes(query)
	if err != nil {
		return langadapter.Info{}, err
	}
	return langadapter.Info{Operation: h.Operation, Name: h.Name}, nil
}

// languages returns the query languages available for this database,
// built lazily around its compilers. Safe for concurrent use.
func (db *dbContext) languages(gj *graphjinEngine) map[string]langadapter.Language {
	db.langsMu.Lock()
	defer db.langsMu.Unlock()
	if db.langs == nil {
		db.langs = map[string]langadapter.Language{}
		if db.qcodeCompiler != nil {
			// gj may be nil when a context is not yet bound to an engine;
			// such languages serve compilation only.
			db.langs["graphql"] = graphqlLang{c: db.qcodeCompiler, gj: gj}
		}
	}
	return db.langs
}

// GetLanguage resolves a query language for the named database. An empty
// dbName selects the primary database.
func (gj *graphjinEngine) GetLanguage(dbName, langName string) (langadapter.Language, error) {
	if dbName == "" {
		dbName = gj.defaultDB
	}
	ctx, ok := gj.GetDatabase(dbName)
	if !ok {
		return nil, fmt.Errorf("database not found: %s", dbName)
	}
	l, ok := ctx.languages(gj)[langName]
	if !ok {
		return nil, fmt.Errorf("database %s: no language %q (available: %v)",
			dbName, langName, gj.ListLanguages(dbName))
	}
	return l, nil
}

// ListLanguages lists query languages available on the named database.
// An empty dbName selects the primary database.
func (gj *graphjinEngine) ListLanguages(dbName string) []string {
	if dbName == "" {
		dbName = gj.defaultDB
	}
	ctx, ok := gj.GetDatabase(dbName)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(ctx.languages(gj)))
	for n := range ctx.languages(gj) {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ListLanguages lists the query languages available on the named
// database (an empty name selects the primary database).
func (g *GraphJin) ListLanguages(dbName string) ([]string, error) {
	gj, err := g.getEngine()
	if err != nil {
		return nil, err
	}
	return gj.ListLanguages(dbName), nil
}
