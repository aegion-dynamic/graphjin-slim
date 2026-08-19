package config

import (
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/cache"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
)

// Service contains HTTP service settings layered onto core.Config.
type Service struct {
	AppName                    string          `mapstructure:"app_name" jsonschema:"title=Application Name"`
	Production                 bool            `jsonschema:"title=Production Mode,default=false"`
	DisableColumnValueSampling bool            `mapstructure:"disable_column_value_sampling" jsonschema:"title=Disable Catalog Column Value Sampling,default=false"`
	ConfigPath                 string          `mapstructure:"config_path" jsonschema:"title=Config Path"`
	LogLevel                   string          `mapstructure:"log_level" jsonschema:"title=Log Level,enum=debug,enum=error,enum=warn,enum=info"`
	LogFormat                  string          `mapstructure:"log_format" jsonschema:"title=Logging Format,enum=auto,enum=json,enum=simple"`
	HostPort                   string          `mapstructure:"host_port" jsonschema:"title=Host and Port"`
	Host                       string          `jsonschema:"title=Host"`
	Port                       string          `jsonschema:"title=Port"`
	HTTPGZip                   bool            `mapstructure:"http_compress" jsonschema:"title=Enable Compression,default=true"`
	RateLimiter                RateLimiter     `mapstructure:"rate_limiter" jsonschema:"title=Set API Rate Limiting"`
	ServerTiming               bool            `mapstructure:"server_timing" jsonschema:"title=Server Timing HTTP Header,default=true"`
	WebUI                      bool            `mapstructure:"web_ui" jsonschema:"title=Enable Web UI,default=false"`
	EnableTracing              bool            `mapstructure:"enable_tracing" jsonschema:"title=Enable Tracing,default=true"`
	WatchAndReload             bool            `mapstructure:"reload_on_config_change" jsonschema:"title=Reload Config"`
	AuthFailBlock              bool            `mapstructure:"auth_fail_block" jsonschema:"title=Block Request on Authorization Failure"`
	AllowedOrigins             []string        `mapstructure:"cors_allowed_origins" jsonschema:"title=HTTP CORS Allowed Origins"`
	AllowedHeaders             []string        `mapstructure:"cors_allowed_headers" jsonschema:"title=HTTP CORS Allowed Headers"`
	DebugCORS                  bool            `mapstructure:"cors_debug" jsonschema:"title=Log CORS"`
	CacheControl               string          `mapstructure:"cache_control" jsonschema:"title=Enable Cache-Control"`
	DB                         database.Config `mapstructure:"database" jsonschema:"title=Database"`
	Secrets                    SecretsConfig   `mapstructure:"secrets" jsonschema:"title=Secrets"`
	Redis                      RedisConfig     `mapstructure:"redis" jsonschema:"title=Redis Configuration"`
	Caching                    cache.Config    `mapstructure:"caching" jsonschema:"title=Caching Configuration"`
}

type SecretsConfig struct {
	Keystore KeystoreConfig `mapstructure:"keystore" jsonschema:"title=Encrypted Keystore"`
}
type KeystoreConfig struct {
	Key  string `mapstructure:"key" jsonschema:"title=Keystore Key" jsonschema_extras:"x-graphjin-sensitive=secret"`
	Path string `mapstructure:"path" jsonschema:"title=Keystore Path"`
}
type RateLimiter struct {
	Rate     float64 `jsonschema:"title=Connection Rate"`
	Bucket   int     `jsonschema:"title=Bucket Size"`
	IPHeader string  `mapstructure:"ip_header" jsonschema:"title=IP From HTTP Header,example=X-Forwarded-For"`
}
type RedisConfig struct {
	URL string `mapstructure:"url" jsonschema:"title=Redis URL"`
}

func (s Service) DevMode() bool            { return !s.Production }
func (s Service) RateLimiterEnabled() bool { return s.RateLimiter.Rate > 0 }
