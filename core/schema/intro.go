package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/util"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/valid"
)

const (
	KIND_SCALAR      = "SCALAR"
	KIND_OBJECT      = "OBJECT"
	KIND_NONNULL     = "NON_NULL"
	KIND_LIST        = "LIST"
	KIND_UNION       = "UNION"
	KIND_ENUM        = "ENUM"
	KIND_INPUT_OBJ   = "INPUT_OBJECT"
	LOC_QUERY        = "QUERY"
	LOC_MUTATION     = "MUTATION"
	LOC_SUBSCRIPTION = "SUBSCRIPTION"
	LOC_FIELD        = "FIELD"

	SUFFIX_EXP      = "Expression"
	SUFFIX_LISTEXP  = "ListExpression"
	SUFFIX_INPUT    = "Input"
	SUFFIX_ORDER_BY = "OrderByInput"
	SUFFIX_WHERE    = "WhereInput"
	SUFFIX_ARGS     = "ArgsInput"
	SUFFIX_ENUM     = "Enum"
)

var (
	TYPE_STRING  = "String"
	TYPE_INT     = "Int"
	TYPE_BOOLEAN = "Boolean"
	TYPE_FLOAT   = "Float"
	TYPE_JSON    = "JSON"
)

type TypeRef struct {
	Kind   string   `json:"kind"`
	Name   *string  `json:"name"`
	OfType *TypeRef `json:"ofType"`
}

type InputValue struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Type         *TypeRef `json:"type"`
	DefaultValue *string  `json:"defaultValue"`
}

type FieldObject struct {
	Name              string       `json:"name"`
	Description       string       `json:"description"`
	Args              []InputValue `json:"args"`
	Type              *TypeRef     `json:"type"`
	IsDeprecated      bool         `json:"isDeprecated"`
	DeprecationReason *string      `json:"deprecationReason"`
}

type EnumValue struct {
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	IsDeprecated      bool    `json:"isDeprecated"`
	DeprecationReason *string `json:"deprecationReason"`
}

type FullType struct {
	Kind          string        `json:"kind"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Fields        []FieldObject `json:"fields"`
	InputFields   []InputValue  `json:"inputFields"`
	EnumValues    []EnumValue   `json:"enumValues"`
	Interfaces    []TypeRef     `json:"interfaces"`
	PossibleTypes []TypeRef     `json:"possibleTypes"`
}

type ShortFullType struct {
	Name string `json:"name"`
}

type DirectiveType struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Locations    []string     `json:"locations"`
	Args         []InputValue `json:"args"`
	IsRepeatable bool         `json:"isRepeatable"`
}

type IntrospectionSchema struct {
	Types            []FullType      `json:"types"`
	QueryType        *ShortFullType  `json:"queryType"`
	MutationType     *ShortFullType  `json:"mutationType"`
	SubscriptionType *ShortFullType  `json:"subscriptionType"`
	Directives       []DirectiveType `json:"directives"`
}

type IntroResult struct {
	Schema IntrospectionSchema `json:"__schema"`
}

// const singularSuffix = "ByID"

var stdTypes = []FullType{
	{
		Kind:        KIND_SCALAR,
		Name:        TYPE_BOOLEAN,
		Description: "The `Boolean` scalar type represents `true` or `false`",
	}, {
		Kind:        KIND_SCALAR,
		Name:        TYPE_FLOAT,
		Description: "The `Float` scalar type represents signed double-precision fractional values as specified by\n[IEEE 754](http://en.wikipedia.org/wiki/IEEE_floating_point).",
	}, {
		Kind:        KIND_SCALAR,
		Name:        TYPE_INT,
		Description: "The `Int` scalar type represents non-fractional signed whole numeric values. Int can represent\nvalues between -(2^31) and 2^31 - 1.\n",
		// Add Int expression after
	}, {
		Kind:        KIND_SCALAR,
		Name:        TYPE_STRING,
		Description: "The `String` scalar type represents textual data, represented as UTF-8 character sequences.\nThe String type is most often used by GraphQL to represent free-form human-readable text.\n",
	}, {
		Kind:        KIND_SCALAR,
		Name:        TYPE_JSON,
		Description: "The `JSON` scalar type represents json data",
	}, {
		Kind:       KIND_OBJECT,
		Name:       "Query",
		Interfaces: []TypeRef{},
		Fields:     []FieldObject{},
	}, {
		Kind:       KIND_OBJECT,
		Name:       "Subscription",
		Interfaces: []TypeRef{},
		Fields:     []FieldObject{},
	}, {
		Kind:       KIND_OBJECT,
		Name:       "Mutation",
		Interfaces: []TypeRef{},
		Fields:     []FieldObject{},
	}, {
		Kind: KIND_ENUM,
		Name: "FindSearchInput",
		EnumValues: []EnumValue{{
			Name:        "children",
			Description: "Children of parent row",
		}, {
			Name:        "parents",
			Description: "Parents of current row",
		}},
	}, {
		Kind:        "ENUM",
		Name:        "OrderDirection",
		Description: "Result ordering types",
		EnumValues: []EnumValue{{
			Name:        "asc",
			Description: "Ascending order",
		}, {
			Name:        "desc",
			Description: "Descending order",
		}, {
			Name:        "asc_nulls_first",
			Description: "Ascending nulls first order",
		}, {
			Name:        "desc_nulls_first",
			Description: "Descending nulls first order",
		}, {
			Name:        "asc_nulls_last",
			Description: "Ascending nulls last order",
		}, {
			Name:        "desc_nulls_last",
			Description: "Descending nulls last order",
		}},
	}, {
		Kind:        KIND_SCALAR,
		Name:        "ID",
		Description: "The `ID` scalar type represents a unique identifier, often used to refetch an object or as key for a cache.\nThe ID type appears in a JSON response as a String; however, it is not intended to be human-readable.\nWhen expected as an input type, any string (such as `\"4\"`) or integer (such as `4`) input value will be accepted\nas an ID.\n",
		// Add IDException after
	}, {
		Kind:        KIND_SCALAR,
		Name:        "Cursor",
		Description: "A cursor is an encoded string use for pagination",
	},
}

