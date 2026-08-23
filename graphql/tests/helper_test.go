package graphql_test

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"

	"github.com/aegion-dynamic/graphjin-slim/graphql/v3"
)

type (
	Compiler = graphql.Compiler
	Config   = graphql.Config
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
	NewCompiler        = graphql.NewCompiler
	ParseSchema        = graphql.ParseSchema
	ParseFrameClause   = graphql.ParseFrameClause
	PascalToSnakeSpace = graphql.PascalToSnakeSpace
)
