package dbjoin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// DBResult holds the result from executing a query against one database.
type DBResult struct {
	Database       string
	Data           json.RawMessage
	FragmentHits   int64
	FragmentMisses int64
	Err            error
}

// DatabaseJoinFieldIDs finds fields that require cross-database joins.
func DatabaseJoinFieldIDs(selects []qcode.Select) ([][]byte, map[string]*qcode.Select, error) {
	if len(selects) == 0 {
		return nil, nil, nil
	}

	fm := make([][]byte, 0)
	sm := make(map[string]*qcode.Select)

	for i, sel := range selects {
		if sel.SkipRender != qcode.SkipTypeDatabaseJoin {
			continue
		}

		placeholderKey := fmt.Sprintf("__%s_db_join", sel.FieldName)
		fm = append(fm, []byte(placeholderKey))
		sm[placeholderKey] = &selects[i]
	}

	return fm, sm, nil
}

// CountDatabaseJoins returns the number of cross-database joins in a QCode.
func CountDatabaseJoins(qc *qcode.QCode) int32 {
	if qc == nil {
		return 0
	}
	var count int32
	for _, sel := range qc.Selects {
		if sel.SkipRender == qcode.SkipTypeDatabaseJoin {
			count++
		}
	}
	return count
}

// MergeRootResults merges results from multiple databases into a single JSON response.
func MergeRootResults(results []DBResult) ([]byte, error) {
	for _, r := range results {
		if r.Err != nil {
			return nil, fmt.Errorf("database %s: %w", r.Database, r.Err)
		}
	}

	if len(results) == 0 {
		return nil, nil
	}

	if len(results) == 1 {
		return results[0].Data, nil
	}

	merged := make(map[string]json.RawMessage)

	for _, r := range results {
		if len(r.Data) == 0 {
			continue
		}

		var obj map[string]json.RawMessage
		if err := json.Unmarshal(r.Data, &obj); err != nil {
			return nil, fmt.Errorf("failed to parse result from %s: %w", r.Database, err)
		}

		for k, v := range obj {
			if _, exists := merged[k]; exists {
				return nil, fmt.Errorf("duplicate key '%s' in multi-database result", k)
			}
			merged[k] = v
		}
	}

	data, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged result: %w", err)
	}

	return data, nil
}

// BuildChildGraphQLQuery constructs a GraphQL query for a cross-database child table.
func BuildChildGraphQLQuery(sel *qcode.Select, selects []qcode.Select, fkCol sdata.DBColumn, parentID []byte) []byte {
	var buf bytes.Buffer

	buf.WriteString("query { ")
	buf.WriteString(sel.Table)

	// Add WHERE filter on the FK column matching the parent ID
	buf.WriteString("(where: {")
	buf.WriteString(fkCol.Name)
	buf.WriteString(": {eq: ")
	WriteGraphQLLiteral(&buf, fkCol, parentID)
	buf.WriteString("}})")

	// Write the requested fields
	buf.WriteString(" { ")
	WriteSelectFields(&buf, sel, selects)
	buf.WriteString(" }")

	buf.WriteString(" }")
	return buf.Bytes()
}

