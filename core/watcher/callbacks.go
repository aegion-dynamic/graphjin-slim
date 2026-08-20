// Package engine contains core runtime orchestration modules.
package watcher

import (
	"fmt"
	"sync"
)

// SchemaCallbacks manages schema-change observers without exposing engine
// state to callers or adapters.
type SchemaCallbacks struct {
	mu        sync.RWMutex
	callbacks []func(string, string)
}

// Register adds an observer. Observers receive the database name and schema
// hash after startup and after subsequent schema changes.
func (c *SchemaCallbacks) Register(fn func(string, string)) {
	if c == nil || fn == nil {
		return
	}
	c.mu.Lock()
	c.callbacks = append(c.callbacks, fn)
	c.mu.Unlock()
}

// Fire invokes a snapshot of the observers asynchronously.
func (c *SchemaCallbacks) Fire(database, hash string) {
	if c == nil {
		return
	}
	c.mu.RLock()
	callbacks := append([]func(string, string){}, c.callbacks...)
	c.mu.RUnlock()
	for _, fn := range callbacks {
		fn := fn
		go func() {
			defer func() { _ = recover() }()
			fn(database, hash)
		}()
	}
}

// String describes the registered observer count for diagnostics.
func (c *SchemaCallbacks) String() string {
	if c == nil {
		return "schema callbacks: 0"
	}
	c.mu.RLock()
	count := len(c.callbacks)
	c.mu.RUnlock()
	return fmt.Sprintf("schema callbacks: %d", count)
}
