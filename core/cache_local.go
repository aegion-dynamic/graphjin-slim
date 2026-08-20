package core

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/qcode"
)

// Cache is a small local LRU used for APQ and introspection JSON.
type Cache struct {
	cache *lru.TwoQueueCache[string, []byte]
}

func (gj *graphjinEngine) initCache() (err error) {
	gj.cache.cache, err = lru.New2Q[string, []byte](5000)
	return
}

func (c Cache) Get(key string) (val []byte, fromCache bool) {
	if c.cache == nil {
		return nil, false
	}
	return c.cache.Get(key)
}

func (c Cache) Set(key string, val []byte) {
	if c.cache == nil {
		return
	}
	c.cache.Add(key, val)
}

type RowRef struct {
	Source, Scope, Kind, Table, ID string
}

func (r RowRef) Normalize() RowRef { return r }
func (r RowRef) DependencyKey() string {
	return r.Source + ":" + r.Scope + ":" + r.Kind + ":" + r.Table + ":" + r.ID
}
func (r RowRef) TableDependency() (RowRef, bool) {
	if r.Table == "" {
		return RowRef{}, false
	}
	return r, true
}

type CacheEntryOptions struct {
	HardTTL, FreshTTL time.Duration
	NoStore           bool
}

type ResponseCacheProvider interface {
	Get(ctx context.Context, key string) (data []byte, isStale bool, found bool)
	Set(ctx context.Context, key string, data []byte, refs []RowRef, queryStartTime time.Time) error
	InvalidateRows(ctx context.Context, refs []RowRef) error
}

type ResponseCacheProviderWithOptions interface {
	SetWithOptions(ctx context.Context, key string, data []byte, refs []RowRef, queryStartTime time.Time, opts CacheEntryOptions) error
}

type RefreshFn func() (data []byte, refs []RowRef, err error)
type RefreshFnWithOptions func() (data []byte, refs []RowRef, opts CacheEntryOptions, err error)

type SWRRefresher interface {
	SubmitRefresh(key string, fn RefreshFn) bool
}
type SWRRefresherWithOptions interface {
	SubmitRefreshWithOptions(key string, fn RefreshFnWithOptions) bool
}

type CacheKeyBuilder struct{}

func NewCacheKeyBuilder() *CacheKeyBuilder { return &CacheKeyBuilder{} }

func (b *CacheKeyBuilder) Build(ctx context.Context, opName, apqKey string, query []byte, vars []byte, role string, databases ...string) string {
	return ""
}
func (b *CacheKeyBuilder) BuildFragment(ctx context.Context, kind, role string, parts map[string]interface{}) string {
	return ""
}
func (b *CacheKeyBuilder) ShouldCache(opName, apqKey string) bool { return false }

func BuildCacheKey(ctx context.Context, opName, apqKey string, query []byte, vars []byte, role string, databases ...string) string {
	return ""
}
func ShouldCacheQuery(opName, apqKey string) bool { return false }

const (
	CacheSourceDB      = "db"
	CacheSourceCodeSQL = "codesql"
	CacheSourceFS      = "fs"
	CacheSourceRemote  = "remote"
	CacheKindRow       = "row"
	CacheKindTable     = "table"
	CacheKindKey       = "key"
	CacheKindPrefix    = "prefix"
	CacheKindResolver  = "resolver"
)

func DBRowRef(database, table, id string) RowRef {
	return RowRef{Source: CacheSourceDB, Scope: database, Kind: CacheKindRow, Table: table, ID: id}
}
func DBTableRef(database, table string) RowRef {
	return RowRef{Source: CacheSourceDB, Scope: database, Kind: CacheKindTable, Table: table}
}
func CodeSQLTableRefs(database string, tables []string) []RowRef { return nil }
func RemoteResolverRef(scope, id string) RowRef {
	return RowRef{Source: CacheSourceRemote, Scope: scope, Kind: CacheKindResolver, ID: id}
}
func RemoteResolverRefs(scope string, ids ...string) []RowRef { return nil }
func FilesystemKeyRefs(name, key string) []RowRef             { return nil }
func FilesystemPrefixRefs(name, prefix string) []RowRef       { return nil }
func FilesystemPrefixRef(name, prefix string) RowRef {
	return RowRef{Source: CacheSourceFS, Scope: name, Kind: CacheKindPrefix, ID: prefix}
}
func ExtractMutationRefs(qc *qcode.QCode, data []byte) []RowRef { return nil }
func NewResponseProcessor(qc *qcode.QCode) *ResponseProcessor   { return &ResponseProcessor{} }

type ResponseProcessor struct{}

func (rp *ResponseProcessor) ProcessForCache(data []byte) ([]byte, []RowRef, error) {
	return data, nil, nil
}

func stripCacheTrackingFields(data []byte) ([]byte, error) { return data, nil }

// No-op response-cache hot-path methods (provider optional / unused in slim).
func (s *gstate) tryCacheGet(c context.Context) bool       { return false }
func (s *gstate) tryCacheSet(c context.Context)            {}
func (s *gstate) invalidateCache(c context.Context)        {}
func (s *gstate) submitSWRRefresh(c context.Context)       {}
func (s *gstate) getAPQKey() string                        { return "" }
func (s *gstate) hasOffsetPagination(qc *qcode.QCode) bool { return false }
func (s *gstate) cacheDatabaseScope() string               { return s.database }

func (s *gstate) fragmentCacheEnabled(qc *qcode.QCode) bool { return false }
func (s *gstate) fragmentCacheGet(ctx context.Context, key string, produce func() ([]byte, []RowRef, CacheEntryOptions, error)) ([]byte, bool) {
	return nil, false
}
func (s *gstate) fragmentCacheSet(ctx context.Context, key string, data []byte, refs []RowRef, start time.Time, opts CacheEntryOptions) {
}
func (s *gstate) processDBFragmentForCache(dbName string, qc *qcode.QCode, data []byte) ([]byte, []RowRef, error) {
	return data, nil, nil
}
func (s *gstate) buildFragmentCacheKey(ctx context.Context, kind string, parts map[string]interface{}) string {
	return ""
}
func (s *gstate) scopeDBRefs(dbName string, refs []RowRef) []RowRef { return refs }
func (s *gstate) isCodeSQLDatabase(dbName string) bool              { return false }
func (s *gstate) dbFragmentKey(ctx context.Context, kind string, dbName string, querySQL string, args []interface{}, qc *qcode.QCode) string {
	return ""
}
func (s *gstate) remoteFragmentKey(ctx context.Context, source, scope, fp string, id []byte, sel *qcode.Select) string {
	return ""
}
func (s *gstate) remoteFragmentCacheOptions(source, scope string) CacheEntryOptions {
	return CacheEntryOptions{}
}
