# dialect

`dialect` defines the SQL dialect abstraction and engine-specific query rendering implementations for PostgreSQL and SQLite.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/dialect
```

## Overview

The GraphJin compiler generates single-query SQL representations for nested GraphQL queries. Differences in JSON aggregation, lateral joins, window frame functions, and upsert clauses are delegated to database-specific dialect implementations.

## Supported Dialects

- **PostgreSQL (`PostgresDialect`)**:
  - Leverages `LATERAL` joins for efficient child relationship traversal.
  - Generates `json_build_object` and `json_agg` for structured JSON output.
  - Native support for geometric / PostGIS types and window functions.
- **SQLite (`SQLiteDialect`)**:
  - Compiles nested relationships using correlated subqueries and CTEs.
  - Uses `json_object` and `json_group_array` for JSON rendering.
  - Implements SQLite-compatible conflict handling and lock-safe mutation paths.

## Key Interface

```go
type Dialect interface {
    RenderTable(ctx Context, ti *sdata.DBTable, alias string)
    RenderColumn(ctx Context, col *sdata.DBColumn)
    RenderJSONNullField(ctx Context, fieldName string)
    RenderWindow(ctx Context, f *qcode.Field)
    RenderChildValue(ctx Context, sel *qcode.Select, renderChild func())
    ...
}
```
