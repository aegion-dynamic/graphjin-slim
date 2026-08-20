# watcher

`watcher` manages background database schema polling, change notification callbacks, and shutdown lifecycle management for GraphJin.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/watcher
```

## Features

- **Schema Change Callbacks (`callbacks.go`)**: Thread-safe observer registration and asynchronous dispatch when database schema changes are discovered.
- **Engine Lifecycle (`lifecycle.go`)**: Idempotent shutdown controller for background goroutines and engine resources.
- **Poller Loop (`watcher.go`)**: Periodic background poller triggering catalog checks and schema reload routines.

## Key APIs

```go
type Lifecycle struct { ... }
func NewLifecycle() *Lifecycle

type SchemaCallbacks struct { ... }
func (c *SchemaCallbacks) Register(fn func(string, string))
func (c *SchemaCallbacks) Fire(database, hash string)

func Start(pollDuration time.Duration, lc *Lifecycle, check CheckFunc, reload ReloadFunc)
```
