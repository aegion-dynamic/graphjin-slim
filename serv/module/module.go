// Package module is the seam between the service and optional product-surface
// modules (embedded web console, OpenAPI export, ...).
//
// Modules are supplied by the application through serv.OptionSetModule; the
// service only knows the Module interface below. Nothing in this package
// imports a concrete module, keeping slim binaries free of product surfaces
// they never asked for. Adding a surface means writing a module and passing
// it in — the service itself never changes.
package module

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"go.uber.org/zap"
)

// Context is the engine-neutral input every module receives at mount time.
//
// Settings carries the module's own configuration block, decoded as generic
// values (yaml/json friendly), read from the top-level `modules:` config
// section under the module's Name(). Each module documents and parses the
// keys it understands, so adding a module never extends this struct. A nil
// section or a false "enabled" value means the module should stay inert:
// mount successfully but contribute nothing.
type Context struct {
	Settings map[string]any

	// Engine is the initialized GraphJin core. It is nil when the service
	// started without a database (dev mode); modules must tolerate that.
	Engine *core.GraphJin

	// Namespace is the configured service namespace (nil = none).
	Namespace *string

	// Logger is the service logger.
	Logger *zap.SugaredLogger
}

// Enabled reports whether the module's settings allow it to activate.
func (c Context) Enabled() bool {
	if c.Settings == nil {
		return false
	}
	v, ok := c.Settings["enabled"]
	if !ok || v == nil {
		return true // section present without an explicit key defaults to on
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1" || strings.EqualFold(t, "yes") || strings.EqualFold(t, "on")
	default:
		return fmt.Sprintf("%v", v) == "true"
	}
}

// String returns a string setting from the module's section.
func (c Context) String(key string) (string, bool) {
	if c.Settings == nil {
		return "", false
	}
	v, ok := c.Settings[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		s = fmt.Sprintf("%v", v)
	}
	return s, true
}

// Route is one HTTP endpoint a module contributes to the service mux.
type Route struct {
	Path    string       // exact mux pattern ("/" is allowed as catch-all)
	Handler http.Handler // may be nil to skip mounting
}

// DevLink is one development-mode URL a module advertises at startup.
type DevLink struct {
	Label string // e.g. "Web UI"
	Path  string // e.g. "/"
}

// Contribution is what a mounted module hands back to the service.
type Contribution struct {
	// Routes are mounted when the module is active.
	Routes []Route

	// DevLinks are printed at startup in development mode.
	DevLinks []DevLink
}

// Module is an optional product surface plugged into the service.
type Module interface {
	// Name matches the key this module reads under the `modules:` config
	// section.
	Name() string

	// Mount resolves settings and builds the contribution. Implementations
	// must stay inert (no routes, no errors) when disabled, mirroring how
	// database adapters fail loudly only when actually opened.
	Mount(ctx Context) (Contribution, error)
}
