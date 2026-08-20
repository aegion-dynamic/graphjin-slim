# dbjoin

`dbjoin` implements cross-database join execution, child GraphQL query generation, and multi-database result merging for GraphJin.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/dbjoin
```

## Overview

When a GraphQL query traverses relationships between tables that reside in different databases, `dbjoin` constructs targeted child queries, maps join keys across boundaries, and merges multi-root JSON payloads.

## Features

- **Child Query Construction (`BuildChildGraphQLQuery`)**: Dynamically constructs GraphQL child queries filtered by parent foreign key identifiers.
- **Root Field Query Filtering (`BuildDatabaseQuery`)**: Filters an incoming GraphQL AST to only the root fields designated for a specific database.
- **Result Merging (`MergeRootResults`)**: Merges concurrent JSON responses from independent databases into a unified response object.
- **Join Field Identification (`DatabaseJoinFieldIDs`)**: Discovers and tracks placeholder fields in parent queries that require cross-database stitching.

## Key Functions

```go
func BuildChildGraphQLQuery(sel *qcode.Select, selects []qcode.Select, fkCol sdata.DBColumn, parentID []byte) []byte
func BuildDatabaseQuery(rawQuery []byte, rootFields []string) ([]byte, error)
func DatabaseJoinFieldIDs(selects []qcode.Select) ([][]byte, map[string]*qcode.Select, error)
func MergeRootResults(results []DBResult) ([]byte, error)
```
