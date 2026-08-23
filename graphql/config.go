package graphql

import "github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"

type Config struct {
	Vars            map[string]string
	TConfig         map[string]qcode.TConfig
	DefaultLimit    int
	AnalyticsMode   bool
	DisableAgg      bool
	DisableFuncs    bool
	EnableCamelcase bool
	DBSchema        string
	Validators      map[string]Validator

	// EnableCacheTracking injects __gj_id fields with primary keys for cache row tracking
	EnableCacheTracking bool
}

func (co *Compiler) getTConfig(schema, name string) qcode.TConfig {
	return co.c.TConfig[(schema + name)]
}
