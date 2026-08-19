// Package names allocates service-owned runtime identifiers.
package names

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
)

// Allocate returns a collision-free runtime database identifier.
func Allocate(base string, conf, runtime *core.Config, active map[string]*sql.DB) string {
	used := make(map[string]struct{})
	add := func(name string) {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			used[name] = struct{}{}
		}
	}
	if conf != nil {
		for _, source := range conf.Sources {
			add(source.Name)
		}
		for name := range conf.Databases {
			add(name)
		}
	}
	if runtime != nil {
		for name := range runtime.Databases {
			add(name)
		}
	}
	for name := range active {
		add(name)
	}
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = fmt.Sprintf("%s_%d", base, suffix)
		}
		if _, exists := used[strings.ToLower(candidate)]; !exists {
			return candidate
		}
	}
}
