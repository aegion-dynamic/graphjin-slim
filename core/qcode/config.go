package qcode

type Config struct {
	Vars            map[string]string
	TConfig         map[string]TConfig
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

type TConfig struct {
	OrderBy map[string][][2]string
}

func (co *Compiler) getTConfig(schema, name string) TConfig {
	return co.c.TConfig[(schema + name)]
}

