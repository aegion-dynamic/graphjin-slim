# qcode

`qcode` is the intermediate representation (IR) and semantic query compiler of GraphJin.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/qcode
```

## Overview

`qcode` bridges raw GraphQL syntax trees from `core/graph` and database schema metadata from `core/sdata`. It validates field names against database columns, resolves multi-hop table relationships, flattens fragments, processes directives, validates mutation payloads, and outputs a normalized `QCode` IR plan ready for SQL compilation.

## Pipeline Responsibilities

- **Semantic Validation**: Validates GraphQL fields and arguments against the database schema graph.
- **Relationship Resolution**: Traverses foreign-key paths and through-table joins between parent and child selectors.
- **Expression Compilation (`expr.go`, `exp.go`)**: Compiles boolean expressions, nested `where` clauses, comparison filters, and GIS operators into an expression tree.
- **Mutation Planning (`mutate.go`)**: Validates input objects, nested relationship mutations, and upsert conflict actions (`on_conflict`).
- **Analytics & Window Functions (`analytics.go`, `window.go`)**: Parses directive-driven analytics like `@running`, `@moving`, `@rank`, and window partitioning.

## Key Types & Functions

```go
type QCode struct {
    Type      QType
    Name      string
    Selects   []Select
    Mutates   []Mutate
    Roots     []int32
    ...
}

func NewCompiler(s *sdata.DBSchema, conf Config) (*Compiler, error)
func (co *Compiler) Compile(query []byte, vmap map[string]json.RawMessage, namespace string) (*QCode, error)
```
