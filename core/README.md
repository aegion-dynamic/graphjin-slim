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

- GraphQL parsing and AST validation
- Schema discovery and relationship modeling (`sdata`)
- Pure query, mutation, and subscription compilation (`qcode`)
- SQL generation for Postgres and SQLite (`psql`, `dialect`)
- Allow-list enforcement for saved queries
- Result encoding and pagination
- Runtime schema discovery and reloads
- Response-cache integration hooks

Core does not own:

- HTTP routing
- Server startup and shutdown
- Application authentication / role evaluation (GraphJin Slim is host-agnostic)
- Application-specific authorization middleware
- Redis connection lifecycle
- CLI commands

## Sub-packages

The compiler sub-packages provide modular stages in the compilation pipeline:

| Package | Responsibility |
| --- | --- |
| `core/graph` | GraphQL lexer, parser, and syntax schema |
| `core/qcode` | Normalized GraphQL query compiler and intermediate representation (IR) |
| `core/psql` | SQL query and mutation compilation |
| `core/dialect` | Postgres and SQLite rendering differences |
| `core/sdata` | Database metadata and schema relationships |
| `core/introspection` | Database metadata discovery queries |
| `core/jsn` | Specialized JSON scanning and mutation helpers |
| `core/allow` | Saved-query allow-list storage and matching |
| `core/valid` | Shared validation helpers |
| `core/util` | Small compiler data structures and graph utilities |
| `core/schema` | GraphQL introspection schema generation |

## Request Pipeline

```text
GraphQL text
    |
    v
core/graph (lexer & parser)
    |
    v
core/qcode (normalized IR & validation)
    |
    v
core/sdata (schema metadata & relationships)
    |
    v
core/psql + core/dialect (SQL compiler)
    |
    v
SQL and bound arguments
    |
    v
database/sql execution
    |
    v
core.Result (JSON response)
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
