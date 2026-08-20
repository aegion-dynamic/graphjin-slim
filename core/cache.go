package core

import (
	"github.com/aegion-dynamic/graphjin-slim/core/v3/cache"
)

// Response-cache types live in core/cache; core re-exports them for the
// stable public API used by serv and embedders.

type ResponseCacheProvider = cache.ResponseCacheProvider
type ResponseCacheProviderWithOptions = cache.ResponseCacheProviderWithOptions
type CacheEntryOptions = cache.CacheEntryOptions
type RefreshFn = cache.RefreshFn
type RefreshFnWithOptions = cache.RefreshFnWithOptions
type SWRRefresher = cache.SWRRefresher
type SWRRefresherWithOptions = cache.SWRRefresherWithOptions
type Cache = cache.Cache

// initCache initializes the local APQ/introspection cache.
func (gj *graphjinEngine) initCache() (err error) {
	gj.cache, err = cache.NewLocal(5000)
	return
}
