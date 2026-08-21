package engine

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/allow"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// OpenAPIInputs is a plain snapshot of the engine state required to
// generate an OpenAPI specification. The generator (typically the openapi
// module) owns all rendering logic; the engine only supplies data that
// cannot be observed from outside.
type OpenAPIInputs struct {
	// Queries are the saved named queries known to the engine's allow list.
	Queries []allow.Item

	// Databases maps database name to its discovered and finalized schema.
	Databases map[string]*sdata.DBSchema
}

// OpenAPISpecInputs returns the saved queries and per-database schemas
// needed to generate an OpenAPI specification. When ns is non-nil only
// queries in that namespace are returned; otherwise queries from every
// namespace are included. In production-security mode the allow list is the
// complete set of executable queries, so generated specs never expose more
// than production allows.
func (g *GraphJin) OpenAPISpecInputs(ns *string) (OpenAPIInputs, error) {
	if g == nil {
		return OpenAPIInputs{}, errEngineNotInitialized
	}
	gj, err := g.getEngine()
	if err != nil {
		return OpenAPIInputs{}, err
	}

	inputs := OpenAPIInputs{
		Databases: make(map[string]*sdata.DBSchema, len(gj.databases)),
	}
	for name, ctx := range gj.databases {
		if ctx.schema != nil {
			inputs.Databases[name] = ctx.schema
		}
	}

	if gj.allowList == nil {
		return inputs, nil
	}
	items, err := gj.allowList.ListAll()
	if err != nil {
		return OpenAPIInputs{}, err
	}
	for _, item := range items {
		if ns != nil && item.Namespace != *ns {
			continue
		}
		inputs.Queries = append(inputs.Queries, item)
	}
	return inputs, nil
}
