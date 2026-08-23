package qcode

import (
	"encoding/json"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

type MType uint8

const (
	MTInsert MType = iota + 1
	MTUpdate
	MTUpsert
	MTDelete
	MTConnect
	MTDisconnect
	MTNone
	MTKeyword
)

// ConflictAction controls the behavior of an insert when its inferred unique
// target conflicts with an existing row.
type ConflictAction uint8

const (
	ConflictNone ConflictAction = iota
	ConflictGet
)

// Mutate is one mutation operation in the IR tree.
type Mutate struct {
	Field

	ID        int32
	ParentID  int32
	SelID     int32
	DependsOn map[int32]struct{}
	Type      MType
	// CType     uint8
	Key            string
	Path           []string
	Val            json.RawMessage
	Cols           []MColumn
	RCols          []MRColumn
	Ti             sdata.DBTable
	Rel            sdata.DBRel
	Where          Filter
	ConflictAction ConflictAction
	ConflictCols   []MColumn
	Multi          bool

	// IsJSON and Array describe how Val was parsed; backends consult
	// them when rendering mutation payloads.
	IsJSON bool
	Array  bool

	// ColVals maps column names to their payload values in neutral
	// form. Populated by frontends; backends bind arguments from it.
	ColVals map[string]ColVal
}

// ColVal is one mutation payload value: either a literal or a $var
// reference, optionally a list of pre-rendered SQL literal items.
type ColVal struct {
	Val       string
	Var       bool
	List      bool
	ListItems []string
}

type MColumn struct {
	Col       sdata.DBColumn
	FieldName string
	Alias     string
	Value     string
	Set       bool
}

type MRColumn struct {
	Col  sdata.DBColumn
	VCol sdata.DBColumn
}

type MTable struct {
	Ti sdata.DBTable
	// CType uint8
}
