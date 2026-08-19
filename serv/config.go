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
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
)

type (
	Core           = core.Config
	Database       = database.Config
	CachingConfig  = cache.Config
	Serv           = configmodule.Service
	SecretsConfig  = configmodule.SecretsConfig
	KeystoreConfig = configmodule.KeystoreConfig
	RateLimiter    = configmodule.RateLimiter
	RedisConfig    = configmodule.RedisConfig
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
			configmodule.SetEnvironmentValue(viper, kv[0], kv[1])
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

// AbsolutePath returns the absolute path of the file.
func (c *Config) AbsolutePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.ConfigPath, p)
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
