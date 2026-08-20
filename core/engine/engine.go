package engine

import (
	"context"
	"database/sql"
	_log "log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/fstable"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/allow"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/nanodb"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/psql"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/sdata"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/openapi"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/storage"
)

// NanoDB aliases
type NanoDB = nanodb.DB

// FS is filesystem abstraction.
type FS = storage.FS

// Tracer minimal
type Tracer interface {
	Start(c context.Context, name string) (context.Context, Spaner)
	NewHTTPClient() *http.Client
}
type Spaner interface {
	SetAttributesString(attrs ...StringAttr)
	IsRecording() bool
	Error(err error)
	End()
}
type StringAttr struct{ Name, Value string }
type tracer struct{}

func (t *tracer) Start(c context.Context, name string) (context.Context, Spaner) { return c, &span{} }
func (t *tracer) NewHTTPClient() *http.Client                                    { return &http.Client{} }

type span struct{}

func (s *span) End()                                    {}
func (s *span) Error(err error)                         {}
func (s *span) IsRecording() bool                       { return false }
func (s *span) SetAttributesString(attrs ...StringAttr) {}

// Cache stub
type Cache struct{}

// ResponseCacheProvider stub
type ResponseCacheProvider interface {
	Get(ctx context.Context, key string) (data []byte, isStale bool, found bool)
	Set(ctx context.Context, key string, data []byte, refs []RowRef, queryStartTime time.Time) error
	InvalidateRows(ctx context.Context, refs []RowRef) error
}
type RowRef struct{}
type CacheKeyBuilder struct{}

func NewCacheKeyBuilder() *CacheKeyBuilder { return &CacheKeyBuilder{} }

// FilesystemBackendFactory stub
type FilesystemBackendFactory func(c FilesystemConfig) (fstable.Backend, error)
type ManagedMutationHandler interface {
	ManagedMutationTables() []string
	ExecuteManagedMutation(context.Context, ManagedMutationRequest) ([]byte, error)
}
type ManagedMutationRequest struct{}
type ManagedQueryHandler interface {
	ManagedQueryTables() []ManagedTable
	ExecuteManagedQuery(context.Context, ManagedQueryRequest) ([]byte, error)
}
type ManagedTable struct{ Name string }
type ManagedQueryRequest struct{}
type ReservedRoleAuthorizer func(context.Context, string) bool
type SavedQuerySaveHook func(context.Context, SavedQuerySaveRequest) (bool, error)
type SavedQuerySaveRequest struct{}
type roleQueryMode int
type stmt struct{}
type compiledRoleMatch struct{}
type resItem struct{}
type ResolverFn func() error
type RequestConfig struct{}
type FederationConfig struct{ Enabled bool }

// DBContext holds per-database state for multi-database support.
type DBContext struct {
	Name          string
	DB            *sql.DB
	DBType        string
	Nano          *NanoDB
	DBInfo        *sdata.DBInfo
	Schema        *sdata.DBSchema
	QcodeCompiler *qcode.Compiler
	PsqlCompiler  *psql.Compiler
}

// Engine is exported version of graphjinEngine.
type Engine struct {
	Conf                       *Config
	CatalogConf                *Config
	Log                        *_log.Logger
	FS                         FS
	Trace                      Tracer
	AllowList                  *allow.List
	SavedQuerySaveHook         SavedQuerySaveHook
	EncryptionKey              [32]byte
	EncryptionKeySet           bool
	Cache                      Cache
	Queries                    sync.Map
	SqliteConflictGetMu        sync.Mutex
	Roles                      map[string]*Role
	RoleStatement              string
	RoleStatementMetadata      psql.Metadata
	RoleQueryMode              roleQueryMode
	RoleGraphQLStmt            stmt
	RoleGraphQLMatches         []compiledRoleMatch
	Tmap                       map[string]qcode.TConfig
	Rtmap                      map[string]ResolverFn
	Rmap                       map[string]resItem
	OpenapiRuntime             *openapi.Runtime
	AbacEnabled                bool
	Subs                       sync.Map
	Prod                       bool
	ProdSec                    bool
	Learn                      bool
	Namespace                  string
	PrintFormat                []byte
	Opts                       []Option
	RuntimeSchemaDDLDir        string
	RuntimeSchemaCacheFirst    bool
	RuntimeSchemaCacheRequired bool
	DisableDBSchemaWatcher     bool
	Done                       chan bool

	Databases map[string]*DBContext
	DefaultDB string

	ResponseCache   ResponseCacheProvider
	CacheKeyBuilder *CacheKeyBuilder

	FederationSDLOnce sync.Once
	FederationSDL     string
	FederationSDLErr  error

	FsFactories map[string]FilesystemBackendFactory
	FsBackends  map[string]fstable.Backend

	ManagedMutationHandlers map[string]ManagedMutationHandler
	ManagedQueryHandlers    map[string]ManagedQueryHandler

	ReservedRoleAuthorizer ReservedRoleAuthorizer
}

// PrimaryDB returns the default database context.
func (gj *Engine) PrimaryDB() *DBContext {
	if ctx, ok := gj.Databases[gj.DefaultDB]; ok {
		return ctx
	}
	return nil
}

// AnyDatabaseReady returns true if at least one database has an initialized schema.
func (gj *Engine) AnyDatabaseReady() bool {
	for _, ctx := range gj.Databases {
		if ctx.Schema != nil {
			return true
		}
	}
	return false
}

// GraphJin is the outer manager, holding lifecycle and callbacks.
type GraphJin struct {
	atomic.Value
	Lifecycle       *Lifecycle
	ReloadMu        sync.Mutex
	SchemaCallbacks SchemaCallbacks
}

// Option configures Engine.
type Option func(*Engine) error
