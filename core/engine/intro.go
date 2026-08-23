package engine

import (
	"encoding/json"
	"sort"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/lang/graphql/valid"
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

	// Validator metadata comes from the frontend's registry so this
	// package never imports a query language directly.
	var validateFormats []string
	for k := range valid.Formats {
		validateFormats = append(validateFormats, k)
	}
	sort.Strings(validateFormats)

	valNames := make([]string, 0, len(valid.Validators))
	for name := range valid.Validators {
		valNames = append(valNames, name)
	}
	sort.Strings(valNames)
	validators := make([]schemapkg.ValidatorInfo, 0, len(valNames))
	for _, name := range valNames {
		v := valid.Validators[name]
		validators = append(validators, schemapkg.ValidatorInfo{
			Name: name,
			Type: v.Type,
			List: v.List,
		})
	}

	return schemapkg.BuildIntrospection(schemapkg.IntroOptions{
		CamelCase:       gj.conf.EnableCamelcase,
		DisableAgg:      gj.conf.DisableAgg,
		Schemas:         schemas,
		ValidateFormats: validateFormats,
		Validators:      validators,
	})
}