// BuildDatabaseQuery creates a new GraphQL query containing only the specified root fields.
func BuildDatabaseQuery(rawQuery []byte, rootFields []string) ([]byte, error) {
	op, err := graph.Parse(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	allowed := make(map[string]bool, len(rootFields))
	for _, f := range rootFields {
		allowed[f] = true
	}

	keepFieldIDs := make(map[int32]bool)
	for _, f := range op.Fields {
		if f.ParentID == -1 && allowed[f.Name] {
			keepFieldIDs[f.ID] = true
			markDescendants(op.Fields, f.ID, keepFieldIDs)
		}
	}

	var buf bytes.Buffer

	switch op.Type {
	case graph.OpQuery:
		buf.WriteString("query")
	case graph.OpMutate:
		buf.WriteString("mutation")
	case graph.OpSub:
		buf.WriteString("subscription")
	}

	if op.Name != "" {
		buf.WriteString(" ")
		buf.WriteString(op.Name)
	}

	if len(op.VarDef) > 0 {
		buf.WriteString("(")
		for i, v := range op.VarDef {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString("$")
			buf.WriteString(v.Name)
		}
		buf.WriteString(")")
	}

	buf.WriteString(" { ")
	writeFieldsRecursive(&buf, op.Fields, -1, keepFieldIDs)
	buf.WriteString(" }")

	return buf.Bytes(), nil
}

func markDescendants(fields []graph.Field, parentID int32, keep map[int32]bool) {
	for _, f := range fields {
		if f.ParentID == parentID {
			keep[f.ID] = true
			markDescendants(fields, f.ID, keep)
		}
	}
}

func writeFieldsRecursive(buf *bytes.Buffer, fields []graph.Field, parentID int32, keepFieldIDs map[int32]bool) {
	first := true
	for _, f := range fields {
		if f.ParentID != parentID || !keepFieldIDs[f.ID] {
			continue
		}

		if !first {
			buf.WriteString(" ")
		}
		first = false

		if f.Alias != "" {
			buf.WriteString(f.Alias)
			buf.WriteString(": ")
		}

		buf.WriteString(f.Name)

		if len(f.Args) > 0 {
			buf.WriteString("(")
			for i, arg := range f.Args {
				if i > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(arg.Name)
				buf.WriteString(": ")
				writeNode(buf, arg.Val)
			}
			buf.WriteString(")")
		}

		hasChildren := false
		for _, child := range fields {
			if child.ParentID == f.ID && keepFieldIDs[child.ID] {
				hasChildren = true
				break
			}
		}

		if hasChildren {
			buf.WriteString(" { ")
			writeFieldsRecursive(buf, fields, f.ID, keepFieldIDs)
			buf.WriteString(" }")
		}
	}
}

func writeNode(buf *bytes.Buffer, n *graph.Node) {
	if n == nil {
		buf.WriteString("null")
		return
	}

	switch n.Type {
	case graph.NodeStr:
		buf.WriteString("\"")
		buf.WriteString(strings.ReplaceAll(n.Val, "\"", "\\\""))
		buf.WriteString("\"")
	case graph.NodeNum, graph.NodeBool:
		buf.WriteString(n.Val)
	case graph.NodeVar:
		buf.WriteString("$")
		buf.WriteString(n.Val)
	case graph.NodeLabel:
		buf.WriteString(n.Val)
	case graph.NodeList:
		buf.WriteString("[")
		for i, child := range n.Children {
			if i > 0 {
				buf.WriteString(", ")
			}
			writeNode(buf, child)
		}
		buf.WriteString("]")
	case graph.NodeObj:
		buf.WriteString("{")
		for i, child := range n.Children {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(child.Name)
			buf.WriteString(": ")
			writeNode(buf, child)
		}
		buf.WriteString("}")
	default:
		buf.WriteString(n.Val)
	}
}

// WriteSelectFields writes the field list for a Select, recursing into children.
func WriteSelectFields(buf *bytes.Buffer, sel *qcode.Select, selects []qcode.Select) {
	first := true
	for _, f := range sel.Fields {
		if !first {
			buf.WriteString(" ")
		}
		first = false
		buf.WriteString(f.FieldName)
	}

	for _, cid := range sel.Children {
		csel := &selects[cid]
		if sel.SkipRender != qcode.SkipTypeDatabaseJoin &&
			(csel.SkipRender == qcode.SkipTypeDatabaseJoin || csel.SkipRender == qcode.SkipTypeRemote) {
			continue
		}
		if !first {
			buf.WriteString(" ")
		}
		first = false
		buf.WriteString(csel.FieldName)
		buf.WriteString(" { ")
		WriteSelectFields(buf, csel, selects)
		buf.WriteString(" }")
	}
}

func WriteGraphQLLiteral(buf *bytes.Buffer, col sdata.DBColumn, val []byte) {
	s := strings.TrimSpace(string(val))
	if s == "" {
		buf.WriteString("null")
		return
	}

	if IsGraphQLStringColumnType(col.Type) {
		writeGraphQLStringLiteral(buf, s)
		return
	}

	if isJSONStringLiteral(s) || isBareGraphQLLiteral(s) {
		buf.WriteString(s)
		return
	}

	writeGraphQLStringLiteral(buf, s)
}

func writeGraphQLStringLiteral(buf *bytes.Buffer, s string) {
	if isJSONStringLiteral(s) {
		buf.WriteString(s)
		return
	}

	b, err := json.Marshal(s)
	if err != nil {
		buf.WriteString("null")
		return
	}
	buf.Write(b)
}

func isJSONStringLiteral(s string) bool {
	if len(s) < 2 || s[0] != '"' {
		return false
	}

	var v string
	return json.Unmarshal([]byte(s), &v) == nil
}

func isBareGraphQLLiteral(s string) bool {
	switch s {
	case "null", "true", "false":
		return true
	}

	return isGraphQLNumberLiteral(s)
}

func isGraphQLNumberLiteral(s string) bool {
	if s == "" || strings.HasPrefix(s, "+") {
		return false
	}

	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case i == 0 && r == '-':
		case r == '.' || r == 'e' || r == 'E' || r == '+':
		default:
			return false
		}
	}

	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func IsGraphQLStringColumnType(typ string) bool {
	t := strings.ToLower(strings.TrimSpace(typ))
	if i := strings.IndexByte(t, '('); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}

	switch t {
	case "text", "varchar", "char", "character", "character varying",
		"uuid", "uniqueidentifier", "citext", "bpchar", "clob", "nclob",
		"date", "time", "timetz", "timestamp", "timestamptz", "datetime",
		"json", "jsonb", "xml",
		"string":
		return true
	}

	return strings.HasPrefix(t, "varchar") ||
		strings.HasPrefix(t, "nvarchar") ||
		strings.HasPrefix(t, "char") ||
		strings.HasPrefix(t, "nchar") ||
		strings.HasPrefix(t, "text") ||
		strings.HasPrefix(t, "ntext") ||
		strings.HasSuffix(t, "text") ||
		strings.HasPrefix(t, "time") ||
		strings.HasPrefix(t, "timestamp") ||
		strings.HasPrefix(t, "datetime")
}
