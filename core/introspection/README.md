# introspection

`introspection` executes database catalog queries to discover tables, columns, foreign keys, unique constraints, and functions from PostgreSQL and SQLite databases.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/introspection
```

## Overview

GraphJin automatically discovers your database structure at runtime by querying the database's information schema and system catalogs. `introspection` provides driver-specific SQL queries and parsers to build the raw `DBInfo` metadata used by the compiler.

## Features

- **PostgreSQL Discovery**: Reads tables, views, primary keys, composite foreign keys, check constraints, column types, and stored functions/procedures.
- **SQLite Discovery**: Introspects `sqlite_master`, `table_info`, and `foreign_key_list` pragmas.
- **Instrumentation & Metrics**: Measures catalog inspection timings and query counts during engine startup.

## Key Functions

```go
func GetDBInfo(db *sql.DB, dbType string, blocklist []string) (*sdata.DBInfo, error)
func GetDBInfoWithContext(ctx context.Context, db *sql.DB, dbType string, blocklist []string) (*sdata.DBInfo, error)
```
