package serv

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/cache"
	configmodule "github.com/aegion-dynamic/graphjin-slim/serv/v3/config"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/internal/util"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
)

type (
	Core          = core.Config
	Database      = database.Config
	CachingConfig = cache.Config
)

//go:generate go run ./internal/tools -o config.schema.json

// Configuration for the GraphJin service
type Config struct {
	// Configuration for the GraphJin compiler core
	Core `mapstructure:",squash" jsonschema:"title=Compiler Configuration"`

	// Configuration for the GraphJin Service
	Serv `mapstructure:",squash" jsonschema:"title=Service Configuration"`

	// Name of another config file in the same directory to inherit and
	// override (one level only; the inherited file cannot itself inherit).
	Inherits string `mapstructure:"inherits" jsonschema:"title=Inherit From Config File"`

	hostPort string
	hash     string
	name     string
	dirty    bool
	viper    *viper.Viper

	webUIExplicit        bool
	parsedConfig         bool
	managedArtifactStore bool
	explicitSettings     map[string]bool
}

// Configuration for the GraphJin Service
type Serv struct {
	// Application name is used in log and debug messages
	AppName string `mapstructure:"app_name" jsonschema:"title=Application Name"`

	// Legacy production switch. Prefer top-level mode for new configs.
	Production bool `jsonschema:"title=Production Mode,default=false"`

	// Stops the catalog from sampling the small value sets of enum-like columns
	DisableColumnValueSampling bool `mapstructure:"disable_column_value_sampling" jsonschema:"title=Disable Catalog Column Value Sampling,default=false"`

	// The default path to find all configuration files and scripts
	ConfigPath string `mapstructure:"config_path" jsonschema:"title=Config Path"`

	// Logging level must be one of debug, error, warn, info
	LogLevel string `mapstructure:"log_level" jsonschema:"title=Log Level,enum=debug,enum=error,enum=warn,enum=info"`

	// Logging Format: "auto", "json", or "simple"
	LogFormat string `mapstructure:"log_format" jsonschema:"title=Logging Format,enum=auto,enum=json,enum=simple"`

	// The host and port the service runs on. Example localhost:8080
	HostPort string `mapstructure:"host_port" jsonschema:"title=Host and Port"`

	// Host to run the service on
	Host string `jsonschema:"title=Host"`

	// Port to run the service on
	Port string `jsonschema:"title=Port"`

	// Enables HTTP compression
	HTTPGZip bool `mapstructure:"http_compress" jsonschema:"title=Enable Compression,default=true"`

	// Sets the API rate limits
	RateLimiter RateLimiter `mapstructure:"rate_limiter" jsonschema:"title=Set API Rate Limiting"`

	// Enables the Server-Timing HTTP header
	ServerTiming bool `mapstructure:"server_timing" jsonschema:"title=Server Timing HTTP Header,default=true"`

	// Enable the web UI
	WebUI bool `mapstructure:"web_ui" jsonschema:"title=Enable Web UI,default=false"`

	// Enable OpenTrace request tracing
	EnableTracing bool `mapstructure:"enable_tracing" jsonschema:"title=Enable Tracing,default=true"`

	// Enables reloading the service on config changes
	WatchAndReload bool `mapstructure:"reload_on_config_change" jsonschema:"title=Reload Config"`

	// Enable blocking requests with a HTTP 401 on auth failure
	AuthFailBlock bool `mapstructure:"auth_fail_block" jsonschema:"title=Block Request on Authorization Failure"`

	// Sets the HTTP CORS Access-Control-Allow-Origin header
	AllowedOrigins []string `mapstructure:"cors_allowed_origins" jsonschema:"title=HTTP CORS Allowed Origins"`

	// Sets the HTTP CORS Access-Control-Allow-Headers header
	AllowedHeaders []string `mapstructure:"cors_allowed_headers" jsonschema:"title=HTTP CORS Allowed Headers"`

	// Enables debug logs for CORS
	DebugCORS bool `mapstructure:"cors_debug" jsonschema:"title=Log CORS"`

	// Sets the HTTP Cache-Control header
	CacheControl string `mapstructure:"cache_control" jsonschema:"title=Enable Cache-Control"`

	// Database configuration
	DB Database `mapstructure:"database" jsonschema:"title=Database"`

	// Local encrypted secrets configuration
	Secrets SecretsConfig `mapstructure:"secrets" jsonschema:"title=Secrets"`

	// Redis configuration
	Redis RedisConfig `mapstructure:"redis" jsonschema:"title=Redis Configuration"`

	// Response caching configuration
	Caching CachingConfig `mapstructure:"caching" jsonschema:"title=Caching Configuration"`
}

