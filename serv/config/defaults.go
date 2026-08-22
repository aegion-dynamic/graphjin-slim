package config

import "github.com/spf13/viper"

// NewViperWithDefaults returns the service configuration reader with all
// supported defaults and environment bindings installed.
func NewViperWithDefaults() *viper.Viper {
	vi := viper.New()

	vi.SetDefault("host_port", "0.0.0.0:8080")
	vi.SetDefault("enable_tracing", false)
	vi.SetDefault("auth_fail_block", false)
	vi.SetDefault("seed_file", "seed.js")
	vi.SetDefault("log_level", "info")
	vi.SetDefault("log_format", "auto")
	vi.SetDefault("default_block", true)

	vi.SetDefault("database.type", "postgres")
	vi.SetDefault("database.host", "localhost")
	vi.SetDefault("database.port", 5432)
	vi.SetDefault("database.user", "postgres")
	vi.SetDefault("database.password", "")
	vi.SetDefault("database.schema", "public")
	vi.SetDefault("database.pool_size", 10)
	vi.SetDefault("env", "development")

	vi.BindEnv("env", "GO_ENV")
	vi.BindEnv("host", "HOST")
	vi.BindEnv("port", "PORT")
	vi.BindEnv("default_limit", "GJ_DEFAULT_LIMIT", "SG_DEFAULT_LIMIT", "SJ_DEFAULT_LIMIT")
	vi.SetDefault("auth.subs_creds_in_vars", false)

	return vi
}
