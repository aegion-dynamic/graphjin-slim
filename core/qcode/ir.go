package qcode

import (
	"encoding/json"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// Code in this file is the language-neutral query IR. It is produced by
// frontends (see core/lang/graphql) and consumed by backends
// (core/sqlgen, core/dialect). It must never import a parser or wire
// protocol.

type QType int8

const (
	QTUnknown      QType = iota // Unknown
	QTQuery                     // Query
	QTSubscription              // Subscription
	QTMutation                  // Mutation
	QTInsert                    // Insert
	QTUpdate                    // Update
	QTDelete                    // Delete
	QTUpsert                    // Upsert
)

type SelType int8

const (
	SelTypeNone SelType = iota
	SelTypeUnion
	SelTypeMember
)

type SkipType int8

const (
	SkipTypeNone SkipType = iota
	SkipTypeDrop
	SkipTypeNulled
	SkipTypeRemote
	// SkipTypeDatabaseJoin indicates this select targets a different database
	// and needs to be handled via cross-database join (similar to remote join
	// but for in-process database calls rather than HTTP)
	SkipTypeDatabaseJoin
)

type ColKey struct {
	Name string
	Base bool
}

type QCode struct {
	Type      QType
	SType     QType
	Name      string
	ActionVar string
	ActionVal json.RawMessage
	Vars      []Var
	Selects   []Select
	Consts    []Constraint
	Roots     []int32
	Mutates   []Mutate
	MUnions   map[string][]int32
	Schema    *sdata.DBSchema
	Remotes   int32
	Cache     Cache
	Typename  bool
	Query     []byte
	Fragments []Fragment
	Warnings  []string // Non-fatal warnings (e.g., missing partition filter)
	// InsertConflictAction is set for insert(..., on_conflict: get).
	// It remains part of the insert operation rather than introducing a
	// separate mutation type.
	InsertConflictAction ConflictAction
	// InsertConflictFallback is set by the SQL compiler when the selected
	// backend needs the retryable select-after-insert fallback.
	InsertConflictFallback bool
}

type Fragment struct {
	Name  string
	Value []byte
}

type Select struct {
	Field
	Type     SelType
	Singular bool
	Typename bool
	Table    string
	Schema   string
	// Database is the target database for this select (multi-database support).
	// Empty string means the default database.
	Database   string
	Fields     []Field
	BCols      []Column
	IArgs      []Arg
	Where      Filter
	OrderBy    []OrderBy
	DistinctOn []sdata.DBColumn
	GroupCols  bool
	// GlobalAgg is true when this select uses aggregate functions
	// without `distinct` — i.e. the entire selection collapses to a
	// single row of global aggregates. Set in compileChildColumns
	// when aggExists && len(DistinctOn) == 0 && this is the top-level
	// select. Drives outer SELECT to skip __gj_id, BCols rendering to
	// emit nothing, and LIMIT to be omitted (a single row is the entire
	// result). Without this flag, the existing render path would emit
	// `LIMIT 20` and produce 20 degenerate per-row rows of aggregates
	// (the bug captured in broken.md).
	GlobalAgg bool
	Paging    Paging
	Children  []int32
	Ti        sdata.DBTable
	Rel       sdata.DBRel
	// ExtraArgs holds GraphQL field arguments that don't match a known
	// qcode arg name. Populated only for selects whose Ti.Type=="remote".
	ExtraArgs               map[string]string
	Joins                   []Join
	PartitionFilterRequired string
	Unrestricted            bool
	Order                   Order
	Through                 string
	ThroughKind             string
	TC                      TConfig
}

type Validation struct {
	Source string
	Type   string
}

type Script struct {
	Source string
	Name   string
}

type TableInfo struct {
	sdata.DBTable
}

type FieldType int8

const (
	FieldTypeTable FieldType = iota
	FieldTypeCol
	FieldTypeFunc
)

type Field struct {
	ID          int32
	ParentID    int32
	Type        FieldType
	Col         sdata.DBColumn
	Func        sdata.DBFunction
	WindowFunc  WindowFunc
	FieldName   string
	FieldFilter Filter
	Args        []Arg
	SkipRender  SkipType
	// Window, when non-nil, marks this aggregate/function field as backend
	// analytic-window IR. The SQL emitter wraps the function call with
	// `OVER (PARTITION BY ... ORDER BY ... <frame>)`. Public GraphQL builds this
	// through analytics directives like @running, @previous, and @rank.
	Window *WindowSpec
}

type Column struct {
	Col         sdata.DBColumn
	FieldFilter Filter
	FieldName   string
}

type Function struct {
	Name string
	// Col       sdata.DBColumn
	Func       sdata.DBFunction
	Args       []Arg
	Agg        bool
	WindowFunc WindowFunc
}

type Filter struct {
	*Exp
}

type Exp struct {
	Op    ExpOp
	Joins []Join
	Order
	OrderBy bool

	Left struct {
		ID      int32
		Table   string
		Col     sdata.DBColumn
		ColName string
		Path    []string
	}
	Right struct {
		ValType  ValType
		Val      string
		ID       int32
		Table    string
		Col      sdata.DBColumn
		ColName  string
		ListType ValType
		ListVal  []string
		Path     []string
		RelPath  []sdata.DBRel // set when Right.Col lives on a related table
	}
	Geo       *GeoExp // GIS-specific expression data
	Children  []*Exp
	childrenA [5]*Exp

	// Scalar-expression payloads (set only for the corresponding Op).
	// These are unused for boolean/comparison ops; keeping them inline
	// avoids allocating a separate struct per leaf node.
	Lit      ExpLit    // OpLiteral
	CaseArms []CaseArm // OpCase
	Else     *Exp      // OpCase ELSE branch (optional)
	CastType string    // OpCast — target SQL type
	RelPath  []sdata.DBRel
	//                     // OpColRef: populated when the column lives on a
	//                     //           related table reached through 1+ FK hops
	//                     //           (e.g. "product.standardcost" from a
	//                     //           salesorderdetail query may be a direct
	//                     //           hop or a chain through an association
	//                     //           table). The renderer emits one nested
	//                     //           correlated subquery per hop. Every hop
	//                     //           must be RelOneToOne (scalar lookup);
	//                     //           one-to-many dereference is rejected at
	//                     //           compile time because a scalar expression
	//                     //           can't consume a list.
}

// ExpLit is a literal scalar value used by OpLiteral leaves.
type ExpLit struct {
	Val     string
	ValType ValType
}

// CaseArm is a single WHEN/THEN pair inside an OpCase node.
// When is a boolean sub-tree (rendered via the existing renderExp);
// Then is a scalar sub-tree (rendered via renderScalarExp).
type CaseArm struct {
	When *Exp
	Then *Exp
}

type Join struct {
	Filter *Exp
	Rel    sdata.DBRel
	Local  bool
}

type ArgType int8

const (
	ArgTypeVal ArgType = iota
	ArgTypeVar
	ArgTypeCol
	ArgTypeExpr // scalar expression tree — Arg.Expr holds the *Exp root
)

type Arg struct {
	Type  ArgType
	DType string
	Name  string
	Val   string
	Col   sdata.DBColumn
	Expr  *Exp // populated when Type == ArgTypeExpr
}

type OrderBy struct {
	KeyVar string
	Key    string
	Col    sdata.DBColumn
	Var    string
	Order  Order
	Func   sdata.DBFunction
	IsFunc bool
	// Alias is set when the user ordered by a SELECT-list alias rather
	// than a column name (e.g. order_by: { revenue: desc } where
	// `revenue` is an expression aggregate field's alias). The validator
	// confirms the alias resolves to a compiled field after
	// compileChildColumns runs; the renderer emits a bare quoted alias
	// (ORDER BY "revenue" DESC), which all 7 SQL dialects accept.
	Alias string
}

type PagingType int8

const (
	PTOffset PagingType = iota
	PTForward
	PTBackward
)

type Paging struct {
	Type      PagingType
	LimitVar  string
	Limit     int32
	OffsetVar string
	Offset    int32
	Cursor    bool
	CursorVar string // "cursor" or "<fieldname>_cursor" for named cursor pagination
	Backward  bool   // true when the query uses last
	NoLimit   bool
}

type Cache struct {
	Header string
}

type Var struct {
	Name string
	Val  json.RawMessage
}

type ExpOp int8

const (
	OpNop ExpOp = iota
	OpAnd
	OpOr
	OpNot
	OpEquals
	OpNotEquals
	OpGreaterOrEquals
	OpLesserOrEquals
	OpGreaterThan
	OpLesserThan
	OpIn
	OpNotIn
	OpLike
	OpNotLike
	OpILike
	OpNotILike
	OpSimilar
	OpNotSimilar
	OpRegex
	OpNotRegex
	OpIRegex
	OpNotIRegex
	OpContains
	OpContainedIn
	OpHasInCommon
	OpHasKey
	OpHasKeyAny
	OpHasKeyAll
	OpIsNull
	OpIsNotNull
	OpTsQuery
	OpFalse
	OpNotDistinct
	OpDistinct
	OpEqualsTrue
	OpNotEqualsTrue
	OpSelectExists
	OpJSONPath     // JSON path operator (->)
	OpJSONPathText // JSON path text operator (->>)

	// GIS/Spatial operators
	OpGeoDistance   // ST_DWithin - distance-based filtering
	OpGeoWithin     // ST_Within - geometry A within B
	OpGeoContains   // ST_Contains - geometry A contains B
	OpGeoIntersects // ST_Intersects - geometries intersect
	OpGeoCoveredBy  // ST_CoveredBy - geometry A covered by B
	OpGeoCovers     // ST_Covers - geometry A covers B
	OpGeoTouches    // ST_Touches - geometries touch at boundary
	OpGeoOverlaps   // ST_Overlaps - geometries overlap
	OpGeoNear       // MongoDB $near / $nearSphere

	// Scalar arithmetic operators — used inside aggregate expressions
	// (e.g. SUM(unitprice * orderqty)). These never appear in WHERE
	// predicates; the validator in qcode/expr.go rejects them outside
	// expression trees. Keeping them in the same ExpOp enum lets the
	// existing Children/Left/Right machinery and dialect rendering be
	// reused. Discipline: arithmetic ops only have arithmetic children,
	// boolean ops only have boolean children, with the bridge being
	// CaseArm.When (boolean) → CaseArm.Then (scalar).
	OpAdd      // a + b (variadic)
	OpSub      // a - b (variadic; subtracts left-to-right)
	OpMul      // a * b (variadic)
	OpDiv      // a / b (binary)
	OpMod      // a % b (binary)
	OpNeg      // -a    (unary)
	OpCoalesce // COALESCE(a, b, ...)
	OpNullIf   // NULLIF(a, b)
	OpCase     // CASE WHEN ... THEN ... ELSE ... END (uses CaseArms + Else)
	OpCast     // CAST(a AS type) — uses CastType
	OpLiteral  // numeric/string/bool literal — uses Lit
	OpColRef   // column reference leaf — uses Left.Col

	// Aggregate-of-expression ops — only legal at the top level of a
	// non-aggregate's expr: argument, used for ratio-of-aggregates
	// (e.g. div(expr: { num_: { sum: { col: ... } }, den: ... })).
	OpAggSum
	OpAggAvg
	OpAggMin
	OpAggMax
	OpAggCount
)

type ValType int8

const (
	ValStr ValType = iota + 1
	ValNum
	ValBool
	ValList
	ValObj
	ValVar
	ValDBVar
	ValSubQuery
	ValPartitionBound // Renders as NOW() - INTERVAL N days (dialect-specific)
	ValRef
)

// GeoUnit represents distance units for GIS operations
type GeoUnit int8

const (
	GeoUnitMeters GeoUnit = iota
	GeoUnitKilometers
	GeoUnitMiles
	GeoUnitFeet
)

// ToMeters converts a distance value to meters based on the unit
func (u GeoUnit) ToMeters(val float64) float64 {
	switch u {
	case GeoUnitKilometers:
		return val * 1000
	case GeoUnitMiles:
		return val * 1609.344
	case GeoUnitFeet:
		return val * 0.3048
	default:
		return val
	}
}

// GeoExp holds GIS-specific expression data
type GeoExp struct {
	// Geometry specification (one of these will be set)
	Point   []float64   // [longitude, latitude] for point
	Polygon [][]float64 // Array of [lon, lat] pairs for polygon ring
	GeoJSON []byte      // Full GeoJSON geometry object

	// Operation parameters
	Distance    float64 // Distance value for st_dwithin
	DistanceVar string  // Variable name for distance if parameterized
	Unit        GeoUnit // Distance unit (meters, km, miles, feet)
	SRID        int     // Spatial Reference ID (default 4326 = WGS84)

	// For MongoDB
	MinDistance float64 // $minDistance for $near
	Spherical   bool    // Use spherical calculations
}

type AggregrateOp int8

const (
	AgCount AggregrateOp = iota + 1
	AgSum
	AgAvg
	AgMax
	AgMin
)

type Order int8

const (
	OrderNone Order = iota
	OrderAsc
	OrderDesc
	OrderAscNullsFirst
	OrderAscNullsLast
	OrderDescNullsFirst
	OrderDescNullsLast
)

func (o Order) String() string {
	return []string{"None", "ASC", "DESC", "ASC NULLS FIRST", "ASC NULLS LAST", "DESC NULLLS FIRST", "DESC NULLS LAST"}[o]
}

// HasFilterOnColumn walks the expression tree and returns true if any
// comparison references the given column name.
func HasFilterOnColumn(ex *Exp, colName string) bool {
	if ex == nil {
		return false
	}
	if ex.Left.Col.Name == colName {
		return true
	}
	for _, child := range ex.Children {
		if HasFilterOnColumn(child, colName) {
			return true
		}
	}
	return false
}

func IsGeoOp(op ExpOp) bool {
	switch op {
	case OpGeoDistance, OpGeoWithin, OpGeoContains, OpGeoIntersects,
		OpGeoCoveredBy, OpGeoCovers, OpGeoTouches, OpGeoOverlaps, OpGeoNear:
		return true
	}
	return false
}

// NewExp returns an expression pre-filling sentinel IDs and the pooled
// child array, matching what frontends previously did inline.
func NewExp() *Exp {
	ex := &Exp{Op: OpNop}
	ex.Left.ID = -1
	ex.Right.ID = -1
	ex.Children = ex.childrenA[:0]
	return ex
}

// SetPooledChildren points Children at the first n slots of the pooled
// child array, avoiding allocation for small fan-outs.
func (ex *Exp) SetPooledChildren(n int) {
	ex.Children = ex.childrenA[:n]
}
