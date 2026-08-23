// Package langadapter is the input seam between GraphJin and query
// languages.
//
// Languages register themselves (usually from an init function in their
// own module); the engine looks up whichever language a request names.
// Nothing in this package parses any particular syntax: every language
// compiles into the shared qcode IR, which is the only contract the rest
// of core understands. This mirrors core/dbadapter on the database side;
// registry semantics are identical by design.
package langadapter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// CompileOptions carries per-request compilation settings.
type CompileOptions struct {
	// Namespace groups saved queries and schema overlays within a
	// single engine instance.
	Namespace string
}

// Info identifies an operation cheaply, without full compilation.
// Used for allow-list keying, APQ caching, and logging; languages that
// can extract it quickly should implement FastInfoer.
type Info struct {
	Operation string // language-defined operation kind, e.g. "query"
	Name      string // operation name, may be empty
}

// Language turns query source text into the qcode IR. Instances are
// bound to one database schema: construct one per database context.
type Language interface {
	// Name matches configuration and route registration ("graphql").
	Name() string

	// Compile parses source text and lowers it into the IR, validated
	// against the database schema this instance was constructed with.
	Compile(query []byte, vars map[string]json.RawMessage, opts CompileOptions) (*qcode.QCode, error)
}

// FastInfoer extracts Info without full compilation. The engine and the
// allow-list use this instead of importing any parser directly.
type FastInfoer interface {
	FastInfo(query []byte) (Info, error)
}

// SchemaDescriber serves protocol-specific schema metadata, such as
// GraphQL introspection documents. Engines delegate to it when the
// incoming request asks for schema description instead of data.
type SchemaDescriber interface {
	// DescribeSchema returns a self-contained metadata document for the
	// databases visible to the engine. ns may be empty for the default.
	DescribeSchema(ns string) (json.RawMessage, error)
}

// SubqueryBuilder constructs child work for cross-database joins in the
// language's own medium: serialized source text, URLs, or any other
// encoding the same Language can consume on the remote side. Without it,
// joins fall back to the built-in GraphQL text protocol.
type SubqueryBuilder interface {
	BuildChildQuery(sel *qcode.Select, selects []qcode.Select,
		fkCol sdata.DBColumn, parentID []byte) ([]byte, error)
}

var (
	mu          sync.RWMutex
	descriptors = map[string]Descriptor{}
)

// Descriptor is the discovery record for a query language. Frontends
// register one at init time; construction of per-database Language
// instances stays with the engine that owns the compilers.
type Descriptor interface {
	// Name matches configuration, routes, and discovery output.
	Name() string
}

// Register installs a language descriptor. Panics on duplicate names:
// registering twice is always a programming error, matching
// database/sql conventions.
func Register(d Descriptor) {
	if d == nil || d.Name() == "" {
		panic("langadapter: Register called with nil descriptor or empty name")
	}
	mu.Lock()
	defer mu.Unlock()
	name := d.Name()
	if _, dup := descriptors[name]; dup {
		panic("langadapter: Register called twice for language " + name)
	}
	descriptors[name] = d
}

// Lookup resolves a language name to its descriptor.
func Lookup(name string) (Descriptor, error) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := descriptors[name]
	if !ok {
		return nil, fmt.Errorf("langadapter: no language registered for %q (available: %s)",
			name, strings.Join(Names(), ", "))
	}
	return d, nil
}

// Names lists registered languages, sorted for stable output.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(descriptors))
	for n := range descriptors {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
