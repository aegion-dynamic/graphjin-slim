package engine

import storage "github.com/aegion-dynamic/graphjin-slim/core/v3/storage"

// FS is the filesystem abstraction used by core.
type FS = storage.FS

// NewOsFS creates a filesystem rooted at basePath.
func NewOsFS(basePath string) FS { return storage.NewOsFS(basePath) }
