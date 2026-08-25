package graphql

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"

	"encoding/json"
	"errors"
	"fmt"
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
	mids := map[string][]int32{}
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
		nextID++

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
			for _, v := range co.processList(mi) {
				st.Push(v)
			}
		} else {
			st.Push(mi)
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
	ms := mState{st: st, qc: qc, mt: mt, id: msID}

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

	for i := range mutates {
		m1 := &mutates[i]

		// Re-id all items to make array access easy.
		m1.ID = mmap[m1.ID]
		m1.ParentID = mmap[m1.ParentID]

		if m1.Type != qcode.MTNone {
			mids[m1.Ti.Name] = append(mids[m1.Ti.Name], m1.ID)
		}

		for id := range m1.DependsOn {
			delete(m1.DependsOn, id)
			m1.DependsOn[mmap[id]] = struct{}{}
		}

		for i, id := range m1.children {
			m1.children[i] = mmap[id]
		}
	}
	qc.MUnions = mids

	// Pull up children of MTNone to the depends-on of it's parent if applicable.
	for i := range mutates {
		m1 := &mutates[i]

		if len(mids[m1.Ti.Name]) > 1 {
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
		mi.ColVals = colValsFromNode(mi.md.Data)
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

	for _, col := range m.Ti.Columns {
		if col.PrimaryKey || !col.UniqueKey {
			continue
		}
		if mc, ok := byName[col.Name]; ok {
			candidates = append(candidates, []qcode.MColumn{mc})
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("on_conflict: get on table %q requires a supplied primary or unique key", m.Ti.Name)
	}
	if len(candidates) > 1 {
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

	m.ConflictAction = qcode.ConflictGet
	m.ConflictCols = candidates[0]
	return nil
}

type mData struct {
	Data   *graph.Node
	IsJSON bool
	Array  bool
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
			return md, fmt.Errorf("variable not found: %s", av.Val)
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
			return md, fmt.Errorf("variable not found: %s", av.Val)
		}
		md.Data, err = graph.ParseArgValue(string(val), true)
		if err != nil {
			return md, err
		}
		// For multi-root mutations, each root must use its own inline data
		// rather than the shared _sg_input CTE which only binds one ActionVar.
		if !multiRoot {
			md.IsJSON = true
		}

	default:
		md.Data = av
	}
	return md, nil
}

// TODO: Handle cases where a column name matches the child table name
// the child path needs to be exluded in the json sent to insert or update

func (co *Compiler) newMutate(ms *mState, m mutItem) error {
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
			if v.Rel.Type == sdata.RelOneToOne {
				ms.st.Push(v)
			}
		}
		ms.st.Push(m)
		for _, v := range items {
			if v.Rel.Type == sdata.RelOneToMany {
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

		if md, err = parseDataValue(ms.qc, v, m.IsJSON); err != nil {
			return nil, err
		}

		if md.Data.Type != graph.NodeObj && md.Data.Type != graph.NodeList {
			continue
		}

		k := co.ParseName(v.Name)

		// Get child-to-parent relationship
		paths, err := co.FindPath(k, m.Key, "")
		// no relationship found must be a keyword
		if err != nil {
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

		if md.Data.Type == graph.NodeList && rel.Type != sdata.RelOneToMany {
			return nil, fmt.Errorf("expecting object for '%s'", k)
		}

		// Nested one-to-many children under an INSERT create new rows;
		// under an UPDATE they attach existing rows by setting the FK.
		if rel.Type == sdata.RelOneToMany && ms.mt == qcode.MTUpdate {
			ty = qcode.MTConnect
		}

		mi := mutItem{Mutate: qcode.Mutate{
			ID:       ms.id,
			ParentID: m.ID,
			Type:     ty,
			Key:      k,
			Path:     append(m.Path, k),
			Rel:      rel,
			Ti:       rel.Left.Ti,
		}, md: md}
		ms.id++

		if md.Data.Type == graph.NodeList {
			ml = co.processList(mi)
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
			m1.ID += int32(i)
			mList = append(mList, m1)
		}
		return mList
	}

	// For SQL databases: use Array flag to control json_to_recordset vs json_to_record
	// The SQL is generated once and processes all elements from the JSON parameter
	if m.IsJSON {
		m.Array = m.md.Data.Type == graph.NodeList
		m.md.Data = m.md.Data.Children[0]
		return []mutItem{m}
	}

	// For non-IsJSON (inline data), create separate mutations for each element
	var mList []mutItem
	for i := range m.md.Data.Children {
		m1 := m
		m1.md.Data = m.md.Data.Children[i]
		m1.Array = m1.md.Data.Type == graph.NodeList
		m1.ID += int32(i)
		mList = append(mList, m1)
	}
	return mList
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

		// Render columns and values needed by the children of the current level
		// Render child foreign key columns if child-to-parent
		// relationship is one-to-many
		for _, v := range items {
			if v.Rel.Type == sdata.RelOneToMany {
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

// colValsFromNode projects the parsed payload node into the neutral
// per-column value map that backends consume.
func colValsFromNode(n *graph.Node) map[string]qcode.ColVal {
	if n == nil {
		return nil
	}
	out := make(map[string]qcode.ColVal, len(n.CMap))
	for k, f := range n.CMap {
		cv := qcode.ColVal{Val: f.Val}
		switch f.Type {
		case graph.NodeVar:
			cv.Var = true
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
		}
		out[k] = cv
	}
	return out
}
