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

SQLCipher is compiled directly into the application from the committed
amalgamation (`sqlite3.c`), so no external SQLCipher library or patch step is
needed. Builds require a C toolchain plus OpenSSL 3 development files; the
resulting binary dynamically links the system `libcrypto.so.3` at runtime.

Local one-time setup (Debian/Ubuntu):

```bash
sudo apt-get install -y build-essential pkg-config libssl-dev
go build ./...
```

CI provisions these packages itself (see `.github/workflows/auto-release.yml`),
so releases never depend on a developer machine. To upgrade SQLCipher, move the
`cipher/` submodule to a new release commit and regenerate the amalgamation:

```bash
./gen.sh
```

## Key Functions

```go
func Open(opts Options) (*sql.DB, error)
func OpenCore(ctx context.Context, name string, dbConf core.DatabaseConfig) (*sql.DB, error)
func WarmPool(db *sql.DB, n int) error
```
