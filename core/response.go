package core

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/cache"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/qcode"
)

// Cache dependency and response-processing types live in core/cache.
// core re-exports them so existing imports of core.RowRef etc. stay stable.

const (
	CacheSourceDB      = cache.CacheSourceDB
	CacheSourceCodeSQL = cache.CacheSourceCodeSQL
	CacheSourceFS      = cache.CacheSourceFS
	CacheSourceRemote  = cache.CacheSourceRemote

	CacheKindRow      = cache.CacheKindRow
	CacheKindTable    = cache.CacheKindTable
	CacheKindKey      = cache.CacheKindKey
	CacheKindPrefix   = cache.CacheKindPrefix
	CacheKindResolver = cache.CacheKindResolver
)

type RowRef = cache.RowRef
type ResponseProcessor = cache.ResponseProcessor

func NewResponseProcessor(qc *qcode.QCode) *ResponseProcessor {
	return cache.NewResponseProcessor(qc)
}

func DBRowRef(database, table, id string) RowRef {
	return cache.DBRowRef(database, table, id)
}

func DBTableRef(database, table string) RowRef {
	return cache.DBTableRef(database, table)
}

func CodeSQLTableRefs(database string, tables []string) []RowRef {
	return cache.CodeSQLTableRefs(database, tables)
}

func RemoteResolverRef(scope, id string) RowRef {
	return cache.RemoteResolverRef(scope, id)
}

func RemoteResolverRefs(scope string, ids ...string) []RowRef {
	return cache.RemoteResolverRefs(scope, ids...)
}

func FilesystemKeyRefs(name, key string) []RowRef {
	return cache.FilesystemKeyRefs(name, key)
}

func FilesystemPrefixRefs(name, prefix string) []RowRef {
	return cache.FilesystemPrefixRefs(name, prefix)
}

func FilesystemPrefixRef(name, prefix string) RowRef {
	return cache.FilesystemPrefixRef(name, prefix)
}

func FilesystemKeyRef(name, key string) RowRef {
	return cache.FilesystemKeyRef(name, key)
}

func ExtractMutationRefs(qc *qcode.QCode, data []byte) []RowRef {
	return cache.ExtractMutationRefs(qc, data)
}

func StripCacheTrackingFields(data []byte) ([]byte, error) {
	return cache.StripCacheTrackingFields(data)
}

func StringifyID(id interface{}) string {
	return cache.StringifyID(id)
}

func stringifyID(id interface{}) string {
	return cache.StringifyID(id)
}

// Unexported helpers kept for call sites still in package core.
func filesystemKeyRef(name, key string) RowRef       { return cache.FilesystemKeyRef(name, key) }
func filesystemPrefixRef(name, prefix string) RowRef { return cache.FilesystemPrefixRef(name, prefix) }
func stripCacheTrackingFields(data []byte) ([]byte, error) {
	return cache.StripCacheTrackingFields(data)
}
