# Core

`core` is the GraphJin compiler and runtime library.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3
```

It turns GraphQL operations into database queries, executes them, and returns
structured results. The package is intended to be embedded by Go applications;
it does not start an HTTP server and does not own application authentication or
application routing.

## Public Entry Points

The stable public surface is in the root `core` package.

Important types and functions include:

```go
core.NewGraphJin
core.NewGraphJinWithFS
core.GraphJin
core.Config
core.Option
core.Result
```

The typical flow is:

```go
engine, err := core.NewGraphJin(conf, db)
if err != nil {
	return err
}
defer engine.Close()

result, err := engine.GraphQL(ctx, query, variables, requestConfig)
```

Use `core.NewTestGraphJin` from `testing.go` for schema-backed tests that do
not need a live database.

## Responsibilities

Core owns:

- GraphQL parsing and validation
- Schema discovery and relationship modeling
- Query, mutation, and subscription planning
- SQL generation for Postgres and SQLite
- Role and allow-list enforcement
- Result encoding and pagination
- Runtime schema reloads
- Response-cache integration hooks
- Filesystem virtual tables
- Configured OpenAPI source integration

Core does not own:

- HTTP routing
- Server startup and shutdown
- Application authentication providers
- Application-specific authorization middleware
- Redis connection lifecycle
- CLI commands

## Internal Packages

The internal packages are implementation modules. Applications should not
import them directly.

| Package | Responsibility |
| --- | --- |
| `internal/graph` | GraphQL lexer, parser, and syntax schema |
| `internal/qcode` | Validated GraphQL query representation |
| `internal/psql` | Query and mutation SQL generation |
| `internal/dialect` | Postgres and SQLite rendering differences |
| `internal/sdata` | Database metadata and schema relationships |
| `internal/introspection` | Database metadata queries |
| `internal/jsn` | Specialized JSON scanning and mutation helpers |
| `internal/allow` | Saved-query allow-list storage and matching |
| `internal/valid` | Shared validation helpers |
| `internal/util` | Small compiler data structures and graph utilities |
| `openapi` | OpenAPI source loading and calling |
| `fstable` | Local and object-storage virtual-table adapters |

## Request Pipeline

```text
GraphQL text
    |
    v
internal/graph
    |
    v
internal/qcode
    |
    v
core schema metadata
    |
    v
internal/psql + internal/dialect
    |
    v
SQL and bound arguments
    |
    v
database/sql execution
    |
    v
core.Result
```

Schema discovery is separate from query compilation conceptually: the compiler
consumes schema metadata, while discovery creates and refreshes that metadata.

## Database Support

The slim runtime supports:

- Postgres
- SQLite

The dialect and introspection code should preserve that boundary. Removed
database implementations and tests should not be reintroduced through generic
fallback code.

## Testing

Run the core suite from the workspace root:

```bash
go test ./core/...
go vet ./core/...
```

Use the lowest-level package tests for compiler behavior and root-package tests
for public runtime behavior. Prefer synthetic schema fixtures and SQLite over
requiring an external database.

## Refactoring Rules

- Keep `core/v3` stable for consumers.
- Keep internal packages below the public root package.
- Put schema discovery behind a schema interface instead of letting query code
  discover tables directly.
- Keep GraphQL parsing out of runtime execution.
- Move behavior into deep modules with small interfaces rather than creating
  forwarding packages.