type Introspection struct {
	schema      *sdata.DBSchema
	camelCase   bool
	disableAgg  bool
	types       map[string]FullType
	enumValues  map[string]EnumValue
	inputValues map[string]InputValue
	result      IntroResult
}

// IntroOptions configures GraphQL introspection generation from DB schemas.
type IntroOptions struct {
	CamelCase  bool
	DisableAgg bool
	// Schemas is the ordered list of database schemas to include.
	Schemas []*sdata.DBSchema
}

// BuildIntrospection builds the GraphQL introspection JSON for the given schemas.
func BuildIntrospection(opts IntroOptions) (result json.RawMessage, err error) {
	in := Introspection{
		camelCase:   opts.CamelCase,
		disableAgg:  opts.DisableAgg,
		types:       make(map[string]FullType),
		enumValues:  make(map[string]EnumValue),
		inputValues: make(map[string]InputValue),
	}

	in.result.Schema = IntrospectionSchema{
		QueryType:        &ShortFullType{Name: "Query"},
		SubscriptionType: &ShortFullType{Name: "Subscription"},
		MutationType:     &ShortFullType{Name: "Mutation"},
	}

	for _, v := range stdTypes {
		in.addType(v)
	}

	v := append(expAll, expScalar...)
	in.addExpTypes(v, "ID", newTypeRef("", "ID", nil))
	in.addExpTypes(v, "String", newTypeRef("", "String", nil))
	in.addExpTypes(v, "Int", newTypeRef("", "Int", nil))
	in.addExpTypes(v, "Boolean", newTypeRef("", "Boolean", nil))
	in.addExpTypes(v, "Float", newTypeRef("", "Float", nil))
	in.addExpTypes(v, "ID", newTypeRef("", "ID", nil))
	in.addExpTypes(v, "String", newTypeRef("", "String", nil))
	in.addExpTypes(v, "Int", newTypeRef("", "Int", nil))
	in.addExpTypes(v, "Boolean", newTypeRef("", "Boolean", nil))
	in.addExpTypes(v, "Float", newTypeRef("", "Float", nil))

	v = append(expAll, expList...)
	in.addExpTypes(v, "StringList", newTypeRef("", "String", nil))
	in.addExpTypes(v, "IntList", newTypeRef("", "Int", nil))
	in.addExpTypes(v, "BooleanList", newTypeRef("", "Boolean", nil))
	in.addExpTypes(v, "FloatList", newTypeRef("", "Float", nil))

	v = append(expAll, expJSON...)
	in.addExpTypes(v, "JSON", newTypeRef("", "String", nil))

	for _, sch := range opts.Schemas {
		if sch == nil {
			continue
		}
		in.schema = sch

		for _, t := range sch.GetTables() {
			if t.Blocked {
				continue
			}
			in.addToTablesEnum(t)
		}

		aliases := sch.GetAliases()
		aliasNames := make([]string, 0, len(aliases))
		for name := range aliases {
			aliasNames = append(aliasNames, name)
		}
		sort.Strings(aliasNames)
		for _, alias := range aliasNames {
			t := aliases[alias]
			if err = in.addTable(t, alias); err != nil {
				return
			}
		}

		for _, t := range sch.GetTables() {
			if err = in.addTable(t, ""); err != nil {
				return
			}
		}
	}

	in.finalizeTablesEnum()

	for _, dt := range dirTypes {
		in.addDirType(dt)
	}
	in.addDirValidateType()

	typeNames := make([]string, 0, len(in.types))
	for name := range in.types {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)
	for _, name := range typeNames {
		in.result.Schema.Types = append(in.result.Schema.Types, in.types[name])
	}

	result, err = json.Marshal(in.result)
	return
}

// addTable adds a table to the introspection schema
func (in *Introspection) addTable(table sdata.DBTable, alias string) (err error) {
	if table.Blocked {
		return
	}
	if len(table.Columns) == 0 && len(table.Args) == 0 {
		return
	}
	if table.Type == "remote" {
		return in.addRemoteTable(table, alias)
	}
	var ftQS FullType

	// add table type to query and subscription
	if ftQS, err = in.addTableType(table, alias); err != nil {
		return
	}
	in.addTypeTo("Query", ftQS)
	in.addCursorFieldTo("Query", ftQS.Name)
	in.addTypeTo("Subscription", ftQS)
	in.addCursorFieldTo("Subscription", ftQS.Name)

	var ftM FullType

	// add table type to mutation
	if ftM, err = in.addInputType(table, ftQS); err != nil {
		return
	}
	in.addTypeTo("Mutation", ftM)

	// add tableByID type to query and subscription
	var ftQSByID FullType

	if ftQSByID, err = in.addTableType(table, alias); err != nil {
		return
	}

	ftQSByID.Name += "ByID"
	ftQSByID.addOrReplaceArg("id", newTypeRef(KIND_NONNULL, "", newTypeRef("", "ID", nil)))
	in.addType(ftQSByID)
	in.addTypeTo("Query", ftQSByID)
	in.addTypeTo("Subscription", ftQSByID)

	return
}

