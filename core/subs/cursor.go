package subs

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/qcode"
)

const systemCursorVersion = "m1:"

type CursorCheckpoint struct {
	SelectionID int32
	Values      map[string]any
	Ok          bool
}

func EncodeSystemCursor(selectionID int32, values []any) string {
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(int64(selectionID), 10) + "," + systemCursorVersion +
		base64.RawURLEncoding.EncodeToString(data)
}

func DecodeSystemCursor(raw string) (int32, []any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil, false
	}
	if strings.HasPrefix(raw, "gj-") {
		if i := strings.IndexByte(raw, ':'); i != -1 {
			raw = raw[i+1:]
		}
	}
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) != 2 {
		return 0, nil, false
	}
	id, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return 0, nil, false
	}
	if strings.HasPrefix(parts[1], systemCursorVersion) {
		data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(parts[1], systemCursorVersion))
		if err != nil {
			return 0, nil, false
		}
		var values []any
		if err := json.Unmarshal(data, &values); err != nil {
			return 0, nil, false
		}
		return int32(id), values, true
	}

	legacy := strings.Split(parts[1], ",")
	values := make([]any, len(legacy))
	for i := range legacy {
		values[i] = legacy[i]
	}
	return int32(id), values, true
}

func CheckpointForSelect(sel *qcode.Select, vars map[string]json.RawMessage) CursorCheckpoint {
	if sel == nil || !sel.Paging.Cursor || sel.Paging.CursorVar == "" {
		return CursorCheckpoint{}
	}
	var raw string
	if err := json.Unmarshal(vars[sel.Paging.CursorVar], &raw); err != nil || raw == "" {
		return CursorCheckpoint{}
	}
	selectionID, values, ok := DecodeSystemCursor(raw)
	if !ok || selectionID != sel.ID || len(values) < len(sel.OrderBy) {
		return CursorCheckpoint{}
	}
	cp := CursorCheckpoint{
		SelectionID: selectionID,
		Values:      make(map[string]any, len(sel.OrderBy)),
		Ok:          true,
	}
	for i, order := range sel.OrderBy {
		cp.Values[SystemCursorOrderKey(order)] = values[i]
	}
	return cp
}

func SystemCursorOrderKey(order qcode.OrderBy) string {
	if order.Key != "" {
		return order.Col.Name + "_" + order.Key
	}
	return order.Col.Name
}

func SystemCursorValues(sel *qcode.Select, row map[string]any) []any {
	if sel == nil || len(sel.OrderBy) == 0 || row == nil {
		return nil
	}
	values := make([]any, 0, len(sel.OrderBy))
	for _, order := range sel.OrderBy {
		values = append(values, SystemOrderValue(row, order))
	}
	return values
}

func SystemOrderValue(row map[string]any, order qcode.OrderBy) any {
	value := row[order.Col.Name]
	if order.Key == "" {
		return value
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed[order.Key]
	case json.RawMessage:
		var object map[string]any
		if json.Unmarshal(typed, &object) == nil {
			return object[order.Key]
		}
	case []byte:
		var object map[string]any
		if json.Unmarshal(typed, &object) == nil {
			return object[order.Key]
		}
	case string:
		var object map[string]any
		if json.Unmarshal([]byte(typed), &object) == nil {
			return object[order.Key]
		}
	}
	return nil
}

func FindSeekExp(ex *qcode.Exp) *qcode.Exp {
	if ex == nil {
		return nil
	}
	if IsSeekExp(ex) {
		return ex
	}
	for _, child := range ex.Children {
		if found := FindSeekExp(child); found != nil {
			return found
		}
	}
	return nil
}

func IsSeekExp(ex *qcode.Exp) bool {
	return ex != nil &&
		ex.Op == qcode.OpOr &&
		len(ex.Children) != 0 &&
		ex.Children[0] != nil &&
		ex.Children[0].Op == qcode.OpIsNull &&
		ex.Children[0].Left.Table == "__cur"
}

func SystemRowLess(sel *qcode.Select, left, right map[string]any) bool {
	if sel == nil {
		return false
	}
	for _, order := range sel.OrderBy {
		cmp := CompareValues(SystemOrderValue(left, order), SystemOrderValue(right, order))
		if cmp == 0 {
			continue
		}
		switch order.Order {
		case qcode.OrderDesc, qcode.OrderDescNullsFirst, qcode.OrderDescNullsLast:
			return cmp > 0
		default:
			return cmp < 0
		}
	}
	return false
}

func PagingLimit(sel *qcode.Select, vars map[string]json.RawMessage) (int, error) {
	if sel == nil {
		return 0, nil
	}
	if sel.Paging.LimitVar != "" {
		return pagingIntVar("limit", sel.Paging.LimitVar, vars, false)
	}
	return int(sel.Paging.Limit), nil
}

func PagingOffset(sel *qcode.Select, vars map[string]json.RawMessage) (int, error) {
	if sel == nil {
		return 0, nil
	}
	if sel.Paging.OffsetVar != "" {
		return pagingIntVar("offset", sel.Paging.OffsetVar, vars, true)
	}
	return int(sel.Paging.Offset), nil
}

func pagingIntVar(kind, name string, vars map[string]json.RawMessage, allowZero bool) (int, error) {
	raw := vars[name]
	if len(raw) == 0 {
		return 0, fmt.Errorf("%s variable %q is required", kind, name)
	}
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return 0, fmt.Errorf("%s variable %q must be an integer", kind, name)
	}
	value, err := strconv.ParseInt(number.String(), 10, 32)
	if err != nil || value < 0 || (!allowZero && value == 0) {
		qualifier := "a positive integer"
		if allowZero {
			qualifier = "a non-negative integer"
		}
		return 0, fmt.Errorf("%s variable %q must be %s", kind, name, qualifier)
	}
	return int(value), nil
}

func CompareValues(a, b any) int {
	af, aok := numberValue(a)
	bf, bok := numberValue(b)
	if aok && bok {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	}
	as, bs := fmt.Sprint(a), fmt.Sprint(b)
	return strings.Compare(as, bs)
}

func numberValue(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		f, err := strconv.ParseFloat(fmt.Sprint(v), 64)
		return f, err == nil
	}
}
