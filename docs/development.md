# Development

## Workspace

This repository is a Go workspace containing two modules:

```text
core/   github.com/aegion-dynamic/graphjin-slim/core/v3
serv/   github.com/aegion-dynamic/graphjin-slim/serv/v3
```

The service module uses the local core module through the workspace and its
`replace` directive. Changes to core should therefore be tested with both
modules.

## Validation

Run the complete suite from the repository root:

```bash
go test ./core/... ./serv/...
go vet ./core/... ./serv/...
```

Format changed Go files before committing:

```bash
gofmt -w path/to/changed.go
```

The core and service tests are intentionally usable without a permanent
external database. Prefer synthetic schemas, SQLite, in-memory adapters, and
fake SQL drivers for new tests.

## Where To Change Code

| Change | Package |
| --- | --- |
| GraphQL parsing | `core/graph` |
| Query compiler & IR | `core/qcode` |
| SQL generation | `core/sqlgen` |
| Postgres/SQLite SQL differences | `core/dialect` |
| Schema metadata & relationships | `core/sdata` |
| Database introspection | `core/introspection` |
| Introspection schema generation | `core/schema` |
| Core public behavior | `core` |
| Service configuration | `serv` / `serv/config` |
| Database connections | `serv/database` |
| Response caching | `serv/cache` |
| HTTP route registration | `serv/http` |
| Service lifecycle | `serv/lifecycle` |

## Dependency Rules

- Keep public imports on `core/v3` and `serv/v3` stable.
- Do not import `serv` from core or from low-level adapters.
- Keep transport code out of core compiler packages.
- Keep database-specific details inside database or dialect adapters.
- Prefer a small interface at a real seam over a large forwarding interface.
- Add a second adapter before introducing a new replaceable seam.
- Test behavior through the module interface instead of private implementation
  state.

## Refactoring Workflow

1. Identify the behavior cluster and its callers.
2. Write down the interface callers actually need.
3. Design the interface a second way before implementing it when the seam is
   uncertain.
4. Move implementation behind the chosen seam without changing behavior.
5. Move tests to the new module and remove tests that reach past its interface.
6. Run both module suites and `go vet`.
7. Commit one coherent extraction and push it.

## Commit Style

Use focused commits that describe the architectural change:

```text
refactor: move database adapters into package
refactor: module service response caching
docs: document core package responsibilities
```

Avoid mixing unrelated formatting, generated dependency changes, and behavior
changes into a package move.
