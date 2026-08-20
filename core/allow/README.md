# allow

`allow` manages the saved-query allow-list and Automatic Persisted Queries (APQ) for GraphJin.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/allow
```

## Overview

In production mode, GraphJin can enforce an allow-list of pre-approved GraphQL operations to prevent arbitrary query execution. `allow` loads, parses, and indexes allowed GraphQL queries by their SHA-256 hash.

## Features

- **Query Indexing**: Fast in-memory lookup of approved queries by hash or name.
- **Fragment Resolution**: Automatically embeds and resolves referenced GraphQL fragments during allow-list parsing.
- **Dev-mode Auto-saving**: In development mode, new queries can be automatically captured and written to the configured filesystem allow-list.
- **APQ Support**: Validates persisted query requests matching incoming hash payloads.

## Key Types & Functions

```go
type List struct { ... }
type Item struct { ... }

func New(path string, fs FS) (*List, error)
func (l *List) Get(hash [32]byte) (Item, bool)
func (l *List) Set(item Item) error
func (l *List) Load() error
```