// addRemoteTable surfaces a synthetic remote table (OpenAPI virtual)
// as a single Query field. Skips the real-table machinery (mutations,
// ByID, where/orderBy), which doesn't apply to remotes.
func (in *Introspection) addRemoteTable(table sdata.DBTable, alias string) error {
	ft := FullType{
		Kind:        KIND_OBJECT,
		InputFields: []InputValue{},
		Interfaces:  []TypeRef{},
	}
	name := table.Name
	if alias != "" {
		name = alias
	}
	ft.Name = in.getName(name)
	ft.Description = table.Comment

	for _, c := range table.Columns {
		if c.Blocked {
			continue
		}
		f, err := in.getColumnField(c)
		if err != nil {
			return err
		}
		ft.Fields = append(ft.Fields, f)
	}

	for _, a := range table.Args {
		base := remoteArgType(a.Type)
		if a.NotNull {
			base = newTypeRef(KIND_NONNULL, "", base)
		}
		ft.addArg(a.Name, base)
	}

	in.addType(ft)
	in.addTypeTo("Query", ft)
	return nil
}

func remoteArgType(t string) *TypeRef {
	switch t {
	case "bigint", "int", "integer", "smallint":
		return newTypeRef("", "Int", nil)
	case "boolean", "bool":
		return newTypeRef("", "Boolean", nil)
	case "numeric", "float", "double", "real", "double precision":
		return newTypeRef("", "Float", nil)
	default:
		return newTypeRef("", "String", nil)
	}
}

// addTypeTo adds a type to the introspection schema
func (in *Introspection) addTypeTo(op string, ft FullType) {
	qt := in.types[op]
	qt.Fields = append(qt.Fields, FieldObject{
		Name:        ft.Name,
		Description: ft.Description,
		Args:        ft.InputFields,
		Type:        newTypeRef("", ft.Name, nil),
	})
	in.types[op] = qt
}

func (in *Introspection) addCursorFieldTo(op, name string) {
	ft := in.types[op]
	addCursorField(&ft, name)
	in.types[op] = ft
}

// getName returns the name of the type
func (in *Introspection) getName(name string) string {
	if in.camelCase {
		return util.ToCamel(name)
	} else {
		return name
	}
}

// addExpTypes adds the expression types to the introspection schema
func (in *Introspection) addExpTypes(exps []exp, name string, rt *TypeRef) {
	ft := FullType{
		Kind:        KIND_INPUT_OBJ,
		Name:        (name + SUFFIX_EXP),
		InputFields: []InputValue{},
		Interfaces:  []TypeRef{},
	}

	for _, ex := range exps {
		rtVal := rt
		if ex.etype != "" {
			rtVal = newTypeRef("", ex.etype, nil)
		}
		ft.InputFields = append(ft.InputFields, InputValue{
			Name:        ex.name,
			Description: ex.desc,
			Type:        rtVal,
		})
	}
	in.addType(ft)
}

// addTableType adds a table type to the introspection schema
func (in *Introspection) addTableType(t sdata.DBTable, alias string) (ft FullType, err error) {
	return in.addTableTypeWithDepth(t, alias, 0)
}

// addTableTypeWithDepth adds a table type with depth to the introspection schema
func (in *Introspection) addTableTypeWithDepth(
	table sdata.DBTable, alias string, depth int,
) (ft FullType, err error) {
	ft = FullType{
		Kind:        KIND_OBJECT,
		InputFields: []InputValue{},
		Interfaces:  []TypeRef{},
	}

	name := table.Name
	if alias != "" {
		name = alias
	}
	name = in.getName(name)

	ft.Name = name
	ft.Description = table.Comment

	var hasSearch bool
	var hasRecursive bool

	if err = in.addColumnsEnumType(table); err != nil {
		return
	}

	functions := in.schema.GetFunctions()
	fnNames := make([]string, 0, len(functions))
	for name := range functions {
		fnNames = append(fnNames, name)
	}
	sort.Strings(fnNames)

	for _, name := range fnNames {
		ty := in.addArgsType(table, functions[name])
		in.addType(ty)
	}

	for _, c := range table.Columns {
		if c.Blocked {
			continue
		}
		if c.FullText {
			hasSearch = true
		}
		if c.FKRecursive {
			hasRecursive = true
		}
		var f1 FieldObject
		f1, err = in.getColumnField(c)
		if err != nil {
			return
		}
		ft.Fields = append(ft.Fields, f1)
	}

	for _, name := range fnNames {
		f1 := in.getFunctionField(table, functions[name])
		ft.Fields = append(ft.Fields, f1)
	}

	in.addAggregateFields(table, &ft)

	relNodes1, err := in.schema.GetFirstDegree(table)
	if err != nil {
		return
	}

	relNodes2, err := in.schema.GetSecondDegree(table)
	if err != nil {
		return
	}

	for _, relNode := range append(relNodes1, relNodes2...) {
		var f FieldObject
		var skip bool
		f, skip, err = in.getTableField(relNode)
		if err != nil {
			return
		}
		if !skip {
			ft.Fields = append(ft.Fields, f)
			if relationshipSupportsCursor(relNode.Type, f.Type) {
				addCursorField(&ft, f.Name)
			}
		}
	}

	ft.addArg("id", newTypeRef("", "ID", nil))
	ft.addArg("limit", newTypeRef("", "Int", nil))
	ft.addArg("offset", newTypeRef("", "Int", nil))
	ft.addArg("distinctOn", newTypeRef("LIST", "", newTypeRef("", "String", nil)))
	ft.addArg("first", newTypeRef("", "Int", nil))
	ft.addArg("last", newTypeRef("", "Int", nil))
	ft.addArg("after", newTypeRef("", "Cursor", nil))
	ft.addArg("before", newTypeRef("", "Cursor", nil))

	in.addOrderByType(table, &ft)
	in.addWhereType(table, &ft)
	in.addTableArgsType(table, &ft)

	if hasSearch {
		ft.addArg("search", newTypeRef("", "String", nil))
	}

	if depth > 1 {
		return
	}
	if depth > 0 {
		ft.addArg("find", newTypeRef("", "FindSearchInput", nil))
	}

	in.addType(ft)

	if hasRecursive {
		_, err = in.addTableTypeWithDepth(table,
			(name + "Recursive"),
			(depth + 1))
	}
	return
}

