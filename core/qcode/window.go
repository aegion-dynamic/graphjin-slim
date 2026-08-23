package qcode

// WindowSpec captures the backend window definition produced by public
// analytics directives such as @running, @previous, and @rank. When a Field has
// a non-nil Window, the SQL emitter wraps the function as
//
//	<func>(args) OVER (PARTITION BY <p1>, ... ORDER BY <o1>, ... <frame>)
//
// and the field does NOT trigger a GROUP BY on the enclosing select: analytics
// directives return one row per input row, not one per group.
//
// All three sections are optional individually. An empty WindowSpec
// emits a bare `OVER ()`, which is valid SQL for internal window functions that
// support the implicit window frame.
type WindowSpec struct {
	Partition []string
	OrderBy   []WindowOrder
	Frame     string // canonical, uppercase frame clause; "" = engine default
}

// NullsHandling controls placement of NULL values in the backend OVER ORDER BY.
// The public analytics directives do not expose null placement yet.
type NullsHandling int8

const (
	NullsDefault NullsHandling = iota
	NullsFirst
	NullsLast
)

// WindowOrder is one column entry in the OVER (... ORDER BY ...) list.
type WindowOrder struct {
	Col   string
	Desc  bool
	Nulls NullsHandling
}

// WindowFunc is an internal SQL analytic/window function selected by public
// GraphJin analytics directives. These are rendered by the SQL compiler rather
// than treated as user-defined database functions.
type WindowFunc int8

const (
	WindowFuncNone WindowFunc = iota
	WindowFuncRowNumber
	WindowFuncRank
	WindowFuncDenseRank
	WindowFuncLag
	WindowFuncLead
	WindowFuncFirstValue
	WindowFuncLastValue
)

func (wf WindowFunc) String() string {
	switch wf {
	case WindowFuncRowNumber:
		return "row_number"
	case WindowFuncRank:
		return "rank"
	case WindowFuncDenseRank:
		return "dense_rank"
	case WindowFuncLag:
		return "lag"
	case WindowFuncLead:
		return "lead"
	case WindowFuncFirstValue:
		return "first_value"
	case WindowFuncLastValue:
		return "last_value"
	default:
		return ""
	}
}

func (wf WindowFunc) IsValueFunc() bool {
	switch wf {
	case WindowFuncLag, WindowFuncLead, WindowFuncFirstValue, WindowFuncLastValue:
		return true
	default:
		return false
	}
}

func WindowSpecHasNulls(w *WindowSpec) bool {
	if w == nil {
		return false
	}
	for _, ord := range w.OrderBy {
		if ord.Nulls != NullsDefault {
			return true
		}
	}
	return false
}
