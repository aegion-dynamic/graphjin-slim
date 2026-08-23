package graphql

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"

	"fmt"
	"strconv"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/util"
)

func (co *Compiler) compileFields(
	st *util.StackInt32,
	op *graph.Operation,
	qc *qcode.QCode,
	sel *qcode.Select,
	field graph.Field,
) (err error) {
	sel.Fields = make([]qcode.Field, 0, len(field.Children))
	sel.BCols = make([]qcode.Column, 0, len(field.Children))

	if sel.Rel.Type == sdata.RelDatabaseJoin {
		co.compileDatabaseJoinPassthroughFields(st, op, sel, field)
		return co.addColumns(qc, sel)
	}

	if err = co.compileChildColumns(st, op, qc, sel, field); err != nil {
		return
	}

	// Ordering by an aggregate (order_by: { sum_price: desc }) forces a
	// grouped compilation even when the selection itself has no aggregate
	// field: ORDER BY SUM(x) is only valid alongside a GROUP BY over the
	// selected columns. Without this the ORDER BY references an aggregate
	// in an ungrouped query, which every dialect rejects.
	for _, ob := range sel.OrderBy {
		if ob.IsFunc {
			sel.GroupCols = true
			break
		}
	}

	if err = co.addColumns(qc, sel); err != nil {
		return
	}

	co.addOrderByColumns(sel)

	// Inject __gj_id field for cache tracking if enabled.
	//
	// Skip for aggregated queries (GroupCols): row-level cache tracking
	// is incoherent when the result is an aggregation over many rows —
	// there is no single PK that identifies the output row. More
	// importantly, re-adding __gj_id to BCols here would undo the work
	// of removeCacheTrackingField (called from compileChildColumns when
	// aggregates are detected) and force a per-row GROUP BY, producing
	// the broken.md degenerate result (20 rows of count_id:1).
	if co.c.EnableCacheTracking && qc.Type == qcode.QTQuery && !sel.GroupCols {
		co.addCacheTrackingField(sel)
	}

	return nil
}

func (co *Compiler) compileDatabaseJoinPassthroughFields(
	st *util.StackInt32,
	op *graph.Operation,
	sel *qcode.Select,
	gf graph.Field,
) {
	for _, cid := range gf.Children {
		f := op.Fields[cid]
		if f.Type == graph.FieldKeyword {
			continue
		}
		name := co.ParseName(f.Name)
		if len(f.Children) != 0 {
			val := f.ID | (sel.ID << 16)
			st.Push(val)
			continue
		}

		fieldName := f.Name
		if f.Alias != "" {
			fieldName = f.Alias
		}
		sel.Fields = append(sel.Fields, qcode.Field{
			ID:        int32(len(sel.Fields)),
			ParentID:  sel.ID,
			Type:      qcode.FieldTypeCol,
			FieldName: fieldName,
			Col: sdata.DBColumn{
				Name:     name,
				Schema:   sel.Ti.Schema,
				Table:    sel.Ti.Name,
				Database: sel.Ti.Database,
			},
		})
	}
}

