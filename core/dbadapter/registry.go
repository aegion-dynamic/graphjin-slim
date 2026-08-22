// Package dbadapter is the seam between GraphJin and database engines.
//
// Engines register themselves (usually from an init function in their own
// module); the service layer looks up whichever adapter a configuration
// names. Nothing in this package imports a concrete driver, keeping core
// free of C dependencies and engine-specific code.
package dbadapter

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// SourceConfig is the engine-neutral input every adapter receives.
//
// Settings carries the engine's own configuration block, decoded as generic
// values (yaml/json friendly). Each adapter documents and parses the keys it
// understands, so adding an engine never extends this struct.
//
// Flat carries the legacy top-level connection fields for backward
// compatibility with configs written before engine-keyed sections existed.
// When the same logical field appears in both places, Settings wins.
type SourceConfig struct {
	Type     string         // adapter name this source resolved to
	Settings map[string]any // engine-specific section (may be nil)

	// Legacy flat fields (pre-engine-section configs).
	Flat FlatFields

	// GetFile optionally loads file contents referenced by configuration
	// (e.g. TLS certificates). nil means "files must be inline".
	GetFile func(path string) ([]byte, error)
}

// FlatFields mirrors the original flat `database:` layout.
type FlatFields struct {
	ConnString    string
	Path          string
	EncryptionKey string
	Host          string
	Port          uint16
	User          string
	Password      string
	DBName        string
	Schema        string
	AppName       string
	OpenDBName    bool
	EnableTLS     bool
	ServerName    string
	ServerCert    string
}

// Adapter opens database connections for one engine.
type Adapter interface {
	// Name matches the `type` string in configuration.
	Name() string

	// Open connects and pings. Implementations must fail loudly and early
	// when required configuration or support is missing.
	Open(ctx context.Context, cfg SourceConfig) (*sql.DB, error)
}

var (
	mu       sync.RWMutex
	adapters = map[string]Adapter{}
)

// Register installs an adapter. Panics on duplicate names: registering twice
// is always a programming error, matching database/sql conventions.
func Register(a Adapter) {
	if a == nil || a.Name() == "" {
		panic("dbadapter: Register called with nil adapter or empty name")
	}
	mu.Lock()
	defer mu.Unlock()
	name := a.Name()
	if _, dup := adapters[name]; dup {
		panic("dbadapter: Register called twice for adapter " + name)
	}
	adapters[name] = a
}

// Lookup resolves a configured type to its adapter.
func Lookup(name string) (Adapter, error) {
	mu.RLock()
	defer mu.RUnlock()
	a, ok := adapters[name]
	if !ok {
		return nil, fmt.Errorf("dbadapter: no adapter registered for %q (available: %s); add the corresponding blank import to your application",
			name, strings.Join(Names(), ", "))
	}
	return a, nil
}

// Names lists registered adapters, sorted for stable output.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(adapters))
	for n := range adapters {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
