# engine

`engine` is the core execution orchestrator for GraphJin Slim.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/engine
```

## Overview

The `engine` package coordinates the runtime lifecycle, multi-database routing, execution state (`gstate`), cross-database query joins (`dbjoin`), error repair diagnostics, and schema change notifications.

## Responsibilities

- **Request State Management (`gstate`)**: Tracks query compilation, argument binding, connection retrieval, and SQL execution for each incoming GraphQL operation.
- **Multi-Database Coordination**: Manages named database contexts (`dbContext`), schema discovery across multiple databases, and cross-database distributed joins (`dbjoin.go`).
- **Runtime Resilience**: Built-in exponential backoff retries for transient lock errors (`retry.go`) and query diagnostic repairs (`repair.go`).
- **Observability & Tracing**: Distributed tracing hooks and OpenTelemetry span propagation (`trace.go`).
- **Schema Lifecycle**: Background schema pollers (`watcher.go`) and atomic runtime reload callbacks (`callbacks.go`).

## Key Entry Points

```go
type Engine struct { ... }

func New(conf *Config, db *sql.DB, options ...Option) (*Engine, error)
func (g *Engine) GraphQL(ctx context.Context, query string, vars json.RawMessage, rc *RequestConfig) (*Result, error)
func (g *Engine) GraphQLTx(ctx context.Context, tx *sql.Tx, query string, vars json.RawMessage, rc *RequestConfig) (*Result, error)
func (g *Engine) ReloadSchema() error
func (g *Engine) Close()
```