func (co *Compiler) compileChildColumns(
	st *util.StackInt32,
	op *graph.Operation,
	qc *qcode.QCode,
	sel *qcode.Select,
	gf graph.Field,
) (err error) {
	var aggExists bool
	var id int32

	for _, cid := range gf.Children {
		field := qcode.Field{ID: id, ParentID: sel.ID, Type: qcode.FieldTypeCol}
		f := op.Fields[cid]

		name := co.ParseName(f.Name)

		if f.Alias != "" {
			field.FieldName = f.Alias
		} else {
			field.FieldName = f.Name
		}

		// these are all remote fields we use
		// these later to strip the response json
		if sel.Rel.Type == sdata.RelRemote {
			if err := validateRemoteField(sel, name); err != nil {
				return err
			}
			// Remote tables have no DB column to resolve, but the response
			// filter still needs the source name: it matches resolver JSON
			// by Col.Name and emits under FieldName (the alias).
			field.Col.Name = name
			sel.Fields = append(sel.Fields, field)
			continue
		}

		if len(f.Children) != 0 {
			val := f.ID | (sel.ID << 16)
			st.Push(val)
			continue
		}

		switch {
		case name == "__typename":
			sel.Typename = true
			continue

		case strings.HasSuffix(name, "_cursor"):
			continue
		}

		var isCol, isFunc, fieldAgg bool
		var fn qcode.Function

		field.Col, isCol = sel.Ti.ColumnExists(name)

		if !isCol {
			fn, isFunc, err = co.isFunction(sel, name, f)
			if err != nil {
				return err
			}
		}

		switch {
		case isCol:
		case isFunc:
			field.Type = qcode.FieldTypeFunc
			field.Func = fn.Func
			field.Args = fn.Args
			field.WindowFunc = fn.WindowFunc
			// Defer flipping the GROUP BY flag until after directive
			// compilation: an aggregate carrying analytics window metadata
			// emits one row per input row and must NOT trigger GROUP BY.
			fieldAgg = fn.Agg
			// For the new expression-aggregate path, run the AST validator.
			// This enforces type-checks (numeric columns under arithmetic)
			// and depth/node caps.
			if len(fn.Args) == 1 && fn.Args[0].Type == qcode.ArgTypeExpr {
				if err := validateExprTree(fn.Args[0].Expr, sel.Ti); err != nil {
					return fmt.Errorf("field '%s': %w", name, err)
				}
			}
		default:
			return fmt.Errorf("field '%s' is not a column or a function", name)
		}

		if err := co.compileFieldDirectives(sel, &field, f.Directives); err != nil {
			return err
		}
		if field.WindowFunc != qcode.WindowFuncNone && field.Window == nil {
			return fmt.Errorf("analytics function %q is internal; use GraphJin analytics directives like @previous, @rank, or @rowNumber on a column field", field.WindowFunc.String())
		}

		// Plain aggregates participate in GROUP BY; analytics aggregates do not
		// (the backend window clause produces a row per input).
		if fieldAgg && field.Window == nil {
			aggExists = true
		}

		if err := co.compileFieldArgs(qc, sel, &field, f.Args); err != nil {
			return err
		}

		if field.Col.Blocked {
			return fmt.Errorf("column: '%s.%s.%s' blocked",
				field.Col.Schema,
				field.Col.Table,
				field.Col.Name)
		}

		if field.SkipRender == qcode.SkipTypeDrop {
			continue
		}

		// this is needed cause recursive selects cannot have functions
		// in them so we need to render the function a level above
		// and therefore the column to run to aggregation function
		// on should be included in the base columns
		if isFunc && fn.Agg && sel.Rel.Type == sdata.RelRecursive {
			// Expression-aggregate fields don't have a single source column
			// to add to BCols (the expression may reference multiple). The
			// recursive-select base-column injection is a workaround for the
			// legacy single-column path; for the expression path, the
			// caller is expected to use distinct + non-recursive selection.
			if len(fn.Args) > 0 && fn.Args[0].Type == qcode.ArgTypeCol {
				selAddBaseCol(sel, qcode.Column{Col: fn.Args[0].Col})
			}
		}
		selAddField(sel, field)
		id++
	}

	if aggExists {
		sel.GroupCols = true
		// Remove injected __gj_id from BCols and Fields — including the
		// primary key in GROUP BY makes every group unique (count always 1).
		selRemoveCacheTrackingField(sel)

		// Detect the global-aggregate case: top-level selection consists
		// ONLY of aggregate fields (no regular columns) and has no
		// `distinct` clause. In this case the entire result is one row of
		// aggregates with no grouping dimension — emit no GROUP BY,
		// no per-row LIMIT, no PK in SELECT.
		//
		// If the user mixes regular columns with aggregates (e.g.
		// `{ products { name count_id } }`), the regular column needs to
		// appear in GROUP BY — that path is unchanged. broken.md's bug
		// only manifests for the pure-aggregate case where the SQL
		// should collapse to a single row but currently emits LIMIT 20.
		if len(sel.DistinctOn) == 0 && sel.Rel.Type == sdata.RelNone &&
			!hasNonAggField(sel.Fields) {
			sel.GlobalAgg = true
			// Suppress LIMIT — a global aggregate produces exactly one
			// row. Without this override, the default limit (20) applied
			// by setLimit would land in the SQL and trigger the per-row
			// degenerate result described in broken.md.
			sel.Paging.NoLimit = true
			sel.Paging.Limit = 0
		}
	}
	return nil
}

// validateRemoteField enforces the closed column surface on remote tables
// that declare one (StrictColumns, e.g. filesystem tables). All other
// remote tables keep the historical lenient pass-through: resolver-backed
// and OpenAPI tables may serve fields their registered columns don't list.
// Keyword selections (__typename, *_cursor) stay pass-through everywhere —
// they are protocol fields, not columns.
func validateRemoteField(sel *qcode.Select, name string) error {
	if !sel.Ti.StrictColumns {
		return nil
	}
	if name == "__typename" || strings.HasSuffix(name, "_cursor") {
		return nil
	}
	if _, ok := sel.Ti.ColumnExists(name); ok {
		return nil
	}
	cols := make([]string, len(sel.Ti.Columns))
	for i, c := range sel.Ti.Columns {
		cols[i] = c.Name
	}
	return fmt.Errorf("column '%s' does not exist on table '%s'; available columns: %s",
		name, sel.Ti.Name, strings.Join(cols, ", "))
}

