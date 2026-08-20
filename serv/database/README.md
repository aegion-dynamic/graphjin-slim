# serv/database

`database` manages database connection opening, driver initialization, connection pooling, and connection retry logic.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/serv/v3/database
```

## Overview

Provides standard connection openers for PostgreSQL (via `pgx/v5`) and SQLite (via `modernc.org/sqlite`), applying pooling limits, idle timeouts, and connection lifecycles.

## Key Functions

```go
func Open(opts Options) (*sql.DB, error)
func OpenCore(ctx context.Context, name string, dbConf core.DatabaseConfig) (*sql.DB, error)
```