// addColumnsEnumType adds an enum type for the columns of the table
func (in *Introspection) addColumnsEnumType(t sdata.DBTable) (err error) {
	tableName := in.getName(t.Name)
	ft := FullType{
		Kind:        KIND_ENUM,
		Name:        (t.Name + "Columns" + SUFFIX_ENUM),
		Description: fmt.Sprintf("Table columns for '%s'", tableName),
	}
	for _, c := range t.Columns {
		if c.Blocked {
			continue
		}
		ft.EnumValues = append(ft.EnumValues, EnumValue{
			Name:        in.getName(c.Name),
			Description: c.Comment,
		})
	}
	in.addType(ft)
	return
}

// addToTablesEnum accumulates a table into the tables enum (called per-database).
func (in *Introspection) addToTablesEnum(t sdata.DBTable) {
	in.enumValues[in.getName(t.Name)] = EnumValue{
		Name:        in.getName(t.Name),
		Description: t.Comment,
	}
}

// finalizeTablesEnum writes the accumulated tables enum type.
func (in *Introspection) finalizeTablesEnum() {
	ft := FullType{
		Kind:        KIND_ENUM,
		Name:        ("tables" + SUFFIX_ENUM),
		Description: "All available tables",
	}
	evNames := make([]string, 0, len(in.enumValues))
	for name := range in.enumValues {
		evNames = append(evNames, name)
	}
	sort.Strings(evNames)
	for _, name := range evNames {
		ft.EnumValues = append(ft.EnumValues, in.enumValues[name])
	}
	in.addType(ft)
}

// addOrderByType adds an order by type to the introspection schema
func (in *Introspection) addOrderByType(t sdata.DBTable, ft *FullType) {
	ty := FullType{
		Kind: KIND_INPUT_OBJ,
		Name: (t.Name + SUFFIX_ORDER_BY),
	}
	for _, c := range t.Columns {
		if c.Blocked {
			continue
		}
		ty.InputFields = append(ty.InputFields, InputValue{
			Name:        in.getName(c.Name),
			Description: c.Comment,
			Type:        newTypeRef("", "OrderDirection", nil),
		})
	}
	in.addType(ty)
	ft.addArg("orderBy", newTypeRef("", (t.Name+SUFFIX_ORDER_BY), nil))
}

// addWhereType adds a where type to the introspection schema
func (in *Introspection) addWhereType(table sdata.DBTable, ft *FullType) {
	tablename := (table.Name + SUFFIX_WHERE)
	ty := FullType{
		Kind: "INPUT_OBJECT",
		Name: tablename,
		InputFields: []InputValue{
			{Name: "and", Type: newTypeRef("", tablename, nil)},
			{Name: "_and", Type: newTypeRef("", tablename, nil)},
			{Name: "or", Type: newTypeRef("", tablename, nil)},
			{Name: "_or", Type: newTypeRef("", tablename, nil)},
			{Name: "not", Type: newTypeRef("", tablename, nil)},
			{Name: "_not", Type: newTypeRef("", tablename, nil)},
		},
	}
	for _, c := range table.Columns {
		if c.Blocked {
			continue
		}
		ft := getTypeFromColumn(c)
		if c.Array {
			ft += SUFFIX_LISTEXP
		} else {
			ft += SUFFIX_EXP
		}
		ty.InputFields = append(ty.InputFields, InputValue{
			Name:        in.getName(c.Name),
			Description: c.Comment,
			Type:        newTypeRef("", ft, nil),
		})
	}
	in.addType(ty)
	ft.addArg("where", newTypeRef("", ty.Name, nil))
}

