# schema

`schema` provides GraphQL introspection schema building (`__schema`, `__type`), schema diffing, and schema migration utilities.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/schema
```

## Overview

This package converts database metadata from `core/sdata` into standard GraphQL introspection responses and schema definitions, enabling GraphQL clients (e.g. GraphiQL, Apollo, Relay) to inspect the API.

## Features

- **Introspection Generation (`intro.go`)**: Builds full GraphQL introspection types (`__Schema`, `__Type`, `__Field`, `__InputValue`, `__EnumValue`, `__Directive`) directly from database tables, columns, and relationships.
- **Schema Diffing (`diff.go`)**: Computes structural differences between two database schemas to detect added, modified, or removed tables, columns, and foreign keys.
- **DDL & Migration Generation (`ddl.go`, `generate.go`)**: Generates migration statements based on schema differentials.

## Key Functions

```go
func BuildIntrospection(opts IntroOptions) (result json.RawMessage, err error)
func Diff(s1, s2 *sdata.DBSchema) (*DiffResult, error)
```
