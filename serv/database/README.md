# serv/database

`database` manages database connection opening, driver initialization, connection pooling, connection retry logic, and optional SQLCipher encryption for SQLite sources.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/serv/v3/database
```

## Overview

Provides standard connection openers for PostgreSQL (via `pgx/v5`) and SQLite
(via a patched `mattn/go-sqlite3` linked against the user's SQLCipher library),
applying pooling limits, idle timeouts, and connection lifecycles.

Both SQLite modes share one adapter:

- `encryption_key` unset → plain SQLite; files remain standard plaintext.
- `encryption_key` set   → SQLCipher: the key is applied via ConnectHook on
  every physical connection before any other statement, and the open is
  validated against `PRAGMA cipher_version`.

The driver registers under the name `"sqlite"`; Postgres uses pgx's `"pgx"`.

## Files

| File | Responsibility |
| --- | --- |
| `open.go` | The seam: `Open`, `OpenCore`, `Connection`, pool configuration |
| `config.go` | Adapter configuration types |
| `postgres.go` | pgx/v5 connection construction and TLS material loading |
| `sqlite.go` | mattn/go-sqlite3 wiring, `_pragma` handling, key application, cipher validation, DSN building |
| `warm.go` | `WarmPool`: pre-open and key N physical connections at startup |

## Encryption build requirements

The Go patch and encryption require linking your own SQLCipher library:

```bash
./scripts/apply-sqlite3-patch.sh          # patches mattn in the module cache

DIST=/path/to/sqlcipher-dist              # lib/libsqlcipher.so (+ libsqlite3.so symlink), include/sqlite3.h
CGO_CFLAGS="-I$DIST/include" \
CGO_LDFLAGS="-L$DIST/lib -Wl,-rpath,$DIST/lib" \
go build -tags libsqlite3 ./...
```

Without that tag the binary links vanilla SQLite: plaintext mode works, and
any config with `encryption_key` fails fast with rebuild guidance.

## Key Functions

```go
func Open(opts Options) (*sql.DB, error)
func OpenCore(ctx context.Context, name string, dbConf core.DatabaseConfig) (*sql.DB, error)
func WarmPool(db *sql.DB, n int) error
```