// SecretsConfig configures secret storage for write-only config inputs.
type SecretsConfig struct {
	Keystore KeystoreConfig `mapstructure:"keystore" jsonschema:"title=Encrypted Keystore"`
}

// KeystoreConfig configures the local encrypted keystore.
type KeystoreConfig struct {
	Key  string `mapstructure:"key" jsonschema:"title=Keystore Key" jsonschema_extras:"x-graphjin-sensitive=secret"`
	Path string `mapstructure:"path" jsonschema:"title=Keystore Path"`
}

// RateLimiter sets the API rate limits
type RateLimiter struct {
	Rate     float64 `jsonschema:"title=Connection Rate"`
	Bucket   int     `jsonschema:"title=Bucket Size"`
	IPHeader string  `mapstructure:"ip_header" jsonschema:"title=IP From HTTP Header,example=X-Forwarded-For"`
}

// RedisConfig configures Redis connection
type RedisConfig struct {
	URL string `mapstructure:"url" jsonschema:"title=Redis URL"`
}

// ReadInConfig reads the config file for the environment.
func ReadInConfig(configFile string) (*Config, error) {
	return readInConfig(configFile, nil)
}

// ReadInConfigFS is the same as ReadInConfig but with a filesystem argument.
func ReadInConfigFS(configFile string, fs afero.Fs) (*Config, error) {
	return readInConfig(configFile, fs)
}

func readInConfig(configFile string, fs afero.Fs) (*Config, error) {
	cp := filepath.Dir(configFile)
	configName := filepath.Base(configFile)
	selectedMode := modeFromConfigName(configName)
	viper := newViper(cp, configName)

	if fs != nil {
		viper.SetFs(fs)
	}

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if pcf := viper.GetString("inherits"); pcf != "" {
		cf := viper.ConfigFileUsed()
		viper = newViper(cp, pcf)
		if fs != nil {
			viper.SetFs(fs)
		}

		if err := viper.ReadInConfig(); err != nil {
			return nil, err
		}

		if value := viper.GetString("inherits"); value != "" {
			return nil, fmt.Errorf("inherited config '%s' cannot itself inherit '%s'", pcf, value)
		}

		viper.SetConfigFile(cf)

		if err := viper.MergeInConfig(); err != nil {
			return nil, err
		}
	}

	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GJ_") || strings.HasPrefix(e, "SJ_") {
			kv := strings.SplitN(e, "=", 2)
			util.SetKeyValue(viper, kv[0], kv[1])
		}
	}
	applyConfigNameMode(viper, selectedMode)

	config := &Config{viper: viper}
	config.ConfigPath = cp
	config.parsedConfig = true
	config.explicitSettings = captureRuntimeDefaultExplicitSettings(viper)

	webUIExplicit := webUISettingExplicit(viper)
	if err := viper.Unmarshal(&config, capabilityMapDecodeOption()); err != nil {
		return nil, fmt.Errorf("failed to decode config, %v", err)
	}
	if err := normalizeConfigMode(config); err != nil {
		return nil, err
	}
	normalizeWebUIDefault(config, webUIExplicit)
	config.webUIExplicit = webUIExplicit

	return config, nil
}

// NewConfig creates a new GraphJin configuration from the provided config string.
func NewConfig(config, format string) (*Config, error) {
	if format == "" {
		format = "yaml"
	}

	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "SG_") {
			continue
		}
		kv := strings.SplitN(e, "=", 2)
		if err := os.Setenv("GJ_"+kv[0][3:], kv[1]); err != nil {
			return nil, err
		}
	}

	viper := newViperWithDefaults()
	viper.SetConfigType(format)

	if err := viper.ReadConfig(strings.NewReader(config)); err != nil {
		return nil, err
	}

	c := &Config{viper: viper, parsedConfig: true}
	c.explicitSettings = captureRuntimeDefaultExplicitSettings(viper)

	webUIExplicit := webUISettingExplicit(viper)
	if err := viper.Unmarshal(&c, capabilityMapDecodeOption()); err != nil {
		return nil, fmt.Errorf("failed to decode config, %v", err)
	}
	if err := normalizeConfigMode(c); err != nil {
		return nil, err
	}
	normalizeWebUIDefault(c, webUIExplicit)
	c.webUIExplicit = webUIExplicit

	return c, nil
}

var boolMapType = reflect.TypeOf(map[string]bool{})

func capabilityMapDecodeOption() viper.DecoderConfigOption {
	return func(config *mapstructure.DecoderConfig) {
		config.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			config.DecodeHook,
			mapstructure.DecodeHookFuncType(flattenBoolMapDecodeHook),
		)
	}
}

