# How to Build a Custom Query & Response DSL on GraphJin

GraphJin's architecture is structured around a classic **Compiler Intermediate Representation (IR)** (`qcode`). The query frontend (parsing syntax) and response backend (response serialization) are completely decoupled from query validation, relational path-finding, SQL generation, and execution.

This document explains:
1. How to build a custom **Input Query DSL** (to replace or supplement GraphQL).
2. How to format the **Response Output in a Custom DSL** (e.g., Protobuf, MsgPack, CSV, YAML, JSON-API, or custom wire formats).

---

## 1. End-to-End Compiler Pipeline

```text
┌────────────────────────────────────────────────────────┐
│                   QUERY INPUT (Frontend)               │
│                                                        │
│   GraphQL Text          Custom DSL           JSON AST  │
│        │                     │                   │     │
│        ▼                     ▼                   ▼     │
│   [core/graph]       [core/your_dsl]     [core/json_dsl]
└──────────────────────────────┬─────────────────────────┘
                               │
                               ▼
┌────────────────────────────────────────────────────────┐
│             INTERMEDIATE REPRESENTATION (IR)           │
│                                                        │
│   [core/qcode]  ──  Normalized Query Tree (AST)       │
│                     (Tables, Filters, Joins, Rel Paths)│
└──────────────────────────────┬─────────────────────────┘
                               │
                               ▼
┌────────────────────────────────────────────────────────┐
│                 BACKEND (Code Generator)               │
│                                                        │
│   [core/psql]   ──  SQL AST & Query Dialect Compiler   │
│   [core/dialect]──  Postgres & SQLite Renderers        │
└──────────────────────────────┬─────────────────────────┘
                               │
                               ▼
┌────────────────────────────────────────────────────────┐
│                   EXECUTION RUNTIME                    │
│                                                        │
│   [core/engine] ──  Execution State, Cache & Pooling   │
│   [core/dbjoin] ──  Cross-DB Distributed Joins         │
│   [core/runtime]──  Retry Resilience & Field Crypto    │
└──────────────────────────────┬─────────────────────────┘
                               │
                               ▼
┌────────────────────────────────────────────────────────┐
│               RESPONSE FORMATTER (Output)              │
│                                                        │
│    JSON Bytes           Protobuf / MsgPack    Custom DSL│
│        │                     │                   │     │
│        ▼                     ▼                   ▼     │
│    [Default]          [core/serializer]   [core/your_fmt]
└────────────────────────────────────────────────────────┘
```

---

## 2. Building a Custom Input Query DSL

The boundary between any query language and SQL code generation is `*qcode.QCode`.

```go
type QCode struct {
    Type      QType          // QTQuery, QTMutation, QTSubscription
    Selects   []Select       // Flat array of table selections
    Roots     []int32        // Indices of root selections in Selects
    Remotes   int32          // Number of remote HTTP joins
}
```

### Key Elements of `qcode.Select`:
- **`Table`**: Target table name in the relational database.
- **`Fields`**: Selected column names and aliases.
- **`Where`**: Filter expression tree (`qcode.Exp`) composed of boolean (`AND`, `OR`, `NOT`) and binary operators (`eq`, `gte`, `in`, `is_null`, etc.).
- **`OrderBy`**: Sorting columns and directions (`Asc`, `Desc`).
- **`Paging`**: Pagination limits and offsets (`Limit`, `Offset`).
- **`Children`**: Indices of nested child selections (representing foreign key joins).
- **`Rel`**: Relationship metadata resolved by `core/sdata` (e.g., `RelOneToMany`, `RelOneToOne`).

### Implementing the Input DSL:

```go
package mydsl

import (
    "github.com/aegion-dynamic/graphjin-slim/core/v3/psql"
    "github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
    "github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

type Compiler struct {
    schema       *sdata.DBSchema
    psqlCompiler *psql.Compiler
}

func NewCompiler(schema *sdata.DBSchema) *Compiler {
    return &Compiler{
        schema:       schema,
        psqlCompiler: psql.NewCompiler(schema),
    }
}

func (c *Compiler) Compile(dslInput []byte) (*qcode.QCode, error) {
    // 1. Parse your DSL text / AST
    ast, err := parseMyDSL(dslInput)
    if err != nil {
        return nil, err
    }

    // 2. Build the QCode IR
    qc := &qcode.QCode{Type: qcode.QTQuery}

    // 3. Resolve parent-child relationships using schema path finding
    //    c.schema.FindPath(parentTable, childTable, throughColumn)

    return qc, nil
}
```

