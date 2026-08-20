package engine

import (
	"sync"
	"sync/atomic"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/storage"
)

// FS is the filesystem abstraction used for config and schema files.
type FS = storage.FS

// DBContext is a placeholder for the eventual engine cut-over.
// Live per-database state still lives in package core.
type DBContext struct{}

// Engine is a placeholder for the eventual engine cut-over.
// Live engine state still lives in package core as graphjinEngine.
type Engine struct{}

// GraphJin is the outer manager shell used by lifecycle helpers.
type GraphJin struct {
	atomic.Value
	Lifecycle       *Lifecycle
	ReloadMu        sync.Mutex
	SchemaCallbacks SchemaCallbacks
}

// Option configures Engine (placeholder for cut-over).
type Option func(*Engine) error
