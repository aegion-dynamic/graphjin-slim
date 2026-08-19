# graphjin-slim

Slim fork of [dosco/graphjin](https://github.com/dosco/graphjin) focused on the data layer: a high-performance GraphQL to Postgres compiler and Go service. This keeps the core that production services embed as a library while trimming standalone surfaces that are not part of the data path.

[![Apache 2.0](https://img.shields.io/github/license/aegion-dynamic/graphjin-slim.svg?style=for-the-badge)](LICENSE)
[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=for-the-badge&logo=go)](https://pkg.go.dev/github.com/aegion-dynamic/graphjin-slim/core/v3)

GraphJin is a compiler and runtime that turns GraphQL into a single optimized SQL query. No resolvers, no ORM, no N+1. It discovers your schema, exposes relationships as nested fields, and compiles every request directly to Postgres. This fork keeps that compiler and the embeddable Go service, without the extra surfaces that live in upstream.

Upstream remains the full project with its broader feature set. This repo tracks upstream and removes what the slim postgres embed does not need.

## What this fork keeps

- `core/` - GraphQL parsing, schema discovery (`sdata`), IR (`qcode`), SQL generation (`psql`, postgres and sqlite dialects), allow list, roles, and the stable public API in `core/api.go`
- `serv/` - `NewGraphJinService`, `GraphQL` and `REST` handlers, `GetDB`, `Attach`, database initialization, health, caching, and config loading
- `core/internal/*` - `graph`, `jsn`, `sdata`, `qcode`, `psql`, `dialect` (postgres and sqlite), `allow`, `valid`, and other internals
- Databases: postgres and sqlite
- Sources: database, file, api
- File backends: local and s3

## What was removed

- `wasm/` - WASM build for NodeJS, nothing in the data path imports it
- `website/` - Hugo docs site
- `benchmark/` - standalone bench scripts
- IDE plugin configs: `.claude-plugin/`, `.codex/`, `.cursor/`
- Other database drivers and standalone service surfaces are not part of the slim runtime. Configuration accepts Postgres and SQLite.

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
)

gjs, err := serv.NewGraphJinService(conf, servOpts...)
if err != nil {
    log.Fatal(err)
}

router.Handle("/graphql", gjs.GraphQL(authHandler))
router.Handle("/rest/*", gjs.REST(nil))
db := gjs.GetDB()
```

## Configuration

See [CONFIG.md](CONFIG.md) and [FEATURES.md](FEATURES.md) for the full config reference. The postgres path is configured with `database.type: postgres` and `database.schema`. Production uses an allow list of saved queries in `config/queries`.

## Development

```bash
go vet ./core/... ./serv/...
go test ./core/... ./serv/...
```

The stable public API is in `core/api.go`. See upstream docs for compiler details.

## License

[Apache 2.0](LICENSE), same as upstream.
