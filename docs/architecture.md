# Architecture

GraphJin Slim is a two-module Go workspace:

```text
core/   GraphQL compiler and runtime library
serv/   Embeddable HTTP service built on core
```

The module paths are intentionally stable:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3
github.com/aegion-dynamic/graphjin-slim/serv/v3
```

## Request Flow

```text
HTTP request
    |
    v
serv HTTP handlers
    |
    v
core GraphJin runtime
    |
    +--> query language (langadapter seam; graphql built in)
    +--> query representation (qcode IR)
    +--> schema metadata
    +--> SQL compiler
    +--> database execution
    +--> response encoding (format seam; json built in)
```

The service owns transport concerns. The core owns GraphQL behavior and SQL
generation. Callers embedding the library should normally interact with
`serv.HttpService` or `core.GraphJin`, not with internal packages.

## Current Modules

### `core`

The public compiler and runtime facade. It exposes the stable `GraphJin` interface,
public configuration, and execution results.

### `core/engine`

The core query execution orchestrator. It manages request execution states (`gstate`),
prepared statement caching, parameter binding, and multi-database query dispatching.

### `core/dbjoin`

Cross-database distributed joins and multi-database result merging. Builds child
GraphQL subqueries filtered by parent foreign keys and stitches nested JSON results.

### `core/watcher`

Background database schema polling, thread-safe schema change callbacks, and
engine shutdown lifecycle management.

### `core/runtime`

Execution resilience, exponential backoff retry policies, field-level AES-GCM
encryption, and error repair diagnostics.

### `core/graph`

GraphQL lexing, parsing, and schema syntax.

### `core/qcode`

The normalized query compiler, IR representation, and AST validation layer
between GraphQL and SQL generation.

### `core/sqlgen`

SQL query and mutation compilation.

### `core/dialect`

Database-specific SQL rendering for Postgres and SQLite.

### `core/langadapter`

The input seam: query languages register factories globally, instances
bind per-database to a qcode compiler, and everything lowers into the
shared qcode IR. Registry semantics mirror `core/dbadapter`. The
built-in graphql language lives in the engine until its frontend is
fully extracted; optional capabilities (`FastInfoer`,
`SchemaDescriber`, `SubqueryBuilder`) let languages own parsing
metadata, schema description, and cross-DB fan-out encoding.

### `core/format`

The output seam: wire formats register by name and own their envelope.
The built-in JSON formatter reproduces historical bytes exactly. serv
resolves formatters per request via `?format=` then Accept headers.

### `core/sdata`

Database schema metadata, tables, columns, functions, relationships, and
snapshots.



### `serv/database`

Database configuration and connection opening for Postgres and SQLite.

### `serv/cache`

Response caching, dependency invalidation, Redis, in-memory caching, stale
while revalidate, and cache metrics.

### `serv/http`

The route registration contract and HTTP route definitions. The root service
package adapts its private handlers into this module.

## Dependency Direction

The intended dependency direction is:

```text
application
    |
    +--> serv
             |
             +--> serv/http
             +--> serv/cache
             +--> serv/database
             +--> core

application (optional database engines, via blank import into core/dbadapter)
    |
    +--> postgres  pgx/v5 connection adapter
    +--> sqlite    SQLCipher-backed driver

application (optional product surfaces, via serv.OptionSetModule)
    |
    +--> webui     embedded React console
    +--> openapi   spec generator over core/v3/{qcode,sdata,schema,allow}
```

Internal compiler packages should not import the service package. Transport
packages should not own compiler behavior. The root service package is the
composition point where configuration, database, cache, HTTP, and core are
assembled. The webui and openapi modules are opt-in: the service only knows
the neutral seam (`serv/module`, shaped like `core/dbadapter`), so slim
binaries are unchanged unless an application passes modules via
`serv.OptionSetModule`. The service never names a module; each module parses
its own section under the top-level `modules:` config key.

## Refactoring Direction

The remaining large root packages are being extracted around behavior rather
than split by arbitrary file size:

```text
serv/config       configuration loading and validation
serv/lifecycle    startup, shutdown, and reload
core/engine       engine lifecycle and database contexts
core/schema       discovery and schema snapshots
core/compiler     GraphQL-to-plan orchestration
core/runtime      execution, results, tracing, and limits
```

Each extraction must preserve the public `core/v3` and `serv/v3` interfaces.
Compatibility aliases belong only at those public facades; new implementation
code should depend on narrower module interfaces.
