# sdata

`sdata` is the schema metadata and relational join graph foundation of GraphJin.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/sdata
```

## Overview

`sdata` structures raw database inspection data (`DBInfo`) into a bidirectional relationship graph (`DBSchema`). It models tables, columns, primary keys, foreign keys, array relationships, polymorphic relations, and multi-hop join paths.

## Key Capabilities

- **Bidirectional Relationship Graph**: Automatically identifies one-to-one, one-to-many, and many-to-many join paths across database tables.
- **Shortest Path Resolution (`path.go`)**: Finds optimal join sequences between disconnected or multi-hop tables (including through-tables).
- **Snapshot & Serialization (`snapshot.go`)**: Serializes and deserializes schema metadata for fast startup caching.
- **Table & Column Aliases**: Resolves table aliases, camelCase-to-snake_case mappings, and custom relationship overrides.

## Key Types & Functions

```go
type DBSchema struct { ... }
type DBTable struct { ... }
type DBColumn struct { ... }
type DBRel struct { ... }

func NewDBSchema(dbinfo *DBInfo, aliases map[string][]string) (*DBSchema, error)
func (s *DBSchema) Find(schema, table string) (DBTable, error)
func (s *DBSchema) FindRel(fromSchema, fromTable, toSchema, toTable string) (DBRel, error)
func (s *DBSchema) FindPath(fromTable, toTable DBTable, through string) ([]DBRel, error)
```
