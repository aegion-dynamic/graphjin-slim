// Package format is the output seam between GraphJin execution results
// and wire representations.
//
// Formatters register themselves (usually from an init function in their
// own module); callers look up whichever format a request negotiates.
// The default JSON formatter reproduces the historical response bytes
// exactly, so adopting the registry never changes output on its own.
package format

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// Payload carries what a formatter renders. Deliberately decoupled from
// engine.Result to keep this package importable from anywhere without
// cycles; engines map their results in.
type Payload struct {
	// Data is the response document as produced by execution.
	Data json.RawMessage
}

// Formatter renders an execution payload into a wire representation,
// including whatever envelope conventions it owns (data/errors wrapping
// belong to the format, not to the engine).
type Formatter interface {
	// Name matches configuration and content negotiation ("json").
	Name() string

	// Format writes the complete wire representation for the payload.
	Format(w io.Writer, p Payload) error
}

// JSON is the built-in formatter: it writes the data document verbatim,
// matching GraphJin's historical byte-for-byte output.
type JSON struct{}

func (JSON) Name() string { return "json" }

func (JSON) Format(w io.Writer, p Payload) error {
	_, err := w.Write(p.Data)
	return err
}

func init() { Register(JSON{}) }

var (
	mu         sync.RWMutex
	formatters = map[string]Formatter{}
)

// Register installs a formatter. Panics on duplicate names: registering
// twice is always a programming error, matching database/sql conventions.
func Register(f Formatter) {
	if f == nil || f.Name() == "" {
		panic("format: Register called with nil formatter or empty name")
	}
	mu.Lock()
	defer mu.Unlock()
	name := f.Name()
	if _, dup := formatters[name]; dup {
		panic("format: Register called twice for formatter " + name)
	}
	formatters[name] = f
}

// Lookup resolves a format name to its formatter. The built-in JSON
// formatter always resolves; anything else must have been registered by
// importing its module.
func Lookup(name string) (Formatter, error) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := formatters[name]
	if !ok {
		return nil, fmt.Errorf("format: no formatter registered for %q (available: %s)",
			name, strings.Join(Names(), ", "))
	}
	return f, nil
}

// Names lists registered formatters, sorted for stable output.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(formatters))
	for n := range formatters {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