func (in *Introspection) addInputType(table sdata.DBTable, ft FullType) (retFT FullType, err error) {
	// upsert
	ty := FullType{
		Kind:        "INPUT_OBJECT",
		Name:        ("upsert" + table.Name + SUFFIX_INPUT),
		InputFields: []InputValue{},
	}
	for _, c := range table.Columns {
		if c.Blocked {
			continue
		}
		ft1 := getTypeFromColumn(c)
		ty.InputFields = append(ty.InputFields, InputValue{
			Name:        in.getName(c.Name),
			Description: c.Comment,
			Type:        newTypeRef("", ft1, nil),
		})
	}
	in.addType(ty)
	ft.addArg("upsert", newTypeRef("", ty.Name, nil))

	// insert
	relNodes1, err := in.schema.GetFirstDegree(table)
	if err != nil {
		return
	}
	relNodes2, err := in.schema.GetSecondDegree(table)
	if err != nil {
		return
	}
	allNodes := append(relNodes1, relNodes2...)
	fieldLen := len(ty.InputFields)

	ty.Name = ("insert" + table.Name + SUFFIX_INPUT)
	for _, relNode := range allNodes {
		t1 := relNode.Table
		if relNode.Type == sdata.RelRemote ||
			relNode.Type == sdata.RelPolymorphic ||
			relNode.Type == sdata.RelEmbedded {
			continue
		}
		ty.InputFields = append(ty.InputFields, InputValue{
			Name:        in.getName(t1.Name),
			Description: t1.Comment,
			Type:        newTypeRef("", ("insert" + t1.Name + SUFFIX_INPUT), nil),
		})
	}
	in.addType(ty)
	ft.addArg("insert", newTypeRef("", ty.Name, nil))

	// update
	ty.Name = ("update" + table.Name + SUFFIX_INPUT)
	i := 0
	for _, relNode := range allNodes {
		t1 := relNode.Table
		if relNode.Type == sdata.RelRemote ||
			relNode.Type == sdata.RelPolymorphic ||
			relNode.Type == sdata.RelEmbedded {
			continue
		}
		ty.InputFields[(fieldLen + i)] = InputValue{
			Name:        in.getName(t1.Name),
			Description: t1.Comment,
			Type:        newTypeRef("", ("update" + t1.Name + SUFFIX_INPUT), nil),
		}
		i++
	}
	description1 := fmt.Sprintf("Connect to rows in table '%s' that match the expression", in.getName(table.Name))
	ty.InputFields = append(ty.InputFields, InputValue{
		Name:        "connect",
		Description: description1,
		Type:        newTypeRef("", (table.Name + SUFFIX_WHERE), nil),
	})
	description2 := fmt.Sprintf("Disconnect from rows in table '%s' that match the expression", in.getName(table.Name))
	ty.InputFields = append(ty.InputFields, InputValue{
		Name:        "disconnect",
		Description: description2,
		Type:        newTypeRef("", (table.Name + SUFFIX_WHERE), nil),
	})
	desciption3 := fmt.Sprintf("Update rows in table '%s' that match the expression", in.getName(table.Name))
	ty.InputFields = append(ty.InputFields, InputValue{
		Name:        "where",
		Description: desciption3,
		Type:        newTypeRef("", (table.Name + SUFFIX_WHERE), nil),
	})
	in.addType(ty)
	ft.addArg("update", newTypeRef("", ty.Name, nil))

	// delete
	ft.addArg("delete", newTypeRef("", TYPE_BOOLEAN, nil))
	retFT = ft
	return
}

// addTableArgsType adds the table arguments type to the introspection schema
func (in *Introspection) addTableArgsType(table sdata.DBTable, ft *FullType) {
	if table.Type != "function" {
		return
	}
	ty := in.addArgsType(table, table.Func)
	in.addType(ty)
	ft.addArg("args", newTypeRef("", ty.Name, nil))
}

// addArgsType adds the arguments type to the introspection schema
func (in *Introspection) addArgsType(table sdata.DBTable, fn sdata.DBFunction) (ft FullType) {
	ft = FullType{
		Kind: "INPUT_OBJECT",
		Name: (table.Name + fn.Name + SUFFIX_ARGS),
	}
	for _, fi := range fn.Inputs {
		var tr *TypeRef
		if fn.Agg {
			tr = newTypeRef("", (table.Name + "Columns" + SUFFIX_ENUM), nil)
		} else {
			tn, list := getType(fi.Type)
			if tn == "" {
				tn = "String"
			}
			tr = newTypeRef("", tn, nil)
			if list {
				tr = newTypeRef(KIND_LIST, "", tr)
			}
		}

		fname := in.getName(fi.Name)
		if fname == "" {
			fname = "_" + strconv.Itoa(fi.ID)
		}
		ft.InputFields = append(ft.InputFields, InputValue{
			Name: fname,
			Type: tr,
		})
	}
	return
}

// getColumnField returns the field object for the given column
func (in *Introspection) getColumnField(column sdata.DBColumn) (field FieldObject, err error) {
	field.Args = []InputValue{}
	field.Name = in.getName(column.Name)
	typeValue := newTypeRef("", "String", nil)

	if v, ok := in.types[getTypeFromColumn(column)]; ok {
		typeValue.Name = &v.Name
		typeValue.Kind = v.Kind
	}

	if column.Array {
		typeValue = newTypeRef(KIND_LIST, "", typeValue)
	}

	if column.NotNull {
		typeValue = newTypeRef(KIND_NONNULL, "", typeValue)
	}

	field.Type = typeValue

	field.Args = append(field.Args, InputValue{
		Name: "includeIf", Type: newTypeRef("", (column.Table + SUFFIX_WHERE), nil),
	})

	field.Args = append(field.Args, InputValue{
		Name: "skipIf", Type: newTypeRef("", (column.Table + SUFFIX_WHERE), nil),
	})

	return
}

// getFunctionField returns the field object for the given function
func (in *Introspection) getFunctionField(t sdata.DBTable, fn sdata.DBFunction) (f FieldObject) {
	f.Name = in.getName(fn.Name)
	f.Args = []InputValue{}
	ty, list := getType(fn.Type)
	f.Type = newTypeRef("", ty, nil)
	if list {
		f.Type = newTypeRef(KIND_LIST, "", f.Type)
	}

	if len(fn.Inputs) != 0 {
		typeName := (t.Name + fn.Name + SUFFIX_ARGS)
		argsArg := InputValue{Name: "args", Type: newTypeRef("", typeName, nil)}
		f.Args = append(f.Args, argsArg)
	}

	f.Args = append(f.Args, InputValue{
		Name: "includeIf", Type: newTypeRef("", (t.Name + SUFFIX_WHERE), nil),
	})

	f.Args = append(f.Args, InputValue{
		Name: "skipIf", Type: newTypeRef("", (t.Name + SUFFIX_WHERE), nil),
	})
	return
}

