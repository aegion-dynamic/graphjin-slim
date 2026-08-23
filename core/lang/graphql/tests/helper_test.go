package graphql_test

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/lang/graphql"
)

type (
	Compiler = graphql.Compiler
	Config   = graphql.Config
	QCode    = graphql.QCode
	Select   = graphql.Select
	Field    = graphql.Field
	Column   = graphql.Column
	Exp      = graphql.Exp
	OrderBy  = graphql.OrderBy
	Paging   = graphql.Paging
	Filter   = graphql.Filter
	SkipType = graphql.SkipType
	QType    = graphql.QType
	ValidErr = graphql.ValidErr
)

const (
	SkipTypeNone         = graphql.SkipTypeNone
	SkipTypeDrop         = graphql.SkipTypeDrop
	SkipTypeRemote       = graphql.SkipTypeRemote
	SkipTypeDatabaseJoin = graphql.SkipTypeDatabaseJoin
	QTQuery              = graphql.QTQuery
	QTMutation           = graphql.QTMutation
	QTSubscription       = graphql.QTSubscription
)

var (
	NewCompiler        = graphql.NewCompiler
	ParseSchema        = graphql.ParseSchema
	ParseFrameClause   = graphql.ParseFrameClause
	PascalToSnakeSpace = graphql.PascalToSnakeSpace
)