func flattenBoolMapDecodeHook(from, to reflect.Type, data any) (any, error) {
	if from == nil || to != boolMapType || from.Kind() != reflect.Map {
		return data, nil
	}
	out := make(map[string]bool)
	if err := flattenBoolMapValue("", reflect.ValueOf(data), out); err != nil {
		return nil, err
	}
	return out, nil
}

func flattenBoolMapValue(prefix string, value reflect.Value, out map[string]bool) error {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return fmt.Errorf("capability %q must be true or false", prefix)
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return fmt.Errorf("capability %q must be true or false", prefix)
	}
	switch value.Kind() {
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			key := strings.TrimSpace(fmt.Sprint(iter.Key().Interface()))
			if key == "" {
				return fmt.Errorf("capability key cannot be empty")
			}
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if err := flattenBoolMapValue(path, iter.Value(), out); err != nil {
				return err
			}
		}
		return nil
	case reflect.Bool:
		out[prefix] = value.Bool()
		return nil
	case reflect.String:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value.String()))
		if err != nil {
			return fmt.Errorf("capability %q must be true or false", prefix)
		}
		out[prefix] = parsed
		return nil
	default:
		return fmt.Errorf("capability %q must be true or false", prefix)
	}
}

var runtimeDefaultKeys = []string{}

func captureRuntimeDefaultExplicitSettings(v *viper.Viper) map[string]bool {
	explicit := make(map[string]bool, len(runtimeDefaultKeys))
	for _, key := range runtimeDefaultKeys {
		explicit[key] = configSettingExplicit(v, key)
	}
	return explicit
}

func configSettingExplicit(v *viper.Viper, key string) bool {
	if v != nil && v.InConfig(key) {
		return true
	}
	envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	for _, prefix := range []string{"GJ_", "SG_", "SJ_"} {
		if value, ok := os.LookupEnv(prefix + envKey); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func (c *Config) settingExplicit(key string) bool {
	if c == nil {
		return false
	}
	if c.explicitSettings != nil {
		if explicit, ok := c.explicitSettings[key]; ok {
			return explicit
		}
	}
	return configSettingExplicit(c.viper, key)
}

func normalizeWebUIDefault(c *Config, explicit bool) {
	if c == nil || explicit {
		return
	}
	c.Serv.WebUI = c.Core.Mode == "dev"
}

func webUISettingExplicit(v *viper.Viper) bool {
	if v == nil {
		return false
	}
	if v.InConfig("web_ui") {
		return true
	}
	for _, key := range []string{"GJ_WEB_UI", "SG_WEB_UI", "SJ_WEB_UI"} {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func applyConfigNameMode(v *viper.Viper, mode string) {
	configmodule.ApplyFilenameMode(v, mode)
}

func modeFromConfigName(configFile string) string {
	return configmodule.ModeFromFilename(configFile)
}

func normalizeConfigMode(c *Config) error {
	if c == nil {
		return nil
	}
	if c.Serv.Production {
		c.Core.Production = true
	}
	return c.Core.NormalizeMode()
}

func productionFromViperMode(v *viper.Viper) (bool, error) {
	return configmodule.ProductionFromViperMode(v)
}

func normalizeAutoBoolSetting(v *viper.Viper, key string, autoValue bool) error {
	return configmodule.NormalizeAutoBoolSetting(v, key, autoValue)
}

func newViperWithDefaults() *viper.Viper {
	return configmodule.NewViperWithDefaults()
}

func removedSettingEnvPresent(suffix string) bool {
	for _, prefix := range []string{"GJ_", "SG_", "SJ_"} {
		if _, ok := os.LookupEnv(prefix + suffix); ok {
			return true
		}
	}
	return false
}

// ShouldUseJSONLogs returns true if logs should be in JSON format.
func (c *Config) ShouldUseJSONLogs() bool {
	return c.Serv.LogFormat == "json" || (!c.Serv.DevMode() && c.Serv.LogFormat == "auto")
}

// DevMode returns true if the config is in development mode.
func (s Serv) DevMode() bool {
	return !s.Production
}

// AbsolutePath returns the absolute path of the file.
func (c *Config) AbsolutePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.ConfigPath, p)
}

// rateLimiterEnable returns true if rate limiting is enabled.
func (s Serv) rateLimiterEnable() bool {
	return s.RateLimiter.Rate > 0
}

// GetConfigName returns the config file name.
func GetConfigName() string {
	return "config.yml"
}

// newViper creates a new viper instance for the given config path and name.
func newViper(configPath string, configName string) *viper.Viper {
	vi := viper.New()
	vi.SetConfigName(configName)
	vi.AddConfigPath(configPath)
	vi.AutomaticEnv()
	return vi
}

// managedArtifactStore is a flag for artifact management.
// In the slim build, this is always false.
var _ = (*Config)(nil)
