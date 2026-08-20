# psql

`psql` is the SQL compiler that compiles validated `qcode.QCode` intermediate representations into optimized SQL queries for PostgreSQL and SQLite.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/psql
```

## Overview

The SQL compiler generates a single, flat, deterministic SQL query for arbitrary GraphQL operations. It handles nested relationships, JSON projection, pagination (cursor-based and limit/offset), ordering, filtering, aggregation, window functions, and mutation CTE pipelines.

## Capabilities

- **Single-Query Compilation**: Compiles complex, deeply nested GraphQL selections into single SQL statements without N+1 query overhead.
- **Relational Joins**: Emits dialect-optimized joins (`LATERAL` in Postgres, correlated subqueries in SQLite).
- **Mutations & Pipelines**: Generates CTEs (Common Table Expressions) for multi-table inserts, updates, upserts (`on_conflict`), and deletes.
- **Aggregation & Analytics**: Translates GraphQL aggregates (`count`, `sum`, `avg`, etc.) and window functions (`@running`, `@moving`, `@rank`) directly into SQL expressions.

## Key Entry Points

```go
type Compiler struct { ... }

func NewCompiler(conf Config) *Compiler
func (co *Compiler) Compile(w io.Writer, qc *qcode.QCode) (Metadata, error)
func (co *Compiler) CompileQuery(w io.Writer, qc *qcode.QCode) (Metadata, error)
func (co *Compiler) CompileMutation(w io.Writer, qc *qcode.QCode) (Metadata, error)
```
