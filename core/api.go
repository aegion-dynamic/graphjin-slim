// Package core provides an API to include and use the GraphJin compiler with your own code.
// For detailed documentation visit https://graphjin.com
package core

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/engine"
)

type (
	GraphJin              = engine.GraphJin
	Engine                = engine.Engine
	Config                = engine.Config
	Result                = engine.Result
	RequestConfig         = engine.RequestConfig
	Option                = engine.Option
	Header                = engine.Header
	Error                 = engine.Error
	FS                    = engine.FS
	Tracer                = engine.Tracer
	Cache                 = engine.Cache
	RowRef                = engine.RowRef
	OpType                = engine.OpType
	RootLimitInfo         = engine.RootLimitInfo
	DatabaseConfig        = engine.DatabaseConfig
	Table                 = engine.Table
	Column                = engine.Column
	Function              = engine.Function
	RelationshipConfig    = engine.RelationshipConfig
	Resolver              = engine.Resolver
	ResolverProps         = engine.ResolverProps
	ResolverConfig        = engine.ResolverConfig
	ResolverReq           = engine.ResolverReq
	ResolverFn            = engine.ResolverFn
	SavedQuerySaveHook    = engine.SavedQuerySaveHook
	SavedQuerySaveRequest = engine.SavedQuerySaveRequest
	SavedQueryFragment    = engine.SavedQueryFragment
	SchemaCallbacks       = engine.SchemaCallbacks
	Lifecycle             = engine.Lifecycle
	Member                = engine.Member
	OpenAPIInputs         = engine.OpenAPIInputs
)

const (
	DefaultDBName  = engine.DefaultDBName
	OpUnknown      = engine.OpUnknown
	OpQuery        = engine.OpQuery
	OpSubscription = engine.OpSubscription
	OpMutation     = engine.OpMutation
)

var (
	NewGraphJin                         = engine.NewGraphJin
	NewGraphJinWithFS                   = engine.NewGraphJinWithFS
	NewTestGraphJin                     = engine.NewTestGraphJin
	NewOsFS                             = engine.NewOsFS
	NewLifecycle                        = engine.NewLifecycle
	CanonicalMode                       = engine.CanonicalMode
	OptionSetNamespace                  = engine.OptionSetNamespace
	OptionSetFS                         = engine.OptionSetFS
	OptionSetDatabases                  = engine.OptionSetDatabases
	OptionSetSavedQuerySaveHook         = engine.OptionSetSavedQuerySaveHook
	OptionSetRuntimeSchemaDDLDir        = engine.OptionSetRuntimeSchemaDDLDir
	OptionSetRuntimeSchemaCacheFirst    = engine.OptionSetRuntimeSchemaCacheFirst
	OptionSetRuntimeSchemaCacheRequired = engine.OptionSetRuntimeSchemaCacheRequired
	OptionSetDBSchemaWatcherDisabled    = engine.OptionSetDBSchemaWatcherDisabled
	OptionSetTrace                      = engine.OptionSetTrace
	OptionSetResolver                   = engine.OptionSetResolver
	ErrNotFound                         = engine.ErrNotFound
	RepairKindTableNotFound             = engine.RepairKindTableNotFound
)