// hasNonAggField reports whether the field list contains any FieldTypeCol
// (regular column) field. Used to distinguish global-aggregate selections
// (only FieldTypeFunc fields) from mixed selections that need GROUP BY on
// the regular columns.
func hasNonAggField(fields []qcode.Field) bool {
	for _, f := range fields {
		if f.Type == qcode.FieldTypeCol {
			return true
		}
	}
	return false
}

func newArgs(sel *qcode.Select, f sdata.DBFunction, arg graph.Arg) (args []qcode.Arg, err error) {
	node := arg.Val
	for i, argNode := range node.Children {
		var a qcode.Arg
		a, err = parseArg(argNode, f, i)
		if err != nil {
			return
		}
		switch argNode.Type {
		case graph.NodeLabel:
			a.Type = qcode.ArgTypeCol
			a.Col, err = sel.Ti.GetColumn(argNode.Val)
		case graph.NodeVar:
			a.Type = qcode.ArgTypeVar
			fallthrough
		default:
			a.Val = argNode.Val
		}
		if err != nil {
			return
		}
		args = append(args, a)
	}
	return
}

func parseArg(arg *graph.Node, f sdata.DBFunction, index int) (a qcode.Arg, err error) {
	argName := arg.Name
	if numArgKeyRe.MatchString(argName) {
		var n int
		argName = argName[1:]
		n, err = strconv.Atoi(argName)
		if err != nil {
			err = fmt.Errorf("db function %s: invalid key: %s", f.Name, arg.Name)
			return
		}
		if n != index {
			err = fmt.Errorf("db function %s: invalid key order: %s", f.Name, arg.Name)
			return
		}
		a = qcode.Arg{DType: f.Inputs[n].Type}
		return
	}

	var input sdata.DBFuncParam
	input, err = f.GetInput(argName)
	if err != nil {
		err = fmt.Errorf("db function %s: %w", f.Name, err)
	}
	a = qcode.Arg{Name: arg.Name, DType: input.Type}
	return
}

func (co *Compiler) addOrderByColumns(sel *qcode.Select) {
	for _, ob := range sel.OrderBy {
		// Aggregate order-by (sum_price → ORDER BY SUM(price)) and
		// SELECT-list-alias order-by (ORDER BY "revenue") never reference
		// the raw column in the emitted SQL. BCols feeds both the base
		// SELECT list and GROUP BY, so projecting the aggregate's source
		// column here would add it to GROUP BY and collapse every group
		// to a single row — turning SUM per dimension into per-row values.
		if ob.IsFunc || ob.Alias != "" {
			continue
		}
		selAddBaseCol(sel, qcode.Column{Col: ob.Col})
	}
}

func (co *Compiler) addColumns(qc *qcode.QCode, sel *qcode.Select) error {
	var rel sdata.DBRel

	switch {
	case len(sel.Joins) == 0:
		rel = sel.Rel
	case sel.Joins[0].Local:
		return nil
	default:
		rel = sel.Joins[0].Rel
	}
	if err := co.addRelColumns(qc, sel, rel); err != nil {
		return err
	}

	// co.addFuncColumns(qc, sel)
	return nil
}

func (co *Compiler) AddRelColumns(qc *qcode.QCode, sel *qcode.Select, rel sdata.DBRel) error {
	return co.addRelColumns(qc, sel, rel)
}

func (co *Compiler) addRelColumns(qc *qcode.QCode, sel *qcode.Select, rel sdata.DBRel) error {
	var psel *qcode.Select

	if sel.ParentID != -1 {
		psel = &qc.Selects[sel.ParentID]
	} else {
		return nil
	}

	switch rel.Type {
	case sdata.RelNone:
		return nil

	case sdata.RelOneToOne, sdata.RelOneToMany:
		selAddBaseCol(psel, qcode.Column{Col: rel.Right.Col})
		// Composite FK: add extra pair columns to parent's base columns
		for _, pair := range rel.ExtraPairs {
			selAddBaseCol(psel, qcode.Column{Col: pair.R})
		}

	case sdata.RelEmbedded:
		selAddBaseCol(psel, qcode.Column{Col: rel.Right.Col})

	case sdata.RelRemote:
		f := qcode.Field{Type: qcode.FieldTypeCol, Col: rel.Right.Col, FieldName: rel.Left.Col.Name}
		selAddField(psel, f)
		sel.SkipRender = qcode.SkipTypeRemote

	case sdata.RelDatabaseJoin:
		// Cross-database join: add the foreign key column to parent for ID extraction,
		// and mark this select to be handled by the database join execution path.
		// Use a synthetic placeholder name (__%s_db_join) so it's unique and matches
		// what databaseJoinFieldIds() searches for during result stitching.
		placeholderName := fmt.Sprintf("__%s_db_join", sel.FieldName)
		f := qcode.Field{Type: qcode.FieldTypeCol, Col: rel.Right.Col, FieldName: placeholderName}
		selAddField(psel, f)
		sel.SkipRender = qcode.SkipTypeDatabaseJoin
		sel.Database = sel.Ti.Database

	case sdata.RelPolymorphic:
		typeCol := rel.Left.Col
		typeCol.Name = rel.Left.Col.FKeyCol

		selAddBaseCol(psel, qcode.Column{Col: rel.Left.Col})
		selAddBaseCol(psel, qcode.Column{Col: typeCol})

	case sdata.RelRecursive:
		selAddBaseCol(sel, qcode.Column{Col: rel.Left.Col})
		selAddBaseCol(sel, qcode.Column{Col: rel.Right.Col})
	}
	return nil
}

