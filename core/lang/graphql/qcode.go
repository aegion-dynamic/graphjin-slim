//go:generate stringer -linecomment -type=QType,MType,SelType,FieldType,SkipType,PagingType,AggregrateOp,ValType,ExpOp -output=./gen_string.go
package graphql

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/util"
)

const (
	maxSelectors        = 100
	singularSuffixCamel = "ByID"
	singularSuffixSnake = "_by_id"
)

var qualifiedGraphQLRootPattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+)\s*[({]`)

type Compiler struct {
	c Config
	s *sdata.DBSchema

	// Per-compile scratch: mutation parse output and tree-building
	// state that backends never observe.
	actionArgs map[string]graph.Arg
	actionArg  graph.Arg
	rootsA     [5]int32
	mutMeta    map[int32]*mutItem
}

func NewCompiler(s *sdata.DBSchema, c Config) (*Compiler, error) {
	if c.DBSchema == "" {
		if s.DBType() != "sqlite" {
			c.DBSchema = s.DBSchema()
		}
	}

	return &Compiler{c: c, s: s}, nil
}

func (co *Compiler) Compile(
	query []byte,
	vmap map[string]json.RawMessage,
	namespace string,
) (qc *qcode.QCode, err error) {
	var op graph.Operation
	op, err = graph.Parse(query)
	if err != nil {
		err = qualifiedGraphQLRootError(query, err)
		return
	}
	// The permissive lexer in older parser builds can recover from a dotted
	// root by discarding the leading identifiers. Reject it explicitly rather
	// than compiling a misleading partial root.
	if qualifiedErr := qualifiedGraphQLRootError(query, nil); qualifiedErr != nil {
		return nil, qualifiedErr
	}

	qc = &qcode.QCode{
		Name:      op.Name,
		SType:     qcode.QTQuery,
		Schema:    co.s,
		Query:     op.Query,
		Fragments: make([]qcode.Fragment, len(op.Frags)),
		Vars:      make([]qcode.Var, len(op.VarDef)),
	}

	for i, f := range op.Frags {
		qc.Fragments[i] = qcode.Fragment{Name: f.Name, Value: f.Value}
	}

	var buf bytes.Buffer
	for i, v := range op.VarDef {
		graphNodeToJSON(v.Val, &buf)
		qc.Vars[i] = qcode.Var{Name: v.Name, Val: buf.Bytes()}
		buf.Reset()
	}

	qc.Roots = co.rootsA[:0]
	qc.Type = GetQType(op.Type)

	if err = co.compileQuery(qc, &op); err != nil {
		return
	}

	if qc.Type == qcode.QTMutation {
		if err = co.compileMutation(qc, vmap); err != nil {
			return
		}
	}
	return
}

func qualifiedGraphQLRootError(query []byte, parseErr error) error {
	match := qualifiedGraphQLRootPattern.FindSubmatch(query)
	if len(match) != 2 {
		return parseErr
	}
	qualified := string(match[1])
	parts := strings.Split(qualified, ".")
	unqualified := parts[len(parts)-1]
	message := fmt.Sprintf("roots are unqualified table names: write `%s`, not `%s`", unqualified, qualified)
	if parseErr != nil {
		return fmt.Errorf("%w: %s", parseErr, message)
	}
	return fmt.Errorf("invalid GraphQL root %q: %s", qualified, message)
}

func (co *Compiler) compileQuery(qc *qcode.QCode, op *graph.Operation) error {
	var id int32

	if len(op.Fields) == 0 {
		return errors.New("invalid graphql no query found")
	}

	if op.Type == graph.OpMutate {
		if err := co.setMutationType(qc, op); err != nil {
			return err
		}
	}
	if err := co.compileOpDirectives(qc, op.Directives); err != nil {
		return err
	}

	qc.Selects = make([]qcode.Select, 0, 5)
	st := util.NewStackInt32()

	if len(op.Fields) == 0 {
		return errors.New("empty query")
	}

	for _, f := range op.Fields {
		if f.ParentID == -1 {
			if f.Name == "__typename" && op.Name != "" {
				qc.Typename = true
			}
			val := f.ID | (-1 << 16)
			st.Push(val)
		}
	}

	for {
		if st.Len() == 0 {
			break
		}

		if id >= maxSelectors {
			return fmt.Errorf("selector limit reached (%d)", maxSelectors)
		}

		val := st.Pop()
		fid := val & 0xFFFF
		parentID := (val >> 16) & 0xFFFF

		field := op.Fields[fid]

		// A keyword is a cursor field at the top-level
		// For example posts_cursor in the root
		if field.Type == graph.FieldKeyword {
			continue
		}

		if field.ParentID == -1 {
			parentID = -1
		}

		s1 := qcode.Select{
			Field: qcode.Field{ID: id, ParentID: parentID, Type: qcode.FieldTypeTable},
		}

		sel := &s1

		name := co.ParseName(field.Name)

		if field.Alias != "" {
			sel.FieldName = field.Alias
		} else {
			sel.FieldName = field.Name
		}

		sel.Children = make([]int32, 0, 5)

		if err := co.compileSelectorDirectives(qc, sel, field.Directives); err != nil {
			return err
		}

		if parentID != -1 && int(parentID) < len(qc.Selects) && qc.Selects[parentID].SkipRender == qcode.SkipTypeDatabaseJoin {
			psel := &qc.Selects[parentID]
			psel.Children = append(psel.Children, sel.ID)
			sel.SkipRender = qcode.SkipTypeDatabaseJoin
			sel.Database = psel.Database
			sel.Table = name
			sel.Ti = sdata.DBTable{Name: name, Database: psel.Database}
			co.compileDatabaseJoinPassthroughFields(st, op, sel, field)
			qc.Selects = append(qc.Selects, s1)
			id++
			continue
		}

		if err := co.addRelInfo(name, op, qc, sel, field); err != nil {
			return err
		}

		co.setLimit(qc, sel)

		if err := co.compileSelectArgs(qc, sel, field.Args); err != nil {
			return err
		}

		if err := co.compileFields(st, op, qc, sel, field); err != nil {
			return err
		}

		// Analytics partition/order columns are stored as names in WindowSpec,
		// so validate them after compileFields has built the field list.
		if err := co.validateAnalytics(qc, sel); err != nil {
			return err
		}

		// Resolve deferred order_by aliases now that the SELECT list is
		// known. compileArgOrderByObj records alias-based ordering
		// tentatively; this pass rejects any alias that doesn't match
		// a compiled field's FieldName.
		if err := validateOrderByAliases(sel); err != nil {
			return err
		}

		// Authorization for ORDER BY and DISTINCT ON columns —
		// mirrors the SELECT-list checks. Must run before
		// the cursor block below appends system entries (PK tie-breakers,
		// clustering keys), which are exempt.
		if err := co.validateOrderBy(qc, sel); err != nil {
			return err
		}

		// Check partition key filter: inject default or warn
		co.checkPartitionFilter(qc, sel)

		// If an actual cursor is available
		if sel.Paging.Cursor {
			// Skip cursor PK ordering when aggregation is active — adding PK
			// columns to ORDER BY conflicts with GROUP BY (they aren't grouped).
			// Aggregated queries degrade to limit-only (no cursor returned).
			if sel.GroupCols {
				sel.Paging.Cursor = false
			} else {

				// Set tie-breaker order column for the cursor direction
				// this column needs to be the last in the order series.
				if err := co.orderByIDCol(sel); err != nil {
					return err
				}

				// Set filter chain needed to make the cursor work
				if sel.Paging.Type != qcode.PTOffset {
					co.addSeekPredicate(sel)
				}
			}
		}

		// Compute and set the relevant where clause required to join
		// this table with its parent
		co.setRelFilters(qc, sel)

		if err := co.validateSelect(sel); err != nil {
			return err
		}

		qc.Selects = append(qc.Selects, s1)
		id++
	}

	if id == 0 {
		return errors.New("invalid query: no selectors found")
	}

	if err := validateGroupByJoinShape(qc); err != nil {
		return err
	}

	return nil
}

// validateGroupByJoinShape rejects queries where a parent selection has both
// a `distinct:` clause and an aggregate, AND nests a child whose join column
// on the parent side is not in `distinct`. The compiler emits a GROUP BY CTE
// that collapses the un-distinct'd FK column, leaving the nested lateral
// join referencing a column the CTE no longer projects ("salesorderdetail_0
// .salesorderid does not exist" / "purchases_0.customer_id does not exist").
//
// Semantically the query is ambiguous — many join-key values per group —
// so silent SQL emission would be wrong even if it executed. The right
// shape for "metric by dimension" is to root at the dimension table.
func validateGroupByJoinShape(qc *qcode.QCode) error {
	for i := range qc.Selects {
		child := &qc.Selects[i]
		if child.ParentID == -1 {
			continue
		}
		switch child.Rel.Type {
		case sdata.RelRemote, sdata.RelDatabaseJoin, sdata.RelNone, sdata.RelPolymorphic:
			continue
		}
		parent := &qc.Selects[child.ParentID]
		if !parent.GroupCols || len(parent.DistinctOn) == 0 {
			continue
		}

		// The column on the parent side that the nested join uses.
		// For single-hop nesting it lives on sel.Rel; for multi-hop, the
		// hop touching the parent is Joins[0].
		var joinCol string
		if len(child.Joins) > 0 {
			joinCol = child.Joins[0].Rel.Right.Col.Name
		} else {
			joinCol = child.Rel.Right.Col.Name
		}
		if joinCol == "" {
			continue
		}

		inDistinct := false
		for _, dc := range parent.DistinctOn {
			if strings.EqualFold(dc.Name, joinCol) {
				inDistinct = true
				break
			}
		}
		if inDistinct {
			continue
		}

		distinctNames := make([]string, len(parent.DistinctOn))
		for j, dc := range parent.DistinctOn {
			distinctNames[j] = dc.Name
		}
		return fmt.Errorf(
			"nested selection '%s' joins through parent column '%s.%s', which is not in distinct: [%s]. The GROUP BY collapses '%s' away, leaving many %s values per group with no defined join. Root the query at the dimension table instead — see get_workflow_guide for the metric-by-dimension pattern.",
			child.Table, parent.Table, joinCol, strings.Join(distinctNames, ", "), joinCol, joinCol)
	}
	return nil
}

func (co *Compiler) addRelInfo(
	name string,
	op *graph.Operation,
	qc *qcode.QCode,
	sel *qcode.Select,
	field graph.Field,
) error {
	var psel *qcode.Select
	var childF, parentF graph.Field
	var err error

	childF = field

	if sel.ParentID == -1 {
		qc.Roots = append(qc.Roots, sel.ID)
	} else {
		psel = &qc.Selects[sel.ParentID]
		psel.Children = append(psel.Children, sel.ID)
		parentF = op.Fields[field.ParentID]
	}

	switch field.Type {
	case graph.FieldUnion:
		sel.Type = qcode.SelTypeUnion
		if psel == nil {
			return fmt.Errorf("union types are only valid with polymorphic relationships")
		}

	case graph.FieldMember:
		// TODO: Fix this
		// if sel.Table != sel.Table {
		// 	return fmt.Errorf("inline fragment: 'on %s' should be 'on %s'", sel.Table, sel.Table)
		// }
		sel.Type = qcode.SelTypeMember
		sel.Singular = psel.Singular

		childF = parentF
		parentF = op.Fields[int(parentF.ParentID)]
	}

	if sel.Rel.Type == sdata.RelSkip {
		sel.Rel.Type = sdata.RelNone
	} else if sel.ParentID != -1 {
		parentName := co.ParseName(parentF.Name)
		childName := co.ParseName(childF.Name)

		var path []sdata.TPath
		if sel.ThroughKind == "column" {
			path, err = co.FindPathByColumn(childName, parentName, sel.Through)
		} else {
			path, err = co.FindPath(childName, parentName, sel.Through)
		}
		if err != nil {
			return graphError(err, childName, parentName, sel.Through)
		}
		sel.Rel = sdata.PathToRel(path[0])

		// Check if this is a cross-database relationship
		// If so, convert to RelDatabaseJoin for special handling
		if sel.Rel.IsCrossDatabase() {
			sel.Rel.Type = sdata.RelDatabaseJoin
		}

		// for _, p := range path {
		// 	rel := sdata.PathToRel(p)
		// 	fmt.Println(childF.Name, parentF.Name,
		// 		"--->>>", rel.Left.Col.Table, rel.Left.Col.Name,
		// 		"|", rel.Right.Col.Table, rel.Right.Col.Name)
		// }

		rpath := path[1:]

		for i := len(rpath) - 1; i >= 0; i-- {
			p := rpath[i]
			rel := sdata.PathToRel(p)
			var pid int32
			if i == len(rpath)-1 {
				pid = sel.ParentID
			} else {
				pid = -1
			}
			sel.Joins = append(sel.Joins, qcode.Join{
				Rel:    rel,
				Filter: buildFilter(rel, pid),
			})
		}
	}

	if sel.ParentID == -1 ||
		sel.Rel.Type == sdata.RelPolymorphic ||
		sel.Rel.Type == sdata.RelNone {
		schema := co.c.DBSchema
		if sel.Schema != "" {
			schema = sel.Schema
		}
		if sel.Ti, err = co.Find(schema, name); err != nil {
			return err
		}
	} else {
		sel.Ti = sel.Rel.Left.Ti
	}

	if sel.ParentID == -1 && sel.Ti.Type == "remote" && sel.Ti.PrimaryCol.FKeyTable == "" {
		sel.Rel = sdata.DBRel{Type: sdata.RelRemote}
		sel.SkipRender = qcode.SkipTypeRemote
	}

	if sel.Ti.Blocked {
		return fmt.Errorf("table: '%t' (%s) blocked", sel.Ti.Blocked, name)
	}

	sel.Table = sel.Ti.Name
	sel.TC = co.getTConfig(sel.Ti.Schema, sel.Ti.Name)

	if sel.Rel.Type == sdata.RelRemote {
		sel.Table = name
		qc.Remotes++
		return nil
	}

	co.setSingular(name, sel)
	return nil
}

func (co *Compiler) setRelFilters(qc *qcode.QCode, sel *qcode.Select) {
	rel := sel.Rel
	pid := sel.ParentID

	if len(sel.Joins) != 0 {
		pid = -1
	}

	switch rel.Type {
	case sdata.RelOneToOne, sdata.RelOneToMany:
		addAndFilter(&sel.Where, buildFilter(rel, pid))

	case sdata.RelEmbedded:
		addAndFilter(&sel.Where, buildFilter(rel, pid))

	case sdata.RelPolymorphic:
		pid = qc.Selects[sel.ParentID].ParentID
		ex := newExpOp(qcode.OpAnd)

		ex1 := newExpOp(qcode.OpEquals)
		ex1.Left.Table = sel.Ti.Name
		ex1.Left.Col = rel.Right.Col
		ex1.Right.ID = pid
		ex1.Right.Col = rel.Left.Col

		ex2 := newExpOp(qcode.OpEquals)
		ex2.Left.ID = pid
		ex2.Left.Col.Table = rel.Left.Col.Table
		ex2.Left.Col.Name = rel.Left.Col.FKeyCol
		ex2.Right.ValType = qcode.ValStr
		ex2.Right.Val = sel.Ti.Name

		ex.Children = []*qcode.Exp{ex1, ex2}
		addAndFilter(&sel.Where, ex)

	case sdata.RelRecursive:
		rcte := "__rcte_" + rel.Right.Ti.Name
		ex := newExpOp(qcode.OpAnd)
		ex1 := newExpOp(qcode.OpIsNotNull)
		ex2 := newExp()
		ex3 := newExp()

		v, _ := sel.GetInternalArg("find")
		switch v.Val {
		case "parents", "parent":
			ex1.Left.Table = rcte
			ex1.Left.Col = rel.Left.Col
			switch {
			case !rel.Left.Col.Array && rel.Right.Col.Array:
				ex2.Op = qcode.OpNotIn
				ex2.Left.Table = rcte
				ex2.Left.Col = rel.Left.Col
				ex2.Right.Table = rcte
				ex2.Right.Col = rel.Right.Col

				ex3.Op = qcode.OpIn
				ex3.Left.Table = rcte
				ex3.Left.Col = rel.Left.Col
				ex3.Right.Col = rel.Right.Col

			case rel.Left.Col.Array && !rel.Right.Col.Array:
				ex2.Op = qcode.OpNotIn
				ex2.Left.Table = rcte
				ex2.Left.Col = rel.Right.Col
				ex2.Right.Table = rcte
				ex2.Right.Col = rel.Left.Col

				ex3.Op = qcode.OpIn
				ex3.Left.Col = rel.Right.Col
				ex3.Right.Table = rcte
				ex3.Right.Col = rel.Left.Col

			default:
				ex2.Op = qcode.OpNotEquals
				ex2.Left.Table = rcte
				ex2.Left.Col = rel.Left.Col
				ex2.Right.Table = rcte
				ex2.Right.Col = rel.Right.Col

				ex3.Op = qcode.OpEquals
				ex3.Left.Col = rel.Right.Col
				ex3.Right.Table = rcte
				ex3.Right.Col = rel.Left.Col
			}

		default:
			ex1.Left.Col = rel.Left.Col
			switch {
			case !rel.Left.Col.Array && rel.Right.Col.Array:
				ex2.Op = qcode.OpNotIn
				ex2.Left.Col = rel.Left.Col
				ex2.Right.Col = rel.Right.Col

				ex3.Op = qcode.OpIn
				ex3.Left.Col = rel.Left.Col
				ex3.Right.Table = rcte
				ex3.Right.Col = rel.Right.Col

			case rel.Left.Col.Array && !rel.Right.Col.Array:
				ex2.Op = qcode.OpNotIn
				ex2.Left.Col = rel.Right.Col
				ex2.Right.Col = rel.Left.Col

				ex3.Op = qcode.OpIn
				ex3.Left.Table = rcte
				ex3.Left.Col = rel.Right.Col
				ex3.Right.Col = rel.Left.Col

			default:
				ex2.Op = qcode.OpNotEquals
				ex2.Left.Col = rel.Left.Col
				ex2.Right.Col = rel.Right.Col

				ex3.Op = qcode.OpEquals
				ex3.Left.Col = rel.Left.Col
				ex3.Right.Table = rcte
				ex3.Right.Col = rel.Right.Col
			}
		}

		ex.Children = []*qcode.Exp{ex1, ex2, ex3}
		addAndFilter(&sel.Where, ex)
	}
}

func (co *Compiler) Find(schema, name string) (sdata.DBTable, error) {
	if co.c.EnableCamelcase {
		name = strings.TrimSuffix(name, singularSuffixSnake)
	} else {
		name = strings.TrimSuffix(name, singularSuffixCamel)
	}
	return co.s.Find(schema, name)
}

func (co *Compiler) FindPath(from, to, through string) ([]sdata.TPath, error) {
	if co.c.EnableCamelcase {
		from = strings.TrimSuffix(from, singularSuffixSnake)
		to = strings.TrimSuffix(to, singularSuffixSnake)
	} else {
		from = strings.TrimSuffix(from, singularSuffixCamel)
		to = strings.TrimSuffix(to, singularSuffixCamel)
	}

	// Try normal graph path first (same-database relationships)
	path, err := co.s.FindPath(from, to, through)
	if err == nil {
		return path, nil
	}

	// Cross-DB fallback: only fire when the in-graph relationship is
	// genuinely missing (ErrPathNotFound or ErrFromEdgeNotFound /
	// ErrToEdgeNotFound — the table isn't in this database's graph).
	// Other errors — most importantly *AmbiguousPathError — must propagate
	// unchanged, otherwise the cross-DB short-circuit silently hides
	// legitimate compile-time signals from the caller.
	switch err {
	case sdata.ErrPathNotFound, sdata.ErrFromEdgeNotFound, sdata.ErrToEdgeNotFound:
		if tp, ok := co.s.FindCrossDBPath(from, to); ok {
			return []sdata.TPath{tp}, nil
		}
	}

	return nil, err
}

func (co *Compiler) FindPathByColumn(from, to, col string) ([]sdata.TPath, error) {
	if co.c.EnableCamelcase {
		from = strings.TrimSuffix(from, singularSuffixSnake)
		to = strings.TrimSuffix(to, singularSuffixSnake)
	} else {
		from = strings.TrimSuffix(from, singularSuffixCamel)
		to = strings.TrimSuffix(to, singularSuffixCamel)
	}
	return co.s.FindPathByColumn(from, to, col)
}

func buildSingleColFilter(leftCol, rightCol sdata.DBColumn, pid int32) *qcode.Exp {
	ex := newExp()
	switch {
	case !leftCol.Array && rightCol.Array:
		ex.Op = qcode.OpIn
		ex.Left.Col = leftCol
		ex.Right.ID = pid
		ex.Right.Col = rightCol

	case leftCol.Array && !rightCol.Array:
		ex.Op = qcode.OpIn
		ex.Left.ID = pid
		ex.Left.Col = rightCol
		ex.Right.Col = leftCol

	default:
		ex.Op = qcode.OpEquals
		ex.Left.Col = leftCol
		ex.Right.ID = pid
		ex.Right.Col = rightCol
	}
	return ex
}

func buildFilter(rel sdata.DBRel, pid int32) *qcode.Exp {
	switch rel.Type {
	case sdata.RelOneToOne, sdata.RelOneToMany:
		primary := buildSingleColFilter(rel.Left.Col, rel.Right.Col, pid)
		if len(rel.ExtraPairs) == 0 {
			return primary
		}
		// Composite FK: AND all column pairs together
		and := newExpOp(qcode.OpAnd)
		and.Children = append(and.Children, primary)
		for _, pair := range rel.ExtraPairs {
			and.Children = append(and.Children, buildSingleColFilter(pair.L, pair.R, pid))
		}
		return and

	case sdata.RelEmbedded:
		ex := newExpOp(qcode.OpEquals)
		ex.Left.Col = rel.Right.Col
		ex.Right.ID = pid
		ex.Right.Col = rel.Right.Col
		return ex

	default:
		return nil
	}
}

func (co *Compiler) setSingular(fieldName string, sel *qcode.Select) {
	if sel.Singular {
		return
	}

	if len(sel.Joins) != 0 {
		return
	}

	if (sel.Rel.Type == sdata.RelOneToMany && !sel.Rel.Right.Col.Array) ||
		sel.Rel.Type == sdata.RelPolymorphic {
		sel.Singular = true
		return
	}
}

func (co *Compiler) setLimit(qc *qcode.QCode, sel *qcode.Select) {
	if sel.Paging.Limit != 0 || sel.Paging.NoLimit {
		return
	}
	if co.c.AnalyticsMode {
		sel.Paging.NoLimit = true
		return
	}
	if co.c.DefaultLimit != 0 {
		sel.Paging.Limit = int32(co.c.DefaultLimit)
		return
	}
	sel.Paging.Limit = 20
}

// This
// (A, B, C) >= (X, Y, Z)
//
// Becomes
// (A > X)
//   OR ((A = X) AND (B > Y))
//   OR ((A = X) AND (B = Y) AND (C > Z))
//   OR ((A = X) AND (B = Y) AND (C = Z)

func (co *Compiler) addSeekPredicate(sel *qcode.Select) {
	var or, and *qcode.Exp
	obLen := len(sel.OrderBy)

	if obLen != 0 {
		ob := sel.OrderBy[0]
		or = newExpOp(qcode.OpOr)

		isnull := newExpOp(qcode.OpIsNull)
		isnull.Left.Table = "__cur"
		isnull.Left.Col = ob.Col

		if ob.Key != "" {
			isnull.Left.ColName = ob.Col.Name + "_" + ob.Key
		}

		or.Children = []*qcode.Exp{isnull}
	}

	for i := 0; i < obLen; i++ {
		if i != 0 {
			and = newExpOp(qcode.OpAnd)
		}

		for n, ob := range sel.OrderBy {
			if n > i {
				break
			}

			f := newExp()
			f.Left.Col = ob.Col
			f.Right.Table = "__cur"
			f.Right.Col = ob.Col

			if ob.Key != "" {
				f.Right.ColName = ob.Col.Name + "_" + ob.Key
			}

			switch {
			case i > 0 && n != i:
				f.Op = qcode.OpEquals
			case ob.Order == qcode.OrderDesc ||
				ob.Order == qcode.OrderDescNullsFirst || ob.Order == qcode.OrderDescNullsLast:
				f.Op = qcode.OpLesserThan
			case ob.Order == qcode.OrderAsc ||
				ob.Order == qcode.OrderAscNullsLast || ob.Order == qcode.OrderAscNullsFirst:
				f.Op = qcode.OpGreaterThan
			default:
				f.Op = qcode.OpGreaterThan
			}

			// could be null needs to be handled
			if !ob.Col.NotNull {
				isnull1 := newExpOp(qcode.OpIsNull)
				isnull1.Left.Table = "__cur"
				isnull1.Left.Col = ob.Col

				isnull2 := newExpOp(qcode.OpIsNull)
				isnull2.Left.Col = ob.Col

				if ob.Key != "" {
					isnull1.Left.ColName = ob.Col.Name + "_" + ob.Key
				}

				or1 := newExpOp(qcode.OpOr)
				or1.Children = append(or.Children, isnull1, isnull2, f)

				// now that f is added to the above or1 we can set f to or1
				f = or1
			}

			if and != nil {
				and.Children = append(and.Children, f)
			} else {
				or.Children = append(or.Children, f)
			}
		}

		if and != nil {
			or.Children = append(or.Children, and)
		}
	}
	addAndFilter(&sel.Where, or)
}

func (co *Compiler) validateSelect(sel *qcode.Select) error {
	if sel.Rel.Type == sdata.RelRecursive {
		v, ok := sel.GetInternalArg("find")
		if !ok {
			return fmt.Errorf("argument 'find' needed for recursive queries")
		}
		if v.Val != "parents" && v.Val != "children" {
			return fmt.Errorf("valid values for 'find' are 'parents' and 'children'")
		}
	}
	return nil
}

func (co *Compiler) checkPartitionFilter(qc *qcode.QCode, sel *qcode.Select) {
	if qc.Type != qcode.QTQuery && qc.Type != qcode.QTSubscription {
		return
	}

	if co.c.AnalyticsMode {
		co.enforcePartitionFilterOLAP(sel)
		return
	}

	if sel.Ti.PartitionKey == "" {
		return
	}
	if qcode.HasFilterOnColumn(sel.Where.Exp, sel.Ti.PartitionKey) {
		return
	}

	if sel.Ti.PartitionRangeDays > 0 {
		cid, ok := sel.Ti.GetColumnIndex(sel.Ti.PartitionKey)
		if !ok {
			qc.Warnings = append(qc.Warnings,
				fmt.Sprintf("partition column %q not found in table %q",
					sel.Ti.PartitionKey, sel.Ti.Name))
			return
		}
		col := sel.Ti.Columns[cid]

		ex := &qcode.Exp{Op: qcode.OpGreaterOrEquals}
		ex.Left.Col = col
		ex.Right.ValType = qcode.ValPartitionBound
		ex.Right.Val = strconv.Itoa(sel.Ti.PartitionRangeDays)
		addAndFilter(&sel.Where, ex)
	} else {
		qc.Warnings = append(qc.Warnings,
			fmt.Sprintf("query on %q has no filter on partition column %q — this may scan all partitions",
				sel.Ti.Name, sel.Ti.PartitionKey))
	}
}

func (co *Compiler) enforcePartitionFilterOLAP(sel *qcode.Select) {
	if sel.Ti.PartitionNone {
		return
	}

	if sel.Ti.PartitionKey != "" {
		if qcode.HasFilterOnColumn(sel.Where.Exp, sel.Ti.PartitionKey) {
			return
		}
		sel.PartitionFilterRequired = fmt.Sprintf(
			"table %q requires a filter on partition column %q. Add one of: where: { %s: { gte: \"<date>\" } }; or pass unrestricted: true to override.",
			sel.Ti.Name, sel.Ti.PartitionKey, sel.Ti.PartitionKey)
		return
	}

	if sel.Ti.ImplicitPartitionKey != "" {
		if qcode.HasFilterOnColumn(sel.Where.Exp, sel.Ti.ImplicitPartitionKey) || sel.Unrestricted {
			return
		}
		sel.PartitionFilterRequired = fmt.Sprintf(
			"table %q requires a filter on temporal column %q. Add one of: where: { %s: { gte: \"<date>\" } }; or pass unrestricted: true to override.",
			sel.Ti.Name, sel.Ti.ImplicitPartitionKey, sel.Ti.ImplicitPartitionKey)
	}
}

func (co *Compiler) setMutationType(qc *qcode.QCode, op *graph.Operation) error {
	var err error

	validateActionArg := func(arg graph.Arg) error {
		v := arg.Val
		if v.Type != graph.NodeVar && v.Type != graph.NodeObj &&
			(v.Type != graph.NodeList || len(v.Children) == 0 && v.Children[0].Type != graph.NodeObj) {
			return argErr(arg, "variable, an object or a list of objects")
		}
		return nil
	}

	// Collect all root fields
	var rootFields []graph.Field
	for _, f := range op.Fields {
		if f.ParentID == -1 {
			rootFields = append(rootFields, f)
		}
	}

	if len(rootFields) == 0 {
		return errors.New(`mutations must contains one of the following arguments (insert, update, upsert or delete)`)
	}

	co.actionArgs = make(map[string]graph.Arg, len(rootFields))

	for ri, rf := range rootFields {
		var fieldType qcode.QType
		var actionArg graph.Arg
		var conflictAction qcode.ConflictAction

		for _, arg := range rf.Args {
			switch arg.Name {
			case "insert":
				fieldType = qcode.QTInsert
				actionArg = arg
				err = validateActionArg(arg)
			case "update":
				fieldType = qcode.QTUpdate
				actionArg = arg
				err = validateActionArg(arg)
			case "upsert":
				fieldType = qcode.QTUpsert
				actionArg = arg
				err = validateActionArg(arg)
			case "delete":
				fieldType = qcode.QTDelete
				if ifNotArg(arg, graph.NodeBool) || ifNotArgVal(arg, "true") {
					err = errors.New("value for 'delete' must be 'true'")
				}
			case "on_conflict", "onConflict":
				if arg.Val == nil || (arg.Val.Type != graph.NodeLabel && arg.Val.Type != graph.NodeStr) {
					err = errors.New("value for 'on_conflict' must be 'get'")
					break
				}
				if arg.Val.Val != "get" {
					err = fmt.Errorf("unsupported on_conflict action %q; valid action: get", arg.Val.Val)
					break
				}
				conflictAction = qcode.ConflictGet
			}

			if err != nil {
				return err
			}
		}

		if fieldType == qcode.QTUnknown {
			return errors.New(`mutations must contains one of the following arguments (insert, update, upsert or delete)`)
		}
		if conflictAction != qcode.ConflictNone && fieldType != qcode.QTInsert {
			return errors.New("on_conflict is only valid with insert")
		}

		if ri == 0 {
			qc.SType = fieldType
			if actionArg.Val != nil {
				qc.ActionVar = actionArg.Val.Val
			}
			co.actionArg = actionArg
		} else if fieldType != qc.SType {
			return errors.New("all root mutations must be of the same type (insert, update, upsert or delete)")
		}
		if conflictAction != qcode.ConflictNone {
			if qc.InsertConflictAction != qcode.ConflictNone {
				return errors.New("on_conflict: get supports exactly one root insert")
			}
			qc.InsertConflictAction = conflictAction
		}

		// Key by alias if present, otherwise by field name
		key := rf.Alias
		if key == "" {
			key = rf.Name
		}
		co.actionArgs[key] = actionArg
	}
	if qc.InsertConflictAction != qcode.ConflictNone && len(rootFields) != 1 {
		return errors.New("on_conflict: get supports exactly one root insert")
	}

	return nil
}

func (co *Compiler) compileArgFilter(qc *qcode.QCode, sel *qcode.Select,
	selID int32, arg graph.Arg,
) (ex *qcode.Exp, err error) {
	st := util.NewStackInf()

	if arg.Val.Type != graph.NodeObj {
		err = fmt.Errorf("expecting an object")
		return
	}

	ex, _, err = co.compileExpNode(sel.Table,
		sel.Ti, st, arg.Val, false, selID)
	if err != nil {
		return
	}
	if err = co.validateUserFilter(qc, sel, ex); err != nil {
		return nil, err
	}

	return
}

func (co *Compiler) validateUserFilter(qc *qcode.QCode, sel *qcode.Select, ex *qcode.Exp) error {
	if ex == nil {
		return nil
	}

	if err := co.validateUserFilterColumn(qc, sel, ex.Left.Col); err != nil {
		return err
	}
	if ex.Right.ValType == qcode.ValRef {
		if err := co.validateUserFilterColumn(qc, sel, ex.Right.Col); err != nil {
			return err
		}
	}
	for _, child := range ex.Children {
		if err := co.validateUserFilter(qc, sel, child); err != nil {
			return err
		}
	}
	for _, arm := range ex.CaseArms {
		if err := co.validateUserFilter(qc, sel, arm.When); err != nil {
			return err
		}
		if err := co.validateUserFilter(qc, sel, arm.Then); err != nil {
			return err
		}
	}
	return co.validateUserFilter(qc, sel, ex.Else)
}

func (co *Compiler) validateUserFilterColumn(
	qc *qcode.QCode,
	sel *qcode.Select,
	col sdata.DBColumn,
) error {
	if col.Name == "" {
		return nil
	}
	if col.Blocked {
		return fmt.Errorf("column: '%s.%s.%s' blocked", col.Schema, col.Table, col.Name)
	}
	return nil
}

func addAndFilterLast(fil *qcode.Filter, ex *qcode.Exp) {
	if fil.Exp == nil {
		fil.Exp = ex
		return
	}
	// save exiting exp pointer (could be a common one from filter config)
	ow := fil.Exp

	// add a new `and` exp and hook the above saved exp pointer a child
	// we don't want to modify an exp object thats common (from filter config)
	fil.Exp = newExpOp(qcode.OpAnd)
	fil.Exp.SetPooledChildren(2)

	// here we append the filter to the last child
	fil.Exp.Children[0] = ow
	fil.Exp.Children[1] = ex
}

func addAndFilter(fil *qcode.Filter, ex *qcode.Exp) {
	if fil.Exp == nil {
		fil.Exp = ex
		return
	}
	// save exiting exp pointer (could be a common one from filter config)
	ow := fil.Exp

	// add a new `and` exp and hook the above saved exp pointer a child
	// we don't want to modify an exp object thats common (from filter config)
	fil.Exp = newExpOp(qcode.OpAnd)
	fil.Exp.SetPooledChildren(2)
	fil.Exp.Children[0] = ex
	fil.Exp.Children[1] = ow
}

func addNotFilter(fil *qcode.Filter, ex *qcode.Exp) {
	ex1 := newExpOp(qcode.OpNot)
	ex1.SetPooledChildren(1)
	ex1.Children[0] = ex

	if fil.Exp == nil {
		fil.Exp = ex1
		return
	}
	// save exiting exp pointer (could be a common one from filter config)
	ow := fil.Exp

	// add a new `and` exp and hook the above saved exp pointer a child
	// we don't want to modify an exp object thats common (from filter config)
	fil.Exp = newExpOp(qcode.OpAnd)
	fil.Exp.SetPooledChildren(2)
	fil.Exp.Children[0] = ex1
	fil.Exp.Children[1] = ow
}

func getArg(args []graph.Arg, name string, validTypes ...graph.ParserType,
) (arg graph.Arg, err error) {
	var ok bool
	arg, ok, err = getOptionalArg(args, name, validTypes...)
	if err != nil {
		return
	}
	if !ok {
		err = reqArgMissing(name)
	}
	return
}

func getOptionalArg(args []graph.Arg, name string, validTypes ...graph.ParserType,
) (arg graph.Arg, ok bool, err error) {
	for _, arg = range args {
		if arg.Name != name {
			continue
		}
		if err = validateArg(arg, validTypes...); err != nil {
			return
		}
		ok = true
		return
	}
	return
}

// todo: add support for list of types
func validateArg(arg graph.Arg, validTypes ...graph.ParserType) (err error) {
	n := len(validTypes)
	for i := 0; i < n; i++ {
		vt := validTypes[i]
		ty := arg.Val.Type

		switch {
		case vt == graph.NodeList && ty != vt:
			continue
		case vt == graph.NodeList && ty == vt:
			if len(arg.Val.Children) == 0 {
				return
			}
			if (i + 1) >= n {
				continue
			}
			childType := arg.Val.Children[0].Type
			if childType == validTypes[(i+1)] {
				return
			}
			i++
			continue
		}

		if ty == graph.NodeStr && arg.Val.Val == "" {
			continue
		}
		if ty == vt {
			return
		}
	}
	err = argErr(arg, argTypes(validTypes))
	return
}

func reqArgMissing(name string) (err error) {
	return fmt.Errorf("required argument '%s' missing", name)
}

func unknownArg(arg graph.Arg) (err error) {
	return fmt.Errorf("unknown argument '%s'", arg.Name)
}

func ifNotArg(arg graph.Arg, ty graph.ParserType) (ok bool) {
	return arg.Val.Type != ty
}

func ifNotArgVal(arg graph.Arg, val string) bool {
	return arg.Val.Val != val
}

func argTypes(types []graph.ParserType) string {
	var sb strings.Builder
	var list bool
	lastIndex := len(types) - 1
	for i, t := range types {
		if !list {
			if i == lastIndex {
				sb.WriteString(" or ")
			} else if i != 0 {
				sb.WriteString(", ")
			}
		}
		if t == graph.NodeList {
			sb.WriteString("a list of ")
			list = true
			continue
		}
		if !list {
			sb.WriteString("a ")
		}
		switch t {
		case graph.NodeBool:
			sb.WriteString("boolean")
		case graph.NodeNum:
			sb.WriteString("number")
		case graph.NodeLabel, graph.NodeStr:
			sb.WriteString("string")
		case graph.NodeObj:
			sb.WriteString("object")
		case graph.NodeVar:
			sb.WriteString("variable")
		}
		if list {
			sb.WriteString("s")
			list = false
		}

	}
	return sb.String()
}

func argErr(arg graph.Arg, ty string) error {
	return fmt.Errorf("value for argument '%s' must be %s", arg.Name, ty)
}

func dbArgErr(name, ty, db string) error {
	return fmt.Errorf("%s: value for argument '%s' must be a %s", db, name, ty)
}
