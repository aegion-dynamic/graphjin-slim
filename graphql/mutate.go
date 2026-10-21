package graphql

import (
	"bytes"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"

	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/util"
)

var errUserIDReq = errors.New("$user_id required for this query")

// mutItem is frontend plumbing around the pure IR Mutate: parse output
// and tree-building scratch that backends never see.
type mutItem struct {
	qcode.Mutate
	md       mData
	children []int32
	render   bool
	// nested are the payload keys that name a relation rather than a
	// column. They are not columns of this table.
	nested map[string]struct{}
}

// const (
// 	CTConnect uint8 = 1 << iota
// 	CTDisconnect
// )

var insertTypes = map[string]qcode.MType{
	"connect": qcode.MTConnect,
	"find":    qcode.MTKeyword,
}

var updateTypes = map[string]qcode.MType{
	"where":      qcode.MTKeyword,
	"find":       qcode.MTKeyword,
	"connect":    qcode.MTConnect,
	"disconnect": qcode.MTDisconnect,
}

type mState struct {
	st        *util.StackInf
	qc        *qcode.QCode
	mt        qcode.MType
	id        int32
	rootSelID int32
	// vars are the values of the request's variables. A payload key bound
	// to a variable is resolved with them, because the value is what says
	// whether the key is a column or a nested relation.
	vars map[string]json.RawMessage
}

func (co *Compiler) compileMutation(scr *mutState, qc *qcode.QCode,
	vmap map[string]json.RawMessage,
) (err error) {
	if qc.ActionVar != "" {
		qc.ActionVal = vmap[qc.ActionVar]
	}

	var whereReq bool

	switch qc.SType {
	case qcode.QTInsert:
	case qcode.QTUpdate:
		whereReq = true
	case qcode.QTUpsert:
		whereReq = true
	case qcode.QTDelete:
		whereReq = true
	default:
		return errors.New("valid mutations: insert, update, upsert, delete'")
	}

	mutates := []mutItem{}
	mmap := map[int32]int32{-1: -1}
	st := util.NewStackInf()
	var nextID int32

	// Process each root select as a separate root mutation
	for _, rootID := range qc.Roots {
		sel := &qc.Selects[rootID]

		if whereReq && sel.Where.Exp == nil {
			return errors.New("where clause required")
		}

		m := qcode.Mutate{
			Field:    qcode.Field{Type: qcode.FieldTypeTable},
			ID:       nextID,
			ParentID: -1,
			Key:      sel.Table,
			Ti:       sel.Ti,
			SelID:    rootID,
		}

		switch qc.SType {
		case qcode.QTInsert:
			m.Type = qcode.MTInsert
		case qcode.QTUpdate:
			m.Type = qcode.MTUpdate
		case qcode.QTUpsert:
			m.Type = qcode.MTUpsert
		case qcode.QTDelete:
			m.Type = qcode.MTDelete
		}

		mi := mutItem{Mutate: m}

		if mi.Type == qcode.MTDelete {
			mi.render = true
			st.Push(mi)
			continue
		}

		mi.md, err = co.parseMutationDataFromArg(scr, qc, sel.FieldName, vmap)
		if err != nil {
			return err
		}

		if mi.md.Data.Type == graph.NodeList {
			items := co.processList(mi)
			for _, v := range items {
				st.Push(v)
			}
			// Every element of the list is its own mutation, so the next
			// root starts after all of them. Sharing an id would give two
			// mutations the same name and the same result set.
			nextID += int32(len(items))
		} else {
			st.Push(mi)
			nextID++
		}
	}

	// Convert QType to MType for mState
	var mt qcode.MType
	switch qc.SType {
	case qcode.QTInsert:
		mt = qcode.MTInsert
	case qcode.QTUpdate:
		mt = qcode.MTUpdate
	case qcode.QTUpsert:
		mt = qcode.MTUpsert
	case qcode.QTDelete:
		mt = qcode.MTDelete
	}
	msID := int32(st.Len() + 1)
	if nextID > msID {
		msID = nextID
	}
	ms := mState{st: st, qc: qc, mt: mt, id: msID, vars: vmap}

	for {
		if st.Len() == 0 {
			break
		}

		intf := st.Pop()
		item, ok := intf.(mutItem)

		if ok && item.render {
			id := int32(len(mutates))
			mmap[item.ID] = id
			mutates = append(mutates, item)
			continue
		}

		ms.rootSelID = item.SelID
		if err := co.newMutate(&ms, item); err != nil {
			return err
		}
	}

	// The mutations of one root and one table read back as one result set.
	// A root reads the rows it wrote, so a mutation with several roots on
	// one table gives each root its own result set.
	groups := map[mutGroup][]int32{}
	tableRoots := map[string]map[int32]struct{}{}

	for i := range mutates {
		m1 := &mutates[i]

		// Re-id all items to make array access easy.
		m1.ID = mmap[m1.ID]
		m1.ParentID = mmap[m1.ParentID]

		if m1.Type != qcode.MTNone {
			g := mutGroup{selID: m1.SelID, table: m1.Ti.Name}
			groups[g] = append(groups[g], m1.ID)
			roots, ok := tableRoots[m1.Ti.Name]
			if !ok {
				roots = map[int32]struct{}{}
				tableRoots[m1.Ti.Name] = roots
			}
			roots[m1.SelID] = struct{}{}
		}

		for id := range m1.DependsOn {
			delete(m1.DependsOn, id)
			m1.DependsOn[mmap[id]] = struct{}{}
		}

		for i, id := range m1.children {
			m1.children[i] = mmap[id]
		}
	}
	qc.MUnions = make(map[string][]int32, len(groups))
	for g, ids := range groups {
		name := unionName(g, tableRoots)
		qc.MUnions[name] = ids
		// The root reads the result set of the table it writes. The
		// mutations it nests write other tables, and those have result sets
		// of their own.
		if int(g.selID) >= len(qc.Selects) {
			continue
		}
		sel := &qc.Selects[g.selID]
		if g.table == sel.Table && name != sel.Table {
			sel.MutationCTE = name
		}
	}

	// Pull up children of MTNone to the depends-on of it's parent if applicable.
	for i := range mutates {
		m1 := &mutates[i]

		// A mutation that writes one table more than once needs a name of
		// its own for each write, and so does a table that more than one
		// root writes to.
		g := mutGroup{selID: m1.SelID, table: m1.Ti.Name}
		if len(groups[g]) > 1 || len(tableRoots[m1.Ti.Name]) > 1 {
			m1.Multi = true
		}

		if m1.Type == qcode.MTNone && m1.ParentID != -1 {
			p := &mutates[m1.ParentID]
			delete(p.DependsOn, m1.ID)

			for _, id := range m1.children {
				m2 := &mutates[id]
				if m2.Rel.Type == sdata.RelOneToMany {
					p.DependsOn[m2.ID] = struct{}{}
				}
			}
		}
	}
	// Snapshot frontend scratch for post-compile inspection
	// (e.g. configureInsertConflict) before values are stripped down to
	// pure IR.
	scr.mutMeta = make(map[int32]*mutItem, len(mutates))
	for i := range mutates {
		scr.mutMeta[mutates[i].ID] = &mutates[i]
	}

	qc.Mutates = make([]qcode.Mutate, 0, len(mutates))
	for _, mi := range mutates {
		if !mi.render {
			continue
		}
		mi.IsJSON = mi.md.IsJSON
		mi.Array = mi.md.Array
		mi.Var = mi.md.Var
		if mi.Var != "" {
			qc.PerRequest = true
		}
		mi.ColVals = co.colValsFromNode(mi.Ti, mi.md.Data)
		qc.Mutates = append(qc.Mutates, mi.Mutate)
	}
	return co.configureInsertConflict(scr, qc)
}