func (co *Compiler) orderByIDCol(sel *qcode.Select) error {
	if sel.Ti.PrimaryCol.Name == "" {
		return fmt.Errorf("table requires primary key: %s", sel.Ti.Name)
	}

	for _, pkCol := range sel.Ti.PrimaryCols {
		selAddBaseCol(sel, qcode.Column{Col: pkCol})

		already := false
		for _, ob := range sel.OrderBy {
			if ob.Col.Name == pkCol.Name {
				already = true
				break
			}
		}
		if !already {
			sel.OrderBy = append(sel.OrderBy, qcode.OrderBy{Col: pkCol, Order: sel.Order})
		}
	}
	return nil
}

// addCacheTrackingField injects __gj_id field with primary key for cache row tracking
func (co *Compiler) addCacheTrackingField(sel *qcode.Select) {
	// Skip if table has no primary key
	pk := sel.Ti.PrimaryCol
	if pk.Name == "" {
		return
	}

	// Skip if __gj_id already exists
	for _, f := range sel.Fields {
		if f.FieldName == "__gj_id" {
			return
		}
	}

	// For single PK, skip if PK column is already requested
	if !sel.Ti.HasCompositePK() {
		for _, f := range sel.Fields {
			if f.Type == qcode.FieldTypeCol && strings.EqualFold(f.Col.Name, pk.Name) {
				return
			}
		}
	}

	// Add the injected field (uses first PK column; cache_response.go extracts the value)
	field := qcode.Field{
		ID:        int32(len(sel.Fields)),
		ParentID:  sel.ID,
		Type:      qcode.FieldTypeCol,
		Col:       pk,
		FieldName: "__gj_id",
	}

	sel.Fields = append(sel.Fields, field)

	// Also add all PK columns to base columns
	for _, pkCol := range sel.Ti.PrimaryCols {
		if selBColExists(sel, pkCol.Name) == -1 {
			sel.BCols = append(sel.BCols, qcode.Column{Col: pkCol, FieldName: "__gj_id"})
		}
	}
}

// Frontend helpers over the IR Select. They live here rather than as
// methods because the IR type lives in core/qcode.
func selAddField(sel *qcode.Select, f qcode.Field) {
	if f.Type == qcode.FieldTypeCol && selBColExists(sel, f.Col.Name) == -1 {
		sel.BCols = append(sel.BCols, qcode.Column{Col: f.Col, FieldName: f.FieldName})
	}
	if selFieldExists(sel, f.FieldName) == -1 {
		sel.Fields = append(sel.Fields, f)
	}
}

func selAddBaseCol(sel *qcode.Select, col qcode.Column) {
	if selBColExists(sel, col.Col.Name) == -1 {
		sel.BCols = append(sel.BCols, col)
	}
}

// selRemoveCacheTrackingField strips the __gj_id column injected by
// addCacheTrackingField. When aggregation functions are present the PK
// must not appear in SELECT or GROUP BY — otherwise every group is
// unique and counts are always 1. This applies to all database dialects.
func selRemoveCacheTrackingField(sel *qcode.Select) {
	for i := len(sel.BCols) - 1; i >= 0; i-- {
		if sel.BCols[i].FieldName == "__gj_id" {
			sel.BCols = append(sel.BCols[:i], sel.BCols[i+1:]...)
		}
	}
	for i := len(sel.Fields) - 1; i >= 0; i-- {
		if sel.Fields[i].FieldName == "__gj_id" {
			sel.Fields = append(sel.Fields[:i], sel.Fields[i+1:]...)
		}
	}
}

func selFieldExists(sel *qcode.Select, name string) int {
	for i, c := range sel.Fields {
		if strings.EqualFold(c.FieldName, name) {
			return i
		}
	}
	return -1
}

func selBColExists(sel *qcode.Select, name string) int {
	for i, c := range sel.BCols {
		if strings.EqualFold(c.Col.Name, name) {
			return i
		}
	}
	return -1
}
