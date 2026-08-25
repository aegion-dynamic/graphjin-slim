# graphjin-slim

Slim fork of [dosco/graphjin](https://github.com/dosco/graphjin) focused on the data layer: a high-performance GraphQL to Postgres compiler and Go service. This keeps the core that production services embed as a library while trimming standalone surfaces that are not part of the data path.

[![Apache 2.0](https://img.shields.io/github/license/aegion-dynamic/graphjin-slim.svg?style=for-the-badge)](LICENSE)
[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=for-the-badge&logo=go)](https://pkg.go.dev/github.com/aegion-dynamic/graphjin-slim/core/v3)

GraphJin is a compiler and runtime that turns GraphQL into a single optimized SQL query. No resolvers, no ORM, no N+1. It discovers your schema, exposes relationships as nested fields, and compiles every request directly to Postgres. This fork keeps that compiler and the embeddable Go service, without the extra surfaces that live in upstream.

Upstream remains the full project with its broader feature set. This repo tracks upstream and removes what the slim postgres embed does not need.

## What this fork keeps

- `core/` - GraphQL parsing, schema discovery (`sdata`), IR (`qcode`), SQL generation (`psql`, postgres and sqlite dialects), allow list, and the stable public API in `core/api.go`
- `serv/` - `NewGraphJinService`, `GraphQL` and `REST` handlers, `GetDB`, `Attach`, database initialization, health, caching, and config loading
- Sub-packages: `core/graph`, `core/jsn`, `core/sdata`, `core/qcode`, `core/psql`, `core/dialect` (postgres and sqlite), `core/allow`, `core/valid`, `core/schema`, `core/introspection`, `core/util`
- Database engines: Postgres and SQLite as opt-in adapter modules (see `core/dbadapter`); optional at-rest encryption via SQLCipher
- Sources: database, file, api
- File backends: local and s3

## Optional modules

Everything beyond the core data path ships as its own workspace module. The
slim build never links them; an application imports exactly what it wants.
Two seams make this work, one per layer:

**Database engines**, registered into `core/dbadapter` by a blank import;
the service resolves whichever engine a config names via `database.type`.
Nothing in core or serv links a driver.

| Module | Import path | Provides |
|---|---|---|
| `postgres/` | `github.com/aegion-dynamic/graphjin-slim/postgres/v3` | Postgres adapter over pgx/v5 |
| `sqlite/` | `github.com/aegion-dynamic/graphjin-slim/sqlite/v3` | SQLite adapter (SQLCipher-backed; optional at-rest encryption) |

```go
import (
    _ "github.com/aegion-dynamic/graphjin-slim/postgres/v3" // or sqlite/v3
)
```

**Product surfaces**, passed to the service through options; the service
knows only the `serv/module` seam and never names a module. Each module
reads its own section under the top-level `modules:` config key.

| Module | Import path | Provides |
|---|---|---|
| `webui/` | `github.com/aegion-dynamic/graphjin-slim/webui/v3` | Embedded React console (`webui.Module`) |
| `openapi/` | `github.com/aegion-dynamic/graphjin-slim/openapi/v3` | OpenAPI 3.0 spec generator for saved queries (`openapi.Module`) |

```go
gjs, err := serv.NewGraphJinService(conf,
    serv.OptionSetModule(webui.Module()),
    serv.OptionSetModule(openapi.Module(openapi.Config{Title: "My API"})),
)
```

Setting `modules.openapi.specs_dir` writes the spec to disk at startup for
SDK codegen pipelines.

## What was removed

- Role and identity system: RBAC/ABAC role models, presets, role-based column/table blocking, and `$user_id` contextual injections have been removed in favor of an un-opinionated, pure GraphQL-to-SQL compiler where authorization is host-owned.
- `wasm/` - WASM build for NodeJS
- `website/` - Hugo docs site
- `benchmark/` - standalone bench scripts
- IDE plugin configs: `.claude-plugin/`, `.codex/`, `.cursor/`
- Other database drivers and standalone service surfaces. Configuration accepts Postgres and SQLite.

## How it works

1. Connects to Postgres and reads your schema automatically
2. Discovers relationships from foreign keys
3. Exposes them as nested GraphQL fields
4. Compiles every request to a single optimized SQL query

## Usage as a Go library

```bash
go get github.com/aegion-dynamic/graphjin-slim/core/v3
```

```go
import (
    "github.com/aegion-dynamic/graphjin-slim/core/v3"
    "github.com/aegion-dynamic/graphjin-slim/serv/v3"

    _ "github.com/aegion-dynamic/graphjin-slim/postgres/v3" // engine adapter (or sqlite/v3)
)

gjs, err := serv.NewGraphJinService(conf, servOpts...)
if err != nil {
    log.Fatal(err)
}

router.Handle("/graphql", gjs.GraphQL(authHandler))
router.Handle("/rest/*", gjs.REST(nil))
db := gjs.GetDB()
```

## Package Documentation

- [Architecture](docs/architecture.md)
- [Core package](core/README.md)
- [Service package](serv/README.md)
- [Development guide](docs/development.md)

Configuration is loaded through `serv.ReadInConfig` or `serv.NewConfig`. The
supported database types are Postgres and SQLite. See [serv/README.md](serv/README.md)
for configuration examples and service ownership.

## Development

```bash
go vet ./core/... ./serv/...
go test ./core/... ./serv/...
```

The stable public interfaces are documented in [core/README.md](core/README.md)
and [serv/README.md](serv/README.md). Implementation packages under
`core/internal` are not public extension points.

## License

[Apache 2.0](LICENSE), same as upstream.
