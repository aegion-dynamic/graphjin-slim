package config

import (
	"strings"

	"github.com/spf13/viper"
)

// SetEnvironmentValue maps a GraphJin environment variable name to the
// deepest matching Viper key. Unknown environment variables are ignored.
func SetEnvironmentValue(v *viper.Viper, key string, value any) bool {
	if strings.HasPrefix(key, "GJ_") || strings.HasPrefix(key, "SG_") {
		key = key[3:]
	}
	underscoreCount := strings.Count(key, "_")
	candidate := strings.ToLower(key)
	if v.Get(candidate) != nil {
		v.Set(candidate, value)
		return true
	}
	for i := 0; i < underscoreCount; i++ {
		candidate = strings.Replace(candidate, "_", ".", 1)
		if v.Get(candidate) != nil {
			v.Set(candidate, value)
			return true
		}
	}
	return false
}
