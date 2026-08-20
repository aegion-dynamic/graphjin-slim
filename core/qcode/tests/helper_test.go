package qcode_test

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
)

type (
	Compiler = qcode.Compiler
	Config   = qcode.Config
	QCode    = qcode.QCode
	Select   = qcode.Select
	Field    = qcode.Field
	Column   = qcode.Column
	Exp      = qcode.Exp
	OrderBy  = qcode.OrderBy
	Paging   = qcode.Paging
	Filter   = qcode.Filter
	SkipType = qcode.SkipType
	QType    = qcode.QType
	ValidErr = qcode.ValidErr
)

const (
	SkipTypeNone         = qcode.SkipTypeNone
	SkipTypeDrop         = qcode.SkipTypeDrop
	SkipTypeRemote       = qcode.SkipTypeRemote
	SkipTypeDatabaseJoin = qcode.SkipTypeDatabaseJoin
	QTQuery              = qcode.QTQuery
	QTMutation           = qcode.QTMutation
	QTSubscription       = qcode.QTSubscription
)

var (
	NewCompiler        = qcode.NewCompiler
	ParseSchema        = qcode.ParseSchema
	ParseFrameClause   = qcode.ParseFrameClause
	PascalToSnakeSpace = qcode.PascalToSnakeSpace
)
