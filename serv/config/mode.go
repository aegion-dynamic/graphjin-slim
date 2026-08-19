// Package config contains service configuration mechanics.
package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/spf13/viper"
)

// ApplyFilenameMode applies a dev/prod filename default without overriding an
// explicit mode setting.
func ApplyFilenameMode(v *viper.Viper, mode string) {
	if mode != "" && !v.IsSet("mode") {
		v.Set("mode", mode)
	}
}

// ModeFromFilename returns the deployment mode implied by a config filename.
func ModeFromFilename(configFile string) string {
	name := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(filepath.Base(configFile), filepath.Ext(configFile))))
	switch name {
	case "development", "dev":
		return "dev"
	case "production", "prod":
		return "prod"
	default:
		return ""
	}
}

// ProductionFromViperMode resolves the legacy production flag and mode field.
func ProductionFromViperMode(v *viper.Viper) (bool, error) {
	mode, err := core.CanonicalMode(v.GetString("mode"))
	if err != nil {
		return false, err
	}
	switch mode {
	case "":
		return v.GetBool("production"), nil
	case "dev":
		return false, nil
	case "prod":
		return true, nil
	default:
		return false, nil
	}
}

// NormalizeAutoBoolSetting parses a boolean setting that accepts auto.
func NormalizeAutoBoolSetting(v *viper.Viper, key string, autoValue bool) error {
	if !v.IsSet(key) {
		return nil
	}
	raw := strings.TrimSpace(strings.ToLower(v.GetString(key)))
	switch raw {
	case "", "auto":
		v.Set(key, autoValue)
	case "true", "1", "yes", "on":
		v.Set(key, true)
	case "false", "0", "no", "off":
		v.Set(key, false)
	default:
		return fmt.Errorf("%s must be true, false, or auto", key)
	}
	return nil
}
