package core

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
)

// Cache is a small local LRU used for APQ and introspection JSON.
type Cache struct {
	cache *lru.TwoQueueCache[string, []byte]
}

func (gj *graphjinEngine) initCache() (err error) {
	gj.cache.cache, err = lru.New2Q[string, []byte](500)
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

// Minimal types still referenced by multi-DB fragment call sites (no-op).
type RowRef struct{ Source, Scope, Kind, Table, ID string }

func (r RowRef) Normalize() RowRef               { return r }
func (r RowRef) DependencyKey() string           { return "" }
func (r RowRef) TableDependency() (RowRef, bool) { return RowRef{}, false }

type CacheEntryOptions struct {
	HardTTL, FreshTTL time.Duration
	NoStore           bool
}

const (
	fragmentKindDBRoot = "db-root"
	fragmentKindDBJoin = "db-join"
	swrRefreshTimeout  = 30 * time.Second
)

func (s *gstate) fragmentCacheEnabled(*qcode.QCode) bool { return false }
func (s *gstate) fragmentCacheGet(context.Context, string, func() ([]byte, []RowRef, CacheEntryOptions, error)) ([]byte, bool) {
	return nil, false
}
func (s *gstate) fragmentCacheSet(context.Context, string, []byte, []RowRef, time.Time, CacheEntryOptions) {
}
func (s *gstate) processDBFragmentForCache(string, *qcode.QCode, []byte) ([]byte, []RowRef, error) {
	return nil, nil, nil
}
func (s *gstate) dbFragmentKey(context.Context, string, string, string, []interface{}, *qcode.QCode) string {
	return ""
}
func (s *gstate) tryCacheGet(context.Context) bool         { return false }
func (s *gstate) tryCacheSet(context.Context)              {}
func (s *gstate) invalidateCache(context.Context)          {}
func (s *gstate) submitSWRRefresh(context.Context)         {}
func stripCacheTrackingFields(data []byte) ([]byte, error) { return data, nil }
