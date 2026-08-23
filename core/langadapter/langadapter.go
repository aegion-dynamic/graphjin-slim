// Package langadapter is the input seam between GraphJin and query
// languages.
//
// Frontends register themselves (usually from an init function in their
// own module); the engine looks up whichever language a configuration
// names. Nothing in this package parses any particular syntax or binds
// a concrete compiler type: frontends lower into the shared qcode IR,
// which is the only contract the rest of core understands. Registry
// semantics mirror core/dbadapter by design. With the frontend living
// in its own module, an application selects languages by blank-
// importing them — exactly like database engines.
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

// DefaultLanguageName is the well-known name of the historically
// built-in language. Engines fall back to it when a request does not
// name one; referencing the string keeps core free of frontend imports.
const DefaultLanguageName = "graphql"

// CompileOptions carries per-request compilation settings.
type CompileOptions struct {
	// Namespace groups saved queries and schema overlays within a
	// single engine instance.
	Namespace string
}

// CompileConfig is the engine-neutral construction configuration every
// compiler factory accepts. Frontends map it onto their internal
// compiler configuration; adding an engine never extends this struct.
type CompileConfig struct {
	Vars            map[string]string
	TConfig         map[string]qcode.TConfig
	DefaultLimit    int
	AnalyticsMode   bool
	DisableAgg      bool
	DisableFuncs    bool
	EnableCamelcase bool
	DBSchema        string

	// EnableCacheTracking asks the frontend to inject primary-key
	// tracking fields for cache row identification.
	EnableCacheTracking bool
}

// Info identifies an operation cheaply, without full compilation.
// Used for allow-list keying, APQ caching, and logging; languages that
// can extract it quickly should implement FastInfoer.
type Info struct {
	Operation string // language-defined operation kind, e.g. "query"
	Name      string // operation name, may be empty
}

// Language turns query source text into the qcode IR. Instances are
// bound to one database schema: factories construct one per database.
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

// SubqueryBuilder constructs child work for cross-database joins in the
// language's own medium: serialized source text, URLs, or any other
// encoding the same Language can consume on the remote side. Without it,
// joins fall back to the built-in GraphQL text protocol.
type SubqueryBuilder interface {
	BuildChildQuery(sel *qcode.Select, selects []qcode.Select,
		fkCol sdata.DBColumn, parentID []byte) ([]byte, error)
}

// Parameter describes one named input variable of a saved query.
type Parameter struct {
	Name     string
	Required bool
	Type     string // language-specific scalar label
}

// ParameterDescriber exposes the input variables of a query source so
// documentation generators never need a parser.
type ParameterDescriber interface {
	QueryParameters(query []byte) ([]Parameter, error)
}

// DescribeOptions carries everything schema description needs, expressed
// in engine-neutral terms.
type DescribeOptions struct {
	Schemas    []*sdata.DBSchema
	CamelCase  bool
	DisableAgg bool
}

// DescribeFactory serves protocol-specific schema metadata (e.g.
// introspection documents) computed purely from its inputs. Engines
// delegate to whichever registered language describes schemas.
type DescribeFactory interface {
	Descriptor
	DescribeSchema(opts DescribeOptions) (json.RawMessage, error)
}

// SchemaParser turns frontend-authored schema definition text into
// neutral DB metadata. Engines and offline tooling use it instead of
// importing any frontend.
type SchemaParser interface {
	Descriptor
	// ParseSchemaSDL parses frontend schema-definition text. An empty
	// dbType falls back to the type declared in the text itself.
	ParseSchemaSDL(b []byte, dbType string, blocklist []string) (*sdata.DBInfo, error)
}

// Descriptor is the discovery record for a query language. Frontends
// register one at init time.
type Descriptor interface {
	// Name matches configuration, routes, and discovery output.
	Name() string
}

// CompilerFactory constructs per-database Language instances bound to a
// specific database schema. Registered globally at init time.
type CompilerFactory interface {
	Descriptor

	// NewCompiler binds a language to the given schema under the given
	// neutral configuration.
	NewCompiler(schema *sdata.DBSchema, cfg CompileConfig) (Language, error)
}

var (
	mu          sync.RWMutex
	descriptors = map[string]Descriptor{}
)

// // Register installs a language descriptor. Panics on duplicate names:
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

// Lookup resolves a language name to its descriptor. Capabilities are
// taken by type-asserting the result:
//
//	d, err := langadapter.Lookup("graphql")
//	cf, ok := d.(langadapter.CompilerFactory)
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
