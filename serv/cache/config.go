// Package cache contains response-cache implementations used by the service.
package cache

// Config controls response-cache behavior.
type Config struct {
	Disable       bool     `mapstructure:"disable" jsonschema:"title=Disable Caching,default=false"`
	TTL           int      `mapstructure:"ttl" jsonschema:"title=Cache TTL,default=3600"`
	FreshTTL      int      `mapstructure:"fresh_ttl" jsonschema:"title=Fresh TTL for SWR,default=300"`
	ExcludeTables []string `mapstructure:"exclude_tables" jsonschema:"title=Exclude Tables"`
}

// CachingConfig is retained inside this package for the existing cache tests.
type CachingConfig = Config
