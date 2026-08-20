package core

import (
	"context"
	"encoding/json"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/cache"
)

// Cache key types live in core/cache; core re-exports them for the public API.

type CacheKeyBuilder = cache.CacheKeyBuilder

func NewCacheKeyBuilder() *CacheKeyBuilder {
	return cache.NewCacheKeyBuilder()
}

func BuildCacheKey(
	ctx context.Context,
	opName string,
	apqKey string,
	query []byte,
	vars json.RawMessage,
	role string,
	databases ...string,
) string {
	return cache.BuildCacheKey(ctx, opName, apqKey, query, vars, role, databases...)
}

func ShouldCacheQuery(opName, apqKey string) bool {
	return cache.ShouldCacheQuery(opName, apqKey)
}
