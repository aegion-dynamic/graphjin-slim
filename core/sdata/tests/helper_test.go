package sdata_test

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

type (
	DBColumn        = sdata.DBColumn
	DBTable         = sdata.DBTable
	DBInfo          = sdata.DBInfo
	DBSchema        = sdata.DBSchema
	DBRel           = sdata.DBRel
	DBRelLeft       = sdata.DBRelLeft
	DBRelRight      = sdata.DBRelRight
	RelType         = sdata.RelType
	TPath           = sdata.TPath
	DBFunction      = sdata.DBFunction
	DBFuncParam     = sdata.DBFuncParam
	CompositeFKInfo = sdata.CompositeFKInfo
	VirtualTable    = sdata.VirtualTable
)

const (
	RelOneToMany    = sdata.RelOneToMany
	RelOneToOne     = sdata.RelOneToOne
	RelPolymorphic  = sdata.RelPolymorphic
	RelRemote       = sdata.RelRemote
	RelEmbedded     = sdata.RelEmbedded
	RelRecursive    = sdata.RelRecursive
	RelNone         = sdata.RelNone
	RelDatabaseJoin = sdata.RelDatabaseJoin
	RelSkip         = sdata.RelSkip
)

var (
	NewDBSchema             = sdata.NewDBSchema
	NewDBInfo               = sdata.NewDBInfo
	NewDBTable              = sdata.NewDBTable
	GetTestDBInfo           = sdata.GetTestDBInfo
	GetTestSchema           = sdata.GetTestSchema
	MarshalDBInfoSnapshot   = sdata.MarshalDBInfoSnapshot
	UnmarshalDBInfoSnapshot = sdata.UnmarshalDBInfoSnapshot
	PathToRel               = sdata.PathToRel
	ErrPathNotFound         = sdata.ErrPathNotFound
	GetTestCompositeFKDBInfo = sdata.GetTestCompositeFKDBInfo
	DBFunctionSortKey       = sdata.DBFunctionSortKey
)