---

## 3. Formatting Replies in a Specific Output DSL

There are two architectural strategies for generating custom response formats, depending on whether you want zero-copy stream transformation or database-native projection:

### Approach A: Fast Stream Transformation (Recommended)

PostgreSQL and SQLite are optimized to produce structured JSON directly from the storage engine via `json_build_object()` and `json_group_array()`.

Instead of deserializing into Go structs and re-serializing, you can implement a streaming transformer using `core/jsn` to convert the DB payload directly into your target DSL wire format:

```text
Database JSON Stream ──> [core/jsn scanner] ──> Custom DSL Emitter ──> Client Buffer
```

#### Example: Custom Response Formatter

```go
package response

import (
    "bytes"
    "io"

    "github.com/aegion-dynamic/graphjin-slim/core/v3/jsn"
)

type Formatter interface {
    Format(w io.Writer, rawJSON []byte) error
}

// Example: Proto / Compact DSL Emitter
type CustomDSLEmitter struct{}

func (e *CustomDSLEmitter) Format(w io.Writer, rawJSON []byte) error {
    // 1. Scan JSON tokens efficiently using core/jsn scanner (zero-alloc)
    // 2. Emit corresponding custom DSL tokens, binary frames, or Protobuf messages
    var buf bytes.Buffer
    buf.WriteString("@response:ok\n")
    // ... encode fields ...
    _, err := w.Write(buf.Bytes())
    return err
}
```

#### Pluggable Response Pipeline:
You can wrap execution results with your custom formatter:

```go
res, err := gj.GraphQL(ctx, query, vars, reqConfig)
if err != nil {
    return nil, err
}

var output bytes.Buffer
if err := customEmitter.Format(&output, res.Data); err != nil {
    return nil, err
}
return output.Bytes(), nil
```

---

### Approach B: SQL-Level Projection Customization (`core/psql`)

If you want the database engine itself to format output differently (e.g. flat columnar tuples, CSV rows, or XML), you customize the root selection rendering in `core/psql`:

1. **`core/psql/query.go`**:
   - By default, `renderSelect()` generates `json_build_object(...)` or `json_agg(...)`.
   - You can configure custom root projections (e.g. `xml_element`, raw column projections for binary drivers, or tabular arrays).
2. **`core/dialect`**:
   - Provides dialect-specific formatting functions for PostgreSQL and SQLite.

---

## 4. What Downstream Modules Handle Automatically

Because the compilation pipeline, execution engine, and serialization layer are strictly modular, the following core capabilities remain **100% reusable across all DSLs**:

| Module | Feature Provided Automatically |
| --- | --- |
| **`core/sdata`** | Relational graph path-finding, multi-hop foreign key discovery, composite keys, and table aliasing. |
| **`core/psql`** | Single-query nested JSON generation, CTE construction, aggregations, window functions, upserts, and mutations. |
| **`core/dialect`** | Database-specific syntax formatting for **PostgreSQL** and **SQLite**. |
| **`core/engine`** | Prepared statement caching, query argument binding, and database connection lifecycle. |
| **`core/dbjoin`** | Cross-database distributed joins and multi-database result stitching. |
| **`core/runtime`** | Transient lock contention retries, OpenTelemetry tracing spans, and field-level AES-GCM encryption. |
| **`core/watcher`** | Background schema change polling and hot-reload. |

---

## 5. Architectural Summary

- **Query Input (`core/graph` or `core/<your_dsl>`)**: Parses language-specific syntax into `qcode.QCode`.
- **Intermediate Representation (`core/qcode`)**: Language-agnostic query and filter tree.
- **Backend (`core/psql`, `core/dialect`)**: Dialect-aware SQL code generation.
- **Execution Runtime (`core/engine`, `core/dbjoin`, `core/runtime`)**: High-performance database execution and distributed join orchestration.
- **Response Output (`core/jsn` or `core/<your_format>`)**: Pluggable formatting (JSON, Protobuf, MsgPack, or custom wire DSLs).