func (in *Introspection) addAggregateFields(table sdata.DBTable, ft *FullType) {
	if in.disableAgg {
		return
	}

	for _, c := range table.Columns {
		if c.Blocked {
			continue
		}

		ft.addFieldIfAbsent(in.getAggregateField(table, c, "count"))

		if !isIntroNumericColumn(c) {
			continue
		}

		ft.addFieldIfAbsent(in.getAggregateField(table, c, "sum"))
		ft.addFieldIfAbsent(in.getAggregateField(table, c, "avg"))
		ft.addFieldIfAbsent(in.getAggregateField(table, c, "min"))
		ft.addFieldIfAbsent(in.getAggregateField(table, c, "max"))
	}
}

func (in *Introspection) getAggregateField(table sdata.DBTable, col sdata.DBColumn, fn string) FieldObject {
	return FieldObject{
		Name: in.getName(fn + "_" + col.Name),
		Args: []InputValue{{
			Name: "includeIf", Type: newTypeRef("", (table.Name + SUFFIX_WHERE), nil),
		}, {
			Name: "skipIf", Type: newTypeRef("", (table.Name + SUFFIX_WHERE), nil),
		}},
		Type: aggregateFieldType(fn, col),
	}
}

func aggregateFieldType(fn string, col sdata.DBColumn) *TypeRef {
	switch fn {
	case "count":
		return newTypeRef("", TYPE_INT, nil)
	case "avg":
		return newTypeRef("", TYPE_FLOAT, nil)
	default:
		return newTypeRef("", introNumericType(col.Type), nil)
	}
}

func isIntroNumericColumn(col sdata.DBColumn) bool {
	if col.Array {
		return false
	}

	raw := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(col.Type)), " ", "")
	if strings.Contains(raw, "[]") || strings.Contains(raw, "array") {
		return false
	}
	if isIntroBooleanColumnType(introTrimColumnTypeModifier(raw)) {
		return false
	}

	switch introBaseColumnType(col.Type) {
	case "int", "int2", "int4", "int8",
		"integer", "bigint", "smallint", "tinyint", "mediumint",
		"serial", "smallserial", "bigserial",
		"decimal", "dec", "numeric", "number",
		"real", "float", "float4", "float8",
		"double", "doubleprecision", "money":
		return true
	default:
		return false
	}
}

func introNumericType(t string) string {
	switch introBaseColumnType(t) {
	case "int", "int2", "int4", "int8",
		"integer", "bigint", "smallint", "tinyint", "mediumint",
		"serial", "smallserial", "bigserial":
		return TYPE_INT
	default:
		return TYPE_FLOAT
	}
}

func introBaseColumnType(t string) string {
	t = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(t)), " ", "")
	if i := strings.IndexAny(t, "(["); i != -1 {
		t = t[:i]
	}
	return introTrimColumnTypeModifier(t)
}

func isIntroBooleanColumnType(t string) bool {
	switch t {
	case "bool", "boolean", "bit", "tinyint(1)", "number(1)", "number(1,0)":
		return true
	default:
		return false
	}
}

func introTrimColumnTypeModifier(t string) string {
	t = strings.TrimSuffix(t, "unsigned")
	return strings.TrimSuffix(t, "signed")
}

func relationshipSupportsCursor(relType sdata.RelType, tr *TypeRef) bool {
	if tr != nil && tr.Kind == KIND_LIST {
		return true
	}
	return relType == sdata.RelOneToMany
}

// getTableField returns the field object for the given table
func (in *Introspection) getTableField(relNode sdata.RelNode) (
	f FieldObject, skip bool, err error,
) {
	f.Args = []InputValue{}
	f.Name = in.getName(relNode.Name)

	tn := in.getName(relNode.Table.Name)
	if _, ok := in.types[tn]; !ok && relNode.Type != sdata.RelRecursive {
		skip = true
		return
	}

	switch relNode.Type {
	case sdata.RelOneToOne:
		f.Type = newTypeRef(KIND_LIST, "", newTypeRef("", tn, nil))
	case sdata.RelRecursive:
		tn += "Recursive"
		f.Type = newTypeRef(KIND_LIST, "", newTypeRef("", tn, nil))
	default:
		f.Type = newTypeRef("", tn, nil)
	}
	return
}

// addDirType adds a directive type to the introspection schema
func (in *Introspection) addDirType(dt dir) {
	d := DirectiveType{
		Name:         dt.name,
		Description:  dt.desc,
		Locations:    dt.locs,
		IsRepeatable: dt.repeat,
	}
	for _, a := range dt.args {
		d.Args = append(d.Args, InputValue{
			Name:        a.name,
			Description: a.desc,
			Type:        newTypeRef("", a.atype, nil),
		})
	}
	if len(dt.args) == 0 {
		d.Args = []InputValue{}
	}
	in.result.Schema.Directives = append(in.result.Schema.Directives, d)
}

