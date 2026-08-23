package engine

import (
	"encoding/json"
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/langadapter"
	schemapkg "github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// Re-export introspection types used by tests and callers.
type (
	TypeRef             = schemapkg.TypeRef
	InputValue          = schemapkg.InputValue
	FieldObject         = schemapkg.FieldObject
	EnumValue           = schemapkg.EnumValue
	FullType            = schemapkg.FullType
	ShortFullType       = schemapkg.ShortFullType
	DirectiveType       = schemapkg.DirectiveType
	IntrospectionSchema = schemapkg.IntrospectionSchema
	IntroResult         = schemapkg.IntroResult
	Introspection       = schemapkg.Introspection
)

const (
	KIND_SCALAR      = schemapkg.KIND_SCALAR
	KIND_OBJECT      = schemapkg.KIND_OBJECT
	KIND_NONNULL     = schemapkg.KIND_NONNULL
	KIND_LIST        = schemapkg.KIND_LIST
	KIND_UNION       = schemapkg.KIND_UNION
	KIND_ENUM        = schemapkg.KIND_ENUM
	KIND_INPUT_OBJ   = schemapkg.KIND_INPUT_OBJ
	LOC_QUERY        = schemapkg.LOC_QUERY
	LOC_MUTATION     = schemapkg.LOC_MUTATION
	LOC_SUBSCRIPTION = schemapkg.LOC_SUBSCRIPTION
	LOC_FIELD        = schemapkg.LOC_FIELD
)

// introQuery returns the introspection query result for this engine.
func (gj *graphjinEngine) introQuery() (result json.RawMessage, err error) {
	schemas := make([]*sdata.DBSchema, 0, len(gj.databases))
	for _, dbName := range gj.sortedDatabaseNames() {
		ctx := gj.databases[dbName]
		if ctx == nil || ctx.schema == nil {
			continue
		}
		schemas = append(schemas, ctx.schema)
	}

	d, err := langadapter.Lookup(langadapter.DefaultLanguageName)
	if err != nil {
		return nil, err
	}
	df, ok := d.(langadapter.DescribeFactory)
	if !ok {
		return nil, fmt.Errorf("language %q does not provide schema description", langadapter.DefaultLanguageName)
	}
	return df.DescribeSchema(langadapter.DescribeOptions{
		Schemas:    schemas,
		CamelCase:  gj.conf.EnableCamelcase,
		DisableAgg: gj.conf.DisableAgg,
	})
}
