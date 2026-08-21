# Service

`serv` is the embeddable HTTP service built on top of the GraphJin core
compiler.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/serv/v3
```

The service package connects configuration, database adapters, caches, HTTP
handlers, and the `core.GraphJin` runtime. It can be embedded in an existing
Go HTTP application or used to run GraphJin's own server lifecycle.

## Public Entry Points

The root `serv` package is the compatibility facade.

Important types and functions include:

```go
serv.NewGraphJinService
serv.HttpService
serv.Config
serv.NewConfig
serv.ReadInConfig
serv.HandlerFunc
```

An embedded application normally creates a service and mounts its handlers:

```go
service, err := serv.NewGraphJinService(conf)
if err != nil {
	return err
}
defer service.Close()

router.Handle("/graphql", service.GraphQL(authHandler))
router.Handle("/rest/", service.REST(nil))
```

The service owns database connections and cache resources created during
construction. Call `Close` when the embedding application shuts down.

## Responsibilities

Service owns:

- Configuration loading and environment overrides
- Postgres and SQLite connection setup
- GraphQL and REST HTTP handlers
- Health endpoint registration
- Response-cache selection
- Server startup and graceful shutdown
- File and object-storage service integration
- Service-level logging and telemetry setup

Core owns the compiler, schema model, query execution, and GraphJin runtime.
The service should compose core; it should not duplicate compiler behavior.

## Package Layout

### `serv/database`

Database configuration and connection opening for Postgres and SQLite.

The package owns connection strings, pool configuration, Postgres TLS setup,
database pinging, and retry behavior. The root service package keeps wrappers
for compatibility with existing consumers.

### `serv/cache`

Response-cache behavior and adapters.

```text
cache.Config
cache.ResponseCache
cache.MemoryCache
cache.RedisCache
```

The package owns TTL policy, dependency invalidation, stale-while-revalidate,
and cache metrics. The memory and Redis implementations share the same cache
interface.

### `serv/http`

Route registration and the minimal mux contract.

The root service package supplies GraphQL, REST, and Web UI handlers. The HTTP
package owns route paths and registration, but does not import the service
package or know how handlers are implemented.

### Root `serv` package

The root package is the composition point. It currently contains the service
state, configuration loader, handler implementation, startup lifecycle, and
compatibility wrappers. These responsibilities are being extracted gradually;
the root package remains the stable import path for users.

## Request Flow

```text
application router
    |
    v
serv/http route registration
    |
    v
serv GraphQL or REST handler
    |
    v
core.GraphJin
    |
    +--> database/sql connection
    +--> cache.Cache
    +--> core schema and compiler
```

Authentication is intentionally supplied by the embedding application through
`HandlerFunc`. The service does not provide an application-specific identity
provider.

## Configuration

Configuration can be loaded from a file or constructed from a string:

```go
conf, err := serv.ReadInConfig("./config.yml")
conf, err := serv.NewConfig(yamlText, "yaml")
```

The supported database types are:

```yaml
database:
  type: postgres
```

or:

```yaml
database:
  type: sqlite
  path: ./app.db
```

SQLite sources can be encrypted at rest by adding an `encryption_key`
(SQLCipher is applied per connection; requires a SQLCipher-linked build —
see [serv/database/README.md](database/README.md)):

```yaml
database:
  type: sqlite
  path: ./app.db
  encryption_key: ${GJ_DB_KEY}   # supply via environment, not in the file
```

Environment overrides use the `GJ_` prefix. Configuration names, defaults, and
the generated `config.schema.json` are part of the service's compatibility
surface.

## Testing

Run the service suite from the workspace root:

```bash
go test ./serv/...
go vet ./serv/...
```

Cache tests live with the cache package. Database behavior should use SQLite or
fake SQL drivers where possible. Service tests should test handler behavior
through the public service interface rather than reaching into private service
state.

## Refactoring Rules

- Keep `serv/v3` stable for consumers.
- Keep `serv` as the composition point, not a dumping ground for adapters.
- Put backend-specific behavior in `database`, `cache`, or `storage` packages.
- Keep transport registration independent from service state.
- Do not make implementation packages import the root `serv` package.
- Prefer one deep interface with multiple adapters over exposing backend details.