// addDirValidateType adds a validate directive type to the introspection schema
func (in *Introspection) addDirValidateType() {
	ft := FullType{
		Kind:        KIND_ENUM,
		Name:        ("validateFormat" + SUFFIX_ENUM),
		Description: "Various formats supported by @validate",
	}
	fmtNames := make([]string, 0, len(valid.Formats))
	for k := range valid.Formats {
		fmtNames = append(fmtNames, k)
	}
	sort.Strings(fmtNames)
	for _, k := range fmtNames {
		ft.EnumValues = append(ft.EnumValues, EnumValue{
			Name: k,
		})
	}
	in.addType(ft)

	d := DirectiveType{
		Name:         "validate",
		Description:  "Add a validation for input variables",
		Locations:    []string{LOC_QUERY, LOC_MUTATION, LOC_SUBSCRIPTION},
		IsRepeatable: true,
	}
	d.Args = append(d.Args, InputValue{
		Name:        "variable",
		Description: "Variable to add the validation on",
		Type:        newTypeRef(KIND_NONNULL, "", newTypeRef("", "String", nil)),
	})
	valNames := make([]string, 0, len(valid.Validators))
	for k := range valid.Validators {
		valNames = append(valNames, k)
	}
	sort.Strings(valNames)
	for _, k := range valNames {
		v := valid.Validators[k]
		if v.Type == "" {
			continue
		}
		var ty *TypeRef
		if v.List {
			ty = newTypeRef(KIND_LIST, "", newTypeRef("", v.Type, nil))
		} else {
			ty = newTypeRef("", v.Type, nil)
		}
		d.Args = append(d.Args, InputValue{
			Name:        k,
			Description: v.Description,
			Type:        ty,
		})
	}
	in.result.Schema.Directives = append(in.result.Schema.Directives, d)
}

func (ft *FullType) addFieldIfAbsent(field FieldObject) {
	for _, f := range ft.Fields {
		if f.Name == field.Name {
			return
		}
	}
	ft.Fields = append(ft.Fields, field)
}

func addCursorField(ft *FullType, name string) {
	ft.addFieldIfAbsent(FieldObject{
		Name: name + "_cursor",
		Args: []InputValue{},
		Type: newTypeRef("", "Cursor", nil),
	})
}

// addArg adds an argument to the full type
func (ft *FullType) addArg(name string, tr *TypeRef) {
	ft.InputFields = append(ft.InputFields, InputValue{
		Name: name,
		Type: tr,
	})
}

// addOrReplaceArg adds or replaces an argument to the full type
func (ft *FullType) addOrReplaceArg(name string, tr *TypeRef) {
	for i, a := range ft.InputFields {
		if a.Name == name {
			ft.InputFields[i].Type = tr
			return
		}
	}
	ft.InputFields = append(ft.InputFields, InputValue{
		Name: name,
		Type: tr,
	})
}

// addType adds a type to the introspection schema
func (in *Introspection) addType(ft FullType) {
	in.types[ft.Name] = ft
}

func newTypeRef(kind, name string, tr *TypeRef) *TypeRef {
	if name == "" {
		return &TypeRef{Kind: kind, Name: nil, OfType: tr}
	}
	return &TypeRef{Kind: kind, Name: &name, OfType: tr}
}

// Returns the type of the given column. Returns ID if column is the primary key
func getTypeFromColumn(col sdata.DBColumn) (gqlType string) {
	if col.PrimaryKey {
		gqlType = "ID"
		return
	}
	gqlType, _ = getType(col.Type)
	return
}

// ColumnGraphQLType returns the GraphQL scalar name for a DB column type.
func ColumnGraphQLType(t string) (gqlType string, list bool) {
	return getType(t)
}

// Returns the GraphQL type for the given column type
func getType(t string) (gqlType string, list bool) {
	if i := strings.IndexRune(t, '('); i != -1 {
		t = t[:i]
	}
	if i := strings.IndexRune(t, '['); i != -1 {
		list = true
		t = t[:i]
	}
	if v, ok := dbTypes[t]; ok {
		gqlType = v
	} else if t == "json" || t == "jsonb" {
		gqlType = "JSON"
	} else {
		gqlType = "String"
	}
	return
}

var dbTypes map[string]string = map[string]string{
	"timestamp without time zone": "String",
	"character varying":           "String",
	"text":                        "String",
	"smallint":                    "Int",
	"integer":                     "Int",
	"bigint":                      "Int",
	"smallserial":                 "Int",
	"serial":                      "Int",
	"bigserial":                   "Int",
	"decimal":                     "Float",
	"numeric":                     "Float",
	"number":                      "Float",
	"real":                        "Float",
	"double precision":            "Float",
	"double":                      "Float",
	"money":                       "Float",
	"boolean":                     "Boolean",
	"varchar":                     "String",
	"timestamp_ntz":               "String",
	"timestamp_ltz":               "String",
	"timestamp_tz":                "String",
}

type dirArg struct {
	name  string
	desc  string
	atype string
}

type dir struct {
	name   string
	desc   string
	locs   []string
	args   []dirArg
	repeat bool
}