func (co *Compiler) configureInsertConflict(scr *mutState, qc *qcode.QCode) error {
	if qc.InsertConflictAction == qcode.ConflictNone {
		return nil
	}
	if qc.SType != qcode.QTInsert {
		return errors.New("on_conflict is only valid with insert")
	}
	if scr.actionArg.Val != nil && scr.actionArg.Val.Type == graph.NodeList {
		return errors.New("on_conflict: get does not support bulk or nested inserts")
	}
	if strings.HasPrefix(strings.TrimSpace(string(qc.ActionVal)), "[") {
		return errors.New("on_conflict: get does not support bulk or nested inserts")
	}
	if len(qc.Roots) != 1 || len(qc.Mutates) != 1 {
		return errors.New("on_conflict: get does not support bulk or nested inserts")
	}

	m := &qc.Mutates[0]
	mi := scr.mutMeta[m.ID]
	if m.Type != qcode.MTInsert || m.ParentID != -1 || m.Array || mi == nil || mi.md.Data == nil || mi.md.Data.Type != graph.NodeObj || len(mi.children) != 0 || len(m.RCols) != 0 {
		return errors.New("on_conflict: get does not support bulk or nested inserts")
	}
	// Reject nested objects/lists in the payload: on_conflict: get only
	// supports a flat single-row insert.
	for _, c := range mi.md.Data.Children {
		if c.Type == graph.NodeObj || c.Type == graph.NodeList {
			return errors.New("on_conflict: get does not support bulk or nested inserts")
		}
	}

	byName := make(map[string]qcode.MColumn, len(m.Cols))
	for _, col := range m.Cols {
		byName[col.Col.Name] = col
	}

	var candidates [][]qcode.MColumn
	if len(m.Ti.PrimaryCols) != 0 {
		pk := make([]qcode.MColumn, 0, len(m.Ti.PrimaryCols))
		for _, col := range m.Ti.PrimaryCols {
			mc, ok := byName[col.Name]
			if !ok {
				pk = nil
				break
			}
			pk = append(pk, mc)
		}
		if len(pk) != 0 {
			candidates = append(candidates, pk)
		}
	}

	// A unique key qualifies when the payload supplies every one of its
	// columns. A key is a property of the tuple, so a composite key needs
	// all of its columns, and a single column key needs its one.
	for _, key := range m.Ti.UniqueKeys {
		cols := make([]qcode.MColumn, 0, len(key))
		for _, name := range key {
			mc, ok := byName[name]
			if !ok {
				cols = nil
				break
			}
			cols = append(cols, mc)
		}
		if len(cols) != 0 {
			candidates = append(candidates, cols)
		}
	}

	// A column that is unique on its own is a key of one column. A column of
	// a composite key is not: it is only unique together with the others.
	for _, col := range m.Ti.Columns {
		if col.PrimaryKey || !col.UniqueKey {
			continue
		}
		if !columnIsPartOfCompositeKey(m.Ti, col.Name) {
			if mc, ok := byName[col.Name]; ok {
				candidates = append(candidates, []qcode.MColumn{mc})
			}
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("on_conflict: get on table %q requires a supplied primary or unique key", m.Ti.Name)
	}
	if len(candidates) > 1 {
		// The loosest key is the one that conflicts first, so it is the one
		// that returns the existing row. Two keys of the same width are
		// genuinely ambiguous.
		loosest := candidates[0]
		for _, candidate := range candidates[1:] {
			if len(candidate) < len(loosest) {
				loosest = candidate
			}
		}
		tied := 0
		for _, candidate := range candidates {
			if len(candidate) == len(loosest) {
				tied++
			}
		}
		if tied > 1 {
			names := make([]string, 0, len(candidates))
			for _, candidate := range candidates {
				parts := make([]string, len(candidate))
				for i := range candidate {
					parts[i] = candidate[i].Col.Name
				}
				names = append(names, strings.Join(parts, "+"))
			}
			return fmt.Errorf("on_conflict: get on table %q is ambiguous; supplied unique keys: %s", m.Ti.Name, strings.Join(names, ", "))
		}
		candidates = [][]qcode.MColumn{loosest}
	}

	m.ConflictAction = qcode.ConflictGet
	m.ConflictCols = candidates[0]
	return nil
}

// columnIsPartOfCompositeKey reports whether the column is one of the
// columns of a key with more than one column. Such a column is not unique on
// its own, so it cannot stand for a key by itself.
func columnIsPartOfCompositeKey(ti sdata.DBTable, name string) bool {
	for _, key := range ti.UniqueKeys {
		if len(key) < 2 {
			continue
		}
		for _, k := range key {
			if k == name {
				return true
			}
		}
	}
	return false
}

// mutGroup is the set of mutations one root writes to one table.
type mutGroup struct {
	selID int32
	table string
}

// unionName is the name of the result set a root reads back.
//
// A table that one root writes to keeps the table's own name, so nothing
// changes for the common case. A table that several roots write to gives
// each root a name of its own, because a root must read the rows it wrote
// and not the rows the others wrote.
func unionName(g mutGroup, tableRoots map[string]map[int32]struct{}) string {
	if len(tableRoots[g.table]) <= 1 {
		return g.table
	}
	return g.table + "__" + strconv.Itoa(int(g.selID))
}

type mData struct {
	Data   *graph.Node
	IsJSON bool
	Array  bool
	// Var is the variable the payload is read from, when it is not the
	// operation's payload variable. See qcode.Mutate.Var.
	Var string
}

func parseDataValue(qc *qcode.QCode, actionVal *graph.Node, isJSON bool) (mData, error) {
	var md mData
	md.Data = actionVal

	// md, err := parseMutationData(qc, actionVal)
	// if err != nil {
	// 	return md, err
	// }

	if isJSON {
		md.IsJSON = isJSON
	}
	return md, nil
}

func (co *Compiler) parseMutationData(scr *mutState, qc *qcode.QCode) (mData, error) {
	var md mData
	var err error

	av := scr.actionArg.Val
	switch av.Type {
	case graph.NodeVar:
		if len(qc.ActionVal) == 0 {
			if err := co.missingPayload(scr, av.Val); err != nil {
				return md, err
			}
			md.Data = emptyPayload()
			md.IsJSON = true
			return md, nil
		}
		md.Data, err = graph.ParseArgValue(string(qc.ActionVal), true)
		if err != nil {
			return md, err
		}
		md.IsJSON = true

	default:
		md.Data = av
	}
	return md, nil
}

// emptyPayload is a payload with no columns.
func emptyPayload() *graph.Node {
	return &graph.Node{Type: graph.NodeObj, CMap: map[string]*graph.Node{}}
}

// missingPayload reports the payload variable that the operation needs but
// the request did not supply.
//
// A description has no value to give, so in shape only mode the payload is
// compiled empty: the operation still describes its shape, it just has no
// columns. Executing it would write nothing, which is why the mode is for
// description alone.
func (co *Compiler) missingPayload(scr *mutState, name string) error {
	if !scr.shapeOnly {
		return fmt.Errorf("variable not found: %s", name)
	}
	return nil
}

func (co *Compiler) parseMutationDataFromArg(scr *mutState, qc *qcode.QCode, key string, vmap map[string]json.RawMessage) (mData, error) {
	var md mData
	var err error

	arg, ok := scr.actionArgs[key]
	if !ok {
		return co.parseMutationData(scr, qc)
	}

	av := arg.Val
	if av == nil {
		return co.parseMutationData(scr, qc)
	}

	multiRoot := len(qc.Roots) > 1

	switch av.Type {
	case graph.NodeVar:
		val := vmap[av.Val]
		if len(val) == 0 {
			if err := co.missingPayload(scr, av.Val); err != nil {
				return md, err
			}
			md.Data = emptyPayload()
			md.IsJSON = true
			return md, nil
		}
		md.Data, err = graph.ParseArgValue(string(val), true)
		if err != nil {
			return md, err
		}
		md.IsJSON = true
		// A mutation with several roots has a payload variable per root, so
		// each root reads its own. The values are bound, never inlined: the
		// statement is compiled once and reused for every request.
		if multiRoot {
			md.Var = av.Val
		}

	default:
		md.Data = av
	}
	return md, nil
}

// connectFilter returns the filter of a connect, when the payload is one.
//
// A connect names a relation and a filter: the relation says which table to
// look in and the filter says which row.
func connectFilter(v *graph.Node) (*graph.Node, bool) {
	if v == nil || v.CMap == nil {
		return nil, false
	}
	c, ok := v.CMap["connect"]
	if !ok || c == nil || c.Type != graph.NodeObj {
		return nil, false
	}
	return c, true
}

// newConnectMutate builds the lookup of a connect.
//
// The lookup holds the parent's key column, and the child's foreign key is
// bound to it. The relation is turned around, because the lookup reads the
// parent and the relation is written from the child.
func (co *Compiler) newConnectMutate(ms *mState, m *mutItem, key string,
	rel sdata.DBRel, filter *graph.Node,
) (mutItem, error) {
	// The relation is written from the child to the parent, but the path
	// may be resolved from either side, so the child is found by its table.
	childFK, parentID := rel.Left.Col, rel.Right.Col
	parent := rel.Right.Ti
	if rel.Left.Ti.Schema != m.Ti.Schema || rel.Left.Ti.Name != m.Ti.Name {
		childFK, parentID = rel.Right.Col, rel.Left.Col
		parent = rel.Left.Ti
	}

	// The lookup reads the parent, so its key column is on the left and the
	// child's foreign key on the right.
	lookup := rel
	lookup.Left.Col = parentID
	lookup.Right.Col = childFK

	mi := mutItem{Mutate: qcode.Mutate{
		ID:       ms.id,
		ParentID: m.ID,
		Type:     qcode.MTConnect,
		Key:      key,
		Path:     append(m.Path, key),
		Rel:      lookup,
		Ti:       parent,
	}, md: mData{Data: filter}}
	ms.id++

	// The filter says which parent row the connect finds.
	st := util.NewStackInf()
	// The node's name is not copied: it is the key the filter had in the
	// payload, and a filter's name is a table, not a key.
	node := &graph.Node{
		Type:     filter.Type,
		Children: filter.Children,
		CMap:     filter.CMap,
	}
	exp, _, err := co.compileBaseExpNode("", parent, st, node, false)
	if err != nil {
		return mi, err
	}
	mi.Where.Exp = exp

	return mi, nil
}

// resolveVarValue returns the value of the variable a payload key is bound
// to, parsed the way a payload is parsed.
//
// It reports false when the variable has no value, or when the value is not
// an object or a list. A payload key whose value is a plain value is a
// column, and the renderer binds the variable itself.
func (co *Compiler) resolveVarValue(ms *mState, node *graph.Node) (*graph.Node, bool) {
	if node == nil || node.Type != graph.NodeVar {
		return nil, false
	}
	raw, ok := ms.vars[node.Val]
	if !ok || len(raw) == 0 {
		return nil, false
	}
	value, err := graph.ParseArgValue(string(raw), true)
	if err != nil {
		return nil, false
	}
	if value.Type != graph.NodeObj && value.Type != graph.NodeList {
		return nil, false
	}
	return value, true
}

// TODO: Handle cases where a column name matches the child table name
// the child path needs to be exluded in the json sent to insert or update

func (co *Compiler) newMutate(ms *mState, m mutItem) error {
	if m.Type == qcode.MTConnect || m.Type == qcode.MTDisconnect {
		// A connect is a lookup, not a write. Its filter is already
		// compiled and there is no payload to process.
		m.render = true
		ms.st.Push(m)
		return nil
	}

	data := m.md.Data

	items, err := co.processNestedMutations(ms, &m, data)
	if err != nil {
		return err
	}

	if err := co.addTablesAndColumns(&m, items, data); err != nil {
		return err
	}

	m.render = true

	// For inserts order the children according to
	// the creation order required by the parent-to-child
	// relationships. For example users need to be created
	// before the products they own.

	// For updates the order defined in the query must be
	// the order used.
	switch m.Type {
	case qcode.MTInsert:
		for _, v := range items {
			if v.Type == qcode.MTConnect {
				continue
			}
			if v.Rel.Type == sdata.RelOneToMany {
				ms.st.Push(v)
			}
		}
		for _, v := range items {
			if v.Rel.Type == sdata.RelOneToOne && v.Type != qcode.MTConnect {
				ms.st.Push(v)
			}
		}
		ms.st.Push(m)
		// A connect is a lookup the insert reads, so it is written first.
		for _, v := range items {
			if v.Type == qcode.MTConnect {
				ms.st.Push(v)
			}
		}

	case qcode.MTUpdate:
		for _, v := range items {
			ms.st.Push(v)
		}
		ms.st.Push(m)

	case qcode.MTUpsert:
		ms.st.Push(m)

	case qcode.MTNone:
		for _, v := range items {
			ms.st.Push(v)
		}
		ms.st.Push(m)
	}
	return nil
}

func (co *Compiler) processNestedMutations(ms *mState, m *mutItem, data *graph.Node) ([]mutItem, error) {
	var ml []mutItem
	var md mData
	var err error

	items := make([]mutItem, 0, len(data.Children))

	for i := range data.Children {
		v := data.Children[i]

		// Inherit the parent's JSON-input flag from md (the Mutate.IsJSON
		// field is only populated at the end of compilation).
		if md, err = parseDataValue(ms.qc, v, m.md.IsJSON); err != nil {
			return nil, err
		}
		md.Var = m.md.Var
		// A full slice expression, so siblings never share a backing array.
		path := append(m.Path[:len(m.Path):len(m.Path)], co.ParseName(v.Name))

		if md.Data.Type == graph.NodeVar {
			// A payload key bound to a variable: the value says what the key
			// is. A list of objects is a nested relation, and a plain value
			// is a column.
			resolved, ok := co.resolveVarValue(ms, md.Data)
			if !ok {
				continue
			}
			// The value only gives the payload its shape. The rows are read
			// from the variable itself, so every request writes its own:
			// the statement is compiled once and reused.
			md.Var = md.Data.Val
			md.Data = resolved
			md.IsJSON = true
			path = nil
		}

		if md.Data.Type != graph.NodeObj && md.Data.Type != graph.NodeList {
			continue
		}

		k := co.ParseName(v.Name)

		// Get child-to-parent relationship
		paths, err := co.FindPath(k, m.Key, "")
		// no relationship found must be a keyword
		if err != nil {
			// An object- or array-valued key can still be a plain column:
			// json/jsonb columns legitimately hold documents and arrays
			// (e.g. form_fields.options), and SQL array columns accept lists
			// of scalars. Fall through to the column check before treating
			// the FindPath failure as fatal (issue #5: "from edge not found").
			if col, colErr := m.Ti.GetColumn(k); colErr == nil {
				switch {
				case isJSONColType(col.Type):
					continue
				case col.Array && md.Data.Type == graph.NodeList:
					continue
				default:
					return nil, fmt.Errorf("column '%s' of type '%s' does not accept an object or array value", k, col.Type)
				}
			}

			var ty qcode.MType
			var ok bool

			switch ms.mt {
			case qcode.MTInsert:
				ty, ok = insertTypes[k]
			case qcode.MTUpdate:
				ty, ok = updateTypes[k]
			}

			if ok && ty != qcode.MTKeyword {
				mi := mutItem{Mutate: qcode.Mutate{
					ID:       ms.id,
					ParentID: m.ParentID,
					Type:     ty,
					Key:      k,
					//	Val:      v,
					Path: append(m.Path, k),
					Ti:   m.Ti,
				}, md: md}
				ml = []mutItem{mi}
				ms.id++
				items = append(items, ml...)
			} else if !ok {
				return nil, err
			}
			continue
		}

		rel := sdata.PathToRel(paths[0])
		ty := ms.mt

		// A nested relation whose payload is a connect is not written, it is
		// looked up: the row that matches the filter is the parent, and the
		// child's foreign key is bound to it. If nothing matches, the
		// child's foreign key is null and the insert writes nothing.
		if filter, ok := connectFilter(v); ok && ms.mt == qcode.MTInsert {
			mi, err := co.newConnectMutate(ms, m, k, rel, filter)
			if err != nil {
				return nil, err
			}
			items = append(items, mi)
			continue
		}

		// The key names a relation, not a column of this table.
		if m.nested == nil {
			m.nested = map[string]struct{}{}
		}
		m.nested[k] = struct{}{}

		if md.Data.Type == graph.NodeList && ms.mt == qcode.MTInsert {
			// FindPath resolves child→parent, which is always RelOneToOne for
			// an FK child. A list value *is* the one-to-many signal: resolve
			// the reverse (parent→child) edge so the child's FK is bound to
			// the parent's id, matching what addTablesAndColumns and the CTE
			// renderer expect of RelOneToMany insert children.
			if rel.Type == sdata.RelOneToOne {
				rel = reverseRel(rel)
			}
		}
		if md.Data.Type == graph.NodeList && rel.Type != sdata.RelOneToMany {
			return nil, fmt.Errorf("expecting object for '%s'", k)
		}

		// Nested one-to-many children under an UPDATE attach existing rows
		// by setting the FK.
		if rel.Type == sdata.RelOneToMany && ms.mt == qcode.MTUpdate {
			ty = qcode.MTConnect
		}

		// The table being mutated is the child side of the relationship:
		// Left for child→parent (RelOneToOne) edges and for legacy
		// parent-side list items whose Right side is the parent mutate's
		// own table; Right for flipped parent→child (RelOneToMany) edges.
		ti := rel.Left.Ti
		if rel.Type == sdata.RelOneToMany &&
			!(rel.Right.Ti.Schema == m.Ti.Schema && rel.Right.Ti.Name == m.Ti.Name) {
			ti = rel.Right.Ti
		}

		mi := mutItem{Mutate: qcode.Mutate{
			ID:       ms.id,
			ParentID: m.ID,
			Type:     ty,
			Key:      k,
			Path:     path,
			Rel:      rel,
			Ti:       ti,
		}, md: md}
		ms.id++

		if md.Data.Type == graph.NodeList {
			ml = co.processList(mi)
			// Every element of an inline list is its own mutation with an id
			// of its own, so the next sibling starts after all of them.
			// Sharing an id would give two mutations the same name.
			if len(ml) > 1 {
				ms.id += int32(len(ml) - 1)
			}
		} else {
			// A single nested object must produce its mutate too; leaving
			// ml nil here silently dropped the child row entirely.
			ml = []mutItem{mi}
		}
		items = append(items, ml...)
	}

	if filterNode, ok := data.CMap["where"]; ok {
		st := util.NewStackInf()
		node := &graph.Node{
			Type:     filterNode.Type,
			Children: filterNode.Children,
			CMap:     filterNode.CMap,
		}

		if m.Where.Exp, _, err = co.compileBaseExpNode(
			"",
			m.Ti,
			st,
			node,
			m.IsJSON); err != nil {
			return nil, err
		}
	}

	if m.Rel.Type == sdata.RelRecursive {
		var find string

		if v1, ok := data.CMap["find"]; !ok {
			if ms.mt == qcode.MTInsert {
				find = "child"
			} else {
				find = "parent"
			}
		} else {
			find = string(v1.Val)
		}

		switch find {
		case "child", "children":
			m.Rel.Type = sdata.RelOneToOne

		case "parent", "parents":
			if ms.mt == qcode.MTInsert {
				return nil, fmt.Errorf("a new '%s' cannot have a parent", m.Key)
			}
			m.Rel.Type = sdata.RelOneToMany
			m.Rel = flipRel(m.Rel)
		}
	}

	return items, nil
}

func (co *Compiler) processList(m mutItem) []mutItem {
	// For MongoDB: always expand arrays into multiple mutations
	// MongoDB processes each element separately in its driver
	if co.s.DBType() == "mongodb" {
		// For single objects, return single mutation
		if m.md.Data.Type != graph.NodeList {
			return []mutItem{m}
		}
		// For arrays, create separate mutations for each element
		var mList []mutItem
		for i := range m.md.Data.Children {
			m1 := m
			m1.md.Data = m.md.Data.Children[i]
			m1.Array = m1.md.Data.Type == graph.NodeList
			m1.md.Array = m1.Array
			m1.ID += int32(i)
			mList = append(mList, m1)
		}
		return mList
	}

	// For SQL databases: use Array flag to control json_to_recordset vs json_to_record
	// The SQL is generated once and processes all elements from the JSON parameter
	if m.md.IsJSON {
		m.Array = m.md.Data.Type == graph.NodeList
		m.md.Array = m.Array
		if m.Array && len(m.md.Data.Children) == 0 {
			// An empty list inserts nothing. The recordset's column list
			// comes from the first element of the array and there is none,
			// so the table's own columns are used: the recordset is empty
			// either way, so the insert writes no rows.
			m.md.Data = co.emptyPayloadNode(m.Ti)
			return []mutItem{m}
		}
		m.md.Data = unionOfElements(m.md.Data)
		return []mutItem{m}
	}

	// For non-IsJSON (inline data), create separate mutations for each element
	if len(m.md.Data.Children) == 0 {
		// An inline list has one insert per element, so an empty list has
		// no insert at all and nothing to return.
		return nil
	}
	var mList []mutItem
	for i := range m.md.Data.Children {
		m1 := m
		m1.md.Data = m.md.Data.Children[i]
		m1.Array = m1.md.Data.Type == graph.NodeList
		m1.md.Array = m1.Array
		m1.ID += int32(i)
		mList = append(mList, m1)
	}
	return mList
}

// unionOfElements is the payload of a list of rows as one row: every key that
// any element has, so the recordset reads a value an element leaves out as
// null and never drops one that only a later element has.
func unionOfElements(list *graph.Node) *graph.Node {
	n := &graph.Node{Type: graph.NodeObj, CMap: map[string]*graph.Node{}}
	for _, el := range list.Children {
		for _, c := range el.Children {
			if _, ok := n.CMap[c.Name]; ok {
				continue
			}
			n.CMap[c.Name] = c
			n.Children = append(n.Children, c)
		}
	}
	return n
}

// emptyPayloadNode builds the payload of a bulk insert that has no rows.
//
// The recordset's column list is taken from the first element of the array,
// and an empty array has none, so the table's own columns are used instead.
// The recordset is empty either way, so the insert writes no rows and the
// operation returns an empty list.
func (co *Compiler) emptyPayloadNode(ti sdata.DBTable) *graph.Node {
	n := &graph.Node{Type: graph.NodeObj, CMap: map[string]*graph.Node{}}
	for _, col := range ti.Columns {
		if col.Blocked {
			continue
		}
		n.CMap[col.Name] = &graph.Node{Type: graph.NodeLabel, Val: "null"}
	}
	return n
}

func (co *Compiler) addTablesAndColumns(m *mutItem, items []mutItem, data *graph.Node) error {
	var err error
	cm := make(map[string]struct{})

	if m.DependsOn == nil {
		m.DependsOn = make(map[int32]struct{})
	}

	switch m.Type {
	case qcode.MTInsert:
		// Render columns and values needed to connect current table and the parent table
		// TODO: check if needed
		if m.Rel.Type == sdata.RelOneToOne {
			m.DependsOn[m.ParentID] = struct{}{}
			m.RCols = append(m.RCols, qcode.MRColumn{
				Col:  m.Rel.Left.Col,
				VCol: m.Rel.Right.Col,
			})
			cm[m.Rel.Left.Col.Name] = struct{}{}
		}

		// A flipped one-to-many list child (nested list under an insert):
		// its INSERT references the parent CTE and must bind its own FK
		// column to the parent's id, exactly like a one-to-one child.
		if m.Rel.Type == sdata.RelOneToMany && m.ParentID != -1 &&
			!(m.Rel.Left.Ti.Schema == m.Ti.Schema && m.Rel.Left.Ti.Name == m.Ti.Name) {
			m.DependsOn[m.ParentID] = struct{}{}
			m.RCols = append(m.RCols, qcode.MRColumn{
				Col:  m.Rel.Right.Col,
				VCol: m.Rel.Left.Col,
			})
			cm[m.Rel.Right.Col.Name] = struct{}{}
		}

		// A connect under an insert binds the insert's foreign key to the
		// row the connect found.
		for _, v := range items {
			if v.Type != qcode.MTConnect {
				continue
			}
			m.DependsOn[v.ID] = struct{}{}
			m.RCols = append(m.RCols, qcode.MRColumn{
				Col:  v.Rel.Right.Col,
				VCol: v.Rel.Left.Col,
			})
			cm[v.Rel.Right.Col.Name] = struct{}{}
		}

		// Render columns and values needed by the children of the current level
		// Render child foreign key columns if child-to-parent
		// relationship is one-to-many
		for _, v := range items {
			if v.Type == qcode.MTConnect {
				// A connect binds the foreign key above.
				continue
			}
			if v.Rel.Type != sdata.RelOneToMany {
				continue
			}

			// Legacy shape only: a parent-side list item nested under this
			// child insert (v.Rel.Right is this table). Its CTE must render
			// before this mutate and its RCols bind this table's FK to the
			// item's id. Flipped list children (v.Rel.Left is this table)
			// instead depend on this mutate and bind their own FK in their
			// own branch above — registering them here would create a
			// dependency cycle.
			if v.Rel.Right.Ti.Schema == m.Ti.Schema && v.Rel.Right.Ti.Name == m.Ti.Name {
				m.DependsOn[v.ID] = struct{}{}
				m.RCols = append(m.RCols, qcode.MRColumn{
					Col:  v.Rel.Right.Col,
					VCol: v.Rel.Left.Col,
				})
				cm[v.Rel.Right.Col.Name] = struct{}{}
			}
		}

	case qcode.MTUpdate:
		// For updates, the parent MUST execute before the child
		// if they are linked, so the child can use the parent's ID in its WHERE clause.
		if m.ParentID != -1 {
			m.DependsOn[m.ParentID] = struct{}{}
		}

		if m.Rel.Type == sdata.RelOneToMany {
			m.RCols = append(m.RCols, qcode.MRColumn{
				Col:  m.Rel.Left.Col,
				VCol: m.Rel.Right.Col,
			})
			cm[m.Rel.Left.Col.Name] = struct{}{}
		}

	default:
		if m.Rel.Type == sdata.RelOneToOne {
			m.DependsOn[m.ParentID] = struct{}{}
		}

		for i, v := range items {
			if v.Rel.Type == sdata.RelOneToOne {
				if v.DependsOn == nil {
					items[i].DependsOn = make(map[int32]struct{})
				}
				items[i].DependsOn[m.ParentID] = struct{}{}
			}
		}
	}

	if m.Cols, err = co.getColumnsFromData(m, data, cm); err != nil {
		return err
	}

	return nil
}

func (co *Compiler) getColumnsFromData(m *mutItem, data *graph.Node, cm map[string]struct{}) ([]qcode.MColumn, error) {
	var cols []qcode.MColumn

	/*
		for i, col := range m.Ti.Columns {
			k := col.Name

			if _, ok := cm[k]; ok {
				continue
			}

			if _, ok := data.CMap[k]; !ok {
				continue
			}

			if col.Blocked {
				return nil, fmt.Errorf("column blocked: %s", k)
			}

			cols = append(cols, qcode.MColumn{Col: m.Ti.Columns[i], FieldName: k})
		}
	*/

	// TODO: This is faster than the above
	// but randomized maps in go make testing harder
	// put this back in once we have integration testing

	for k := range data.CMap {
		k1 := k
		k := co.ParseName(k)

		if _, ok := cm[k]; ok {
			continue
		}

		if _, ok := m.nested[k]; ok {
			// A relation the payload nests, not a column.
			continue
		}

		col, ok := m.Ti.ColumnExists(k)
		if !ok {
			// Object- and list-valued keys are relationship targets or nested
			// shapes: processNestedMutations resolves those and errors on the
			// truly unknown ones. Keyword keys (find/where/connect/disconnect)
			// are consumed by the mutation compiler itself. Everything else is
			// a scalar the caller believes is a column, and silently dropping
			// it loses data — a benchmark insert lost its timestamp this way
			// and only failed two steps later as a NOT NULL constraint — so it
			// now errors exactly like unknown object keys always have.
			node := data.CMap[k1]
			if node != nil && (node.Type == graph.NodeObj || node.Type == graph.NodeList) {
				continue
			}
			if _, keyword := insertTypes[k]; keyword {
				continue
			}
			if _, keyword := updateTypes[k]; keyword {
				continue
			}
			_, err := m.Ti.GetColumn(k)
			return nil, err
		}

		cols = append(cols, qcode.MColumn{Col: col, FieldName: k1, Alias: k})
	}

	return cols, nil
}

func flipRel(rel sdata.DBRel) sdata.DBRel {
	rc := rel.Right.Col
	rel.Right.Col = rel.Left.Col
	rel.Left.Col = rc
	return rel
}

// reverseRel reverses the direction of a relationship edge, swapping both
// sides and the cardinality. FindPath(from=child, to=parent) returns the
// child→parent edge (RelOneToOne); list-valued nested children need the
// parent→child edge (RelOneToMany) so downstream rendering binds the child's
// FK column to the parent's id.
func reverseRel(rel sdata.DBRel) sdata.DBRel {
	lt, lc := rel.Left.Ti, rel.Left.Col
	rel.Left.Ti, rel.Left.Col = rel.Right.Ti, rel.Right.Col
	rel.Right.Ti, rel.Right.Col = lt, lc
	switch rel.Type {
	case sdata.RelOneToOne:
		rel.Type = sdata.RelOneToMany
	case sdata.RelOneToMany:
		rel.Type = sdata.RelOneToOne
	}
	return rel
}

// isJSONColType reports whether a DB column type holds JSON documents and
// therefore accepts object/array values directly.
func isJSONColType(t string) bool {
	return t == "json" || t == "jsonb"
}

// colValsFromNode projects the parsed payload node into the neutral
// per-column value map that backends consume. Object/array payloads are
// only turned into JSON literals when the target column is json/jsonb —
// the renderer then quotes them and casts to the column type. SQL array
// columns keep the List/ListItems shape.
func (co *Compiler) colValsFromNode(t sdata.DBTable, n *graph.Node) map[string]qcode.ColVal {
	if n == nil {
		return nil
	}
	out := make(map[string]qcode.ColVal, len(n.CMap))
	for k, f := range n.CMap {
		cv := qcode.ColVal{Val: f.Val}
		switch f.Type {
		case graph.NodeVar:
			cv.Var = true
		case graph.NodeLabel:
			// A bare null in a payload is SQL NULL, not the four character
			// string "null". The renderer quoted it, so `update: {cover:
			// null}` stored the string and Postgres refused the uuid cast.
			if f.Val == "null" {
				cv.Null = true
			}
		case graph.NodeList:
			cv.List = true
			items := make([]string, 0, len(f.Children))
			for _, c := range f.Children {
				if c.Type == graph.NodeNum {
					items = append(items, c.Val)
				} else {
					items = append(items, `'`+c.Val+`'`)
				}
			}
			cv.ListItems = items
			if co.isJSONCol(t, k) {
				cv.Val = nodeToJSON(f)
				cv.List = false
				cv.ListItems = nil
			}
		case graph.NodeObj:
			if co.isJSONCol(t, k) {
				cv.Val = nodeToJSON(f)
			}
		}
		out[k] = cv
	}
	return out
}

// isJSONCol reports whether key k names a json/jsonb column on table t.
func (co *Compiler) isJSONCol(t sdata.DBTable, k string) bool {
	col, ok := t.ColumnExists(co.ParseName(k))
	return ok && isJSONColType(col.Type)
}

// nodeToJSON serializes a parsed value node back into compact JSON text so
// object/array payloads can be rendered as literals for json/jsonb columns.
func nodeToJSON(n *graph.Node) string {
	var b bytes.Buffer
	if err := writeNodeJSON(&b, n); err != nil {
		return ""
	}
	return b.String()
}

func writeNodeJSON(b *bytes.Buffer, n *graph.Node) error {
	switch n.Type {
	case graph.NodeObj:
		b.WriteByte('{')
		for i, c := range n.Children {
			if i != 0 {
				b.WriteByte(',')
			}
			key, err := json.Marshal(c.Name)
			if err != nil {
				return err
			}
			b.Write(key)
			b.WriteByte(':')
			if err := writeNodeJSON(b, c); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case graph.NodeList:
		b.WriteByte('[')
		for i, c := range n.Children {
			if i != 0 {
				b.WriteByte(',')
			}
			if err := writeNodeJSON(b, c); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case graph.NodeNum, graph.NodeBool:
		b.WriteString(n.Val)
	case graph.NodeVar:
		return fmt.Errorf("variables are not supported inside inline object or array values")
	default:
		// NodeStr, NodeLabel: escape as a JSON string
		v, err := json.Marshal(n.Val)
		if err != nil {
			return err
		}
		b.Write(v)
	}
	return nil
}