var dirTypes []dir = []dir{
	{
		name: "cacheControl",
		desc: "Set the cache-control header to be passed back with the query result",
		locs: []string{LOC_QUERY, LOC_MUTATION, LOC_SUBSCRIPTION},
		args: []dirArg{{
			name:  "maxAge",
			desc:  "The maximum amount of time (in seconds) a resource is considered fresh",
			atype: "Int",
		}, {
			name:  "scope",
			desc:  "Set to 'public' when any cache can store the data and 'private' when only the browser cache should",
			atype: "String",
		}},
	},
	{
		name: "skip",
		desc: "Skip field if defined condition is met",
		locs: []string{LOC_FIELD},
		args: []dirArg{{
			name:  "if",
			desc:  "If a variable is true",
			atype: "Boolean",
		}, {
			name:  "ifVar",
			desc:  "If a variable is true",
			atype: "String",
		}},
	},
	{
		name: "include",
		desc: "Include field if defined condition is met",
		locs: []string{LOC_FIELD},
		args: []dirArg{{
			name:  "if",
			desc:  "If a variable is true",
			atype: "Boolean",
		}, {
			name:  "ifVar",
			desc:  "If a variable is true",
			atype: "String",
		}},
	},
	{
		name: "schema",
		desc: "Specify database schema to use (Postgres specific)",
		locs: []string{LOC_FIELD},
		args: []dirArg{{
			name:  "name",
			desc:  "Name of schema",
			atype: "String",
		}},
	},
	{
		name: "notRelated",
		desc: "Treat this selector as if it were a top-level selector with no relation to its parent",
		locs: []string{LOC_FIELD},
	},
	{
		name: "through",
		desc: "use the specified table as a join-table to connect this field and it's parent",
		locs: []string{LOC_FIELD},
		args: []dirArg{{
			name:  "table",
			desc:  "Table name",
			atype: "tables" + SUFFIX_ENUM,
		}},
	},
}

type exp struct {
	name  string
	desc  string
	etype string
}

const (
	likeDesc       = "Value matching pattern where '%' represents zero or more characters and '_' represents a single character. Eg. '_r%' finds values having 'r' in second position"
	notLikeDesc    = "Value not matching pattern where '%' represents zero or more characters and '_' represents a single character. Eg. '_r%' finds values not having 'r' in second position"
	iLikeDesc      = "Value matching (case-insensitive) pattern where '%' represents zero or more characters and '_' represents a single character. Eg. '_r%' finds values having 'r' in second position"
	notILikeDesc   = "Value not matching (case-insensitive) pattern where '%' represents zero or more characters and '_' represents a single character. Eg. '_r%' finds values not having 'r' in second position"
	similarDesc    = "Value matching regex pattern. Similar to the 'like' operator but with support for regex. Pattern must match entire value."
	notSimilarDesc = "Value not matching regex pattern. Similar to the 'like' operator but with support for regex. Pattern must not match entire value."
)

var expAll = []exp{
	{name: "isNull", desc: "Is value null (true) or not null (false)", etype: "Boolean"},
	{name: "_isNull", desc: "Is value null (true) or not null (false)", etype: "Boolean"},
}

var expScalar = []exp{
	{name: "equals", desc: "Equals value"},
	{name: "_eq", desc: "Equals value"},
	{name: "notEquals", desc: "Does not equal value"},
	{name: "_neq", desc: "Does not equal value"},
	{name: "greaterThan", desc: "Is greater than value"},
	{name: "_gt", desc: "Is greater than value"},
	{name: "lesserThan", desc: "Is lesser than value"},
	{name: "_lt", desc: "Is lesser than value"},
	{name: "greaterOrEquals", desc: "Is greater than or equals value"},
	{name: "_gte", desc: "Is greater than or equals value"},
	{name: "lesserOrEquals", desc: "Is lesser than or equals value"},
	{name: "_lte", desc: "Is lesser than or equals value"},
	{name: "like", desc: iLikeDesc},
	{name: "_like", desc: iLikeDesc},
	{name: "notLike", desc: notLikeDesc},
	{name: "_nlike", desc: notLikeDesc},
	{name: "iLike", desc: iLikeDesc},
	{name: "_ilike", desc: iLikeDesc},
	{name: "notILike", desc: notILikeDesc},
	{name: "_nilike", desc: notILikeDesc},
	{name: "similar", desc: similarDesc},
	{name: "_similar", desc: similarDesc},
	{name: "notSimilar", desc: notSimilarDesc},
	{name: "_nsimilar", desc: notSimilarDesc},
	{name: "regex", desc: "Value matches regex pattern"},
	{name: "_regex", desc: "Value matches regex pattern"},
	{name: "notRegex", desc: "Value not matching regex pattern"},
	{name: "_nregex", desc: "Value not matching regex pattern"},
	{name: "iRegex", desc: "Value matches (case-insensitive) regex pattern"},
	{name: "_iregex", desc: "Value matches (case-insensitive) regex pattern"},
	{name: "notIRegex", desc: "Value not matching (case-insensitive) regex pattern"},
	{name: "_niregex", desc: "Value not matching (case-insensitive) regex pattern"},
}

var expList = []exp{
	{name: "in", desc: "Is in list of values"},
	{name: "_in", desc: "Is in list of values"},
	{name: "notIn", desc: "Is not in list of values"},
	{name: "_nin", desc: "Is not in list of values"},
}

var expJSON = []exp{
	{name: "hasKey", desc: "JSON value contains this key"},
	{name: "_hasKey", desc: "JSON value contains this key"},
	{name: "hasKeyAny", desc: "JSON value contains any of these keys"},
	{name: "_hasKeyAny", desc: "JSON value contains any of these keys"},
	{name: "hasKeyAll", desc: "JSON value contains all of these keys"},
	{name: "_hasKeyAll", desc: "JSON value contains all of these keys"},
	{name: "contains", desc: "JSON value matches any of they key/value pairs"},
	{name: "_contains", desc: "JSON value matches any of they key/value pairs"},
	{name: "containedIn", desc: "JSON value contains all of they key/value pairs"},
	{name: "_containedIn", desc: "JSON value contains all of they key/value pairs"},
}
