# Input & Output Seams: Pluggable Languages, Pluggable Formats

**Status:** Proposal / design document
**Scope:** `core/*`, `serv/*`, `openapi/*`, `webui/*`
**Companion reading:** [architecture.md](./architecture.md), `core/dbadapter/registry.go`

---

## 1. TL;DR

GraphJin Slim already has one proven registry seam — `core/dbadapter` — that lets
database engines (Postgres, SQLite) plug in without core importing a single
driver. This document proposes completing the symmetry with two more seams built
on exactly the same pattern:

| Seam | Registry | Contract | First implementations |
|---|---|---|---|
| **Input** (query languages) | `langadapter` | compile text → `*qcode.QCode` | GraphQL; later: URL query DSL |
| **Output** (wire formats) | formatter registry | format `*Result` → bytes + envelope | JSON (today's behavior); later: protobuf, msgpack, CSV |

Also done as of this writing: `core/psql` was renamed to `core/sqlgen`, because
the package is the *generic* SQL generator for every dialect and the old name
misled everyone who read the tree for the first time. (Phase 1 complete; zero
references remain.)

The goal is **not** "support five languages." The goal is:

1. Make the IR boundary (`qcode.QCode`) real instead of aspirational — today
   GraphQL is fused into eight packages downstream of it.
2. Unlock one high-fit second input language (a PostgREST-style URL DSL) for the
   segment of users who want runtime-composable queries over plain GETs.
3. Fix output-side defects that exist *today*: non-GraphQL consumers receive
   GraphQL-shaped error envelopes, and JSON is the only possible wire format.

Every phase leaves the system fully working. No big-bang rewrite.

---

## 2. Problem & Motivation

### 2.1 The advertised boundary vs. reality

The architecture claims the query frontend is swappable: any language can
compile into the `qcode` IR, which flows into SQL generation and execution.
That claim is only conventionally true. In practice GraphQL is fused into the
system at eight verified points (line numbers as of this writing):

| # | Coupling | Evidence |
|---|----------|----------|
| 1 | The GraphQL parser is baked into the qcode compiler entry | `qcode.Compiler.Compile()` calls `graph.Parse(query)` itself (`core/qcode/qcode.go:525`) |
| 2 | The engine parses GraphQL directly | `graph.FastParseBytes` inside the request path (`core/engine/engine.go:553`), used for op-name/type extraction and APQ |
| 3 | The security layer parses GraphQL | allow-list keys saved queries by GraphQL operation name (`core/allow/allow.go:158`) |
| 4 | Cross-DB joins round-trip through GraphQL text | `dbjoin.BuildChildGraphQLQuery` re-emits literal GraphQL source mid-execution (`core/dbjoin/dbjoin.go:105`); `engine/dbjoin.go:186–192` reparses it |
| 5 | Introspection (a GraphQL protocol feature) lives in core engine | `core/introspection`, `engine/intro.go`, `EnableIntrospection` config |
| 6 | Validators are keyed on parser token types | `core/valid/valid.go` registers validators against `graph.ParserType` |
| 7 | Schema metadata speaks GraphQL names | `schema.ColumnGraphQLType()` (`core/schema/intro.go:1171`), consumed by openapi |
| 8 | Surface layers hardcode the protocol | `serv/http/routes.go:27` mounts `/api/v1/graphql`; `webui/console/src/api.ts:5` falls back to it; openapi generates `GraphQLError` schemas and "Executes the %s GraphQL query" descriptions (`openapi/generate.go:136,359,395`) |

A seam with exactly one implementation on each side is not a seam; it is a
layering diagram. The backend proved the pattern works — postgres and sqlite
both self-register through `dbadapter`, and core stays driver-free. The
frontend never received the same treatment.

This has a daily maintenance cost even if no second language is ever written:
every feature touching the query surface threads through packages that pretend
GraphQL does not exist but import it anyway; internal plumbing (dbjoin) cannot
move data without serializing to a wire format and reparsing it; the security
layer depends on parser trivia.

### 2.2 Why openapi/saved-queries does not cover this

The fair objection: *"the openapi module already gives non-GraphQL clients
access — isn't that enough?"* Partially. It solves a different problem:

| | REST from saved queries (today) | An input language |
|---|---|---|
| What clients express | Nothing — they invoke a named, pre-approved query | New queries composed at request time |
| New access pattern | Developer edits GraphQL, saves it, redeploys | Client composes it right now |
| Analogy | Fixed menu | Kitchen |

Saved-query REST is a projection of operations frozen at deploy time. It cannot
serve combinatorial access — arbitrary filter/pagination/nesting combinations —
because every combination must be anticipated server-side ahead of time. That
combinatorial workload is precisely what a query language is for.

### 2.3 The one second input language worth building

GraphJin's pitch is zero-code APIs, and a meaningful part of its potential
audience finds GraphQL ceremony excessive for CRUD: POST-everything, mandatory
client libraries, persisted-query machinery. The proven alternative pattern is
a PostgREST-style URL DSL:

```text
GET /products?select=name,price&price=gt.100&order=price.desc&limit=20
```

That is expressive-at-runtime like GraphQL, but GET-able: HTTP-cacheable,
deep-linkable, curl-able, zero client library. Saved-query REST cannot become
this, because composition happens per-request, not at save time.

This is the concrete second-language candidate. Not five hypothetical DSLs —
one that fits the existing user base and validates the seam with a real second
implementation.

### 2.4 Output-side defects exist today

Independent of any new input language:

- **Envelope ownership is undefined.** `Result.Errors` follows GraphQL error
  conventions, and responses are wrapped as `{"data": ..., "errors": ...}`.
  A REST/openapi consumer who opted out of GraphQL receives GraphQL-shaped
  errors anyway. The envelope belongs to nobody, so it defaults to GraphQL.
- **JSON is the only wire format.** Binary formats (protobuf/msgpack) matter
  for mobile/IoT bandwidth; CSV matters for data export. Today the nested
  document is assembled inside SQL (`json_build_object`/`json_agg` in the SQL
  generator), so nothing downstream can re-shape it without another pass.

### 2.5 What this is explicitly not

- Not a plugin marketplace or dynamic loading story — registration is static,
  init-time, compile-time linked, exactly like `dbadapter`.
- Not a commitment to maintain N languages. Core ships GraphQL; everything
  else lives in its own module and registers itself.
- Not a transport unification. Languages differ at the HTTP layer too, and the
  design keeps them different on purpose (see §6.3).
- Not a rewrite. Phases are individually shippable; the public API survives
  every phase unchanged.

---

## 3. Design principles

1. **One pattern, three registries.** `langadapter` (input), formatters
   (output), and the existing `dbadapter` share semantics: `Register` panics on
   duplicates, `Lookup` fails with a sorted list of available names, adapters
   self-register from their own modules via `init()`. Core imports nothing
   concrete.
2. **Registries expose discovery; surfaces consume only discovery.**
   serv, webui, and openapi never hardcode a language name or endpoint path.
3. **Core is protocol-free.** Nothing under `core/` outside the GraphQL
   frontend imports the GraphQL parser.
4. **Compatibility is a feature.** `/api/v1/graphql`, the Go API
   (`gj.GraphQL(...)`), response bytes, and saved-query storage all keep
   working identically after every phase.
5. **Optional capabilities over fat interfaces.** Like `dialect`'s optional
   `FullQueryCompiler`/`NameMapSetter`, capabilities beyond the minimal
   contract are separate interfaces asserted at runtime.

---

## 4. Target architecture

```text
          INPUT SEAM                          OUTPUT SEAM
 ┌────────────────────────────┐      ┌────────────────────────────┐
 │ langadapter registry       │      │ formatter registry         │
 │                            │      │                            │
 │  graphql → QCode           │      │  Result → json             │
 │  urldsl  → QCode           │      │  Result → proto/msgpack/csv│
 └─────────────┬──────────────┘      └─────────────▲──────────────┘
               ▼                                   │
        ┌──────────────────────────────────────────────────┐
        │                qcode.QCode (IR)                  │
        │   language-neutral query plan, schema-resolved   │
        └─────────────┬────────────────────────────────────┘
                      ▼
        ┌──────────────────────────────────────────────────┐
        │     BACKEND: sqlgen (SQL AST) + dialect renderers│
        └─────────────┬────────────────────────────────────┘
                      ▼
        ┌──────────────────────────────────────────────────┐
        │  RUNTIME: engine · dbjoin · runtime              │
        │    ENGINE SEAM (exists): dbadapter               │
        └─────────────┬────────────────────────────────────┘
                      ▼
                 *Result{Data}
```

Full symmetry: **N input languages × M database engines × K output formats**,
meeting at the same IR and runtime. Adding an axis never touches the other two.

Package layout — final state:

```text
graphql/                 its own module — the GraphQL frontend: parser,
                         lowering, validation registry; self-registers via
                         langadapter and is blank-imported like an engine
core/langadapter/        the input seam + registry (mirror of dbadapter)
core/format/             the output seam + registry (JSON built in)
core/qcode/              pure IR — types, enums, pure methods; imports only sdata
core/sqlgen/             SQL generation for every dialect (renamed from psql)
```

---

## 5. The IR contract (`qcode.QCode`)

The IR is the pivot both seams hang off; it must be strictly language-neutral.

**Stays as-is (already neutral):** `Selects []Select`, `Roots []int32`,
`Mutates []Mutate`, expression trees (`Exp`), paging/ordering, `Schema`
(`sdata.DBSchema`), cache metadata.

**Neutralized (small deltas):**

| Item | Problem | Resolution |
|---|---|---|
| `Order` enum carries `ASC NULLS FIRST/LAST` | SQL-specific vocabulary in the "neutral" layer | Acceptable pragmatism: keep, but document as SQL-rendering hints, not language syntax |
| `NewCompiler` checks `DBType() != "sqlite"` | dialect-awareness leak | Move the check behind a schema capability flag |
| `Fragments []Fragment` | reads GraphQL-specific | Rename conceptually to reusable named snippets; harmless structurally |
| `Typename bool` | introspection flag | Move behind a language-capability field on `QCode` options |

Rule going forward: **qcode may not gain a dependency on any language or wire
protocol**, enforced by an import audit in CI (§11 Phase 8).

---

## 6. Input seam: `langadapter`

### 6.1 The core interface

Mirrors `dbadapter.Adapter`: small, honest, impossible to misuse.

```go
package langadapter

// Info is cheap metadata extracted without full compilation.
// Replaces direct FastParse calls in engine and allow.
type Info struct {
    Operation string // "query", "mutation", ... (language-defined)
    Name      string // operation name, if any
}

// Language turns source text into the IR. This is the whole contract.
type Language interface {
    // Name matches configuration and route registration ("graphql").
    Name() string

    // Compile parses and lowers source into the IR, validated against
    // the given database schema.
    Compile(query []byte, vars map[string]json.RawMessage,
        opts CompileOptions) (*qcode.QCode, error)
}

// Registry semantics identical to dbadapter: panic on duplicate
// registration, Lookup returns available names on failure, Names()
// sorted for stable output.
func Register(l Language)
func Lookup(name string) (Language, error)
func Names() []string
```

### 6.2 Optional capabilities

Each is a separate interface; consumers type-assert when they need it.

```go
// FastInfo extracts identity cheaply (allow-list keying, APQ, logging).
// Engine and allow use this instead of importing any parser.
type FastInfoer interface {
    FastInfo(query []byte) (Info, error)
}

// HTTPEndpointer lets a language own its HTTP surface. Mounted by serv;
// never hardcoded in routes.go.
type HTTPEndpointer interface {
    EndpointPath() string // "/api/v1/graphql"
    Handler(deps HandlerDeps) http.Handler
}

// SchemaDescriber exposes typed metadata about operations/scalars/errors.
// Powers webui autocomplete and openapi spec sections; wraps introspection
// for GraphQL.
type SchemaDescriber interface {
    Describe(schema *sdata.DBSchema, ns string) (Description, error)
}

// SubqueryBuilder lets the runtime fan out cross-database child work in the
// language's own medium. Replaces dbjoin's GraphQL text re-emission; a
// language could return QCode subsets, URLs, or serialized plans.
type SubqueryBuilder interface {
    BuildChild(sel *qcode.Select, selects []qcode.Select,
        fkCol sdata.DBColumn, parentID []byte) ([]byte, error)
}
```

### 6.3 Per-language endpoints (and why not one neutral endpoint)

Discovery first — clients never hardcode paths:

```text
GET /api/v1/languages
→ { "inputs":  [{"name": "graphql", "endpoint": "/api/v1/graphql"}, ...],
    "outputs": ["json"] }
```

Then each registered `HTTPEndpointer` contributes its own route; `routes.go`
becomes a dumb mounter. A single neutral endpoint (`/api/v1/query?lang=x`)
was considered and rejected: **a language is a wire protocol, not just a
parser.** GraphQL wants POST-with-JSON-vars, APQ extensions, and someday
websocket subscriptions; a URL DSL wants GET + query params for HTTP caching.
A unified endpoint forces a lowest-common-denominator transport, recreating
the coupling problem inverted. Per-adapter handlers let each language own its
quirks while core stays protocol-free.

### 6.4 Where GraphQL goes

`core/lang/graphql` becomes the single owner of:

- the `graph` lexer/parser (package moves or becomes private to the frontend),
- GraphQL-specific lowering currently interleaved through ten qcode files
  (~300 `graph.` references): AST walking, directives, fragment splicing,
- introspection (implements `SchemaDescriber`; the engine delegates via
  interface assertion instead of owning the protocol),
- the `{"data": ..., "errors": ...}` envelope conventions (§7.3).

After this lands, the import audit is simple to state: **`v3/graph` may only be
imported from within `core/lang/graphql/`.**

---

## 7. Output seam: formats

### 7.1 Interface and registry

```go
package format

// Formatter renders an execution result into a wire representation,
// including its envelope (data + errors + extensions placement).
type Formatter interface {
    Name() string // "json"
    Format(w io.Writer, res *Result) error
}

func Register(f Formatter)
func Lookup(name string) (Formatter, error)
func Names() []string
```

Selection: explicit (`?format=`), then content negotiation (`Accept`), then
default `json`. Default behavior is byte-identical to today.

### 7.2 Shallow now, deep later

There are two possible insertion depths, and they must not be conflated:

- **Shallow (ship first):** transform the result stream post-execution using
  `core/jsn`'s zero-allocation scanner — JSON → protobuf/msgpack/YAML/CSV-ish
  re-encoding. Works because the DB already emits a structured nested document.
  Cannot change projection semantics.
- **Deep (designed-for, not built):** binary/columnar-native output requires
  nesting *not* happen in SQL, i.e. projection hints flowing through
  qcode → sqlgen → dialect. Reserved as an optional `Projector` capability
  interface on formatters; sqlgen consults it only when present. No code until
  a real consumer exists.

### 7.3 Envelope ownership

Today the `{"data": ..., "errors": ...}` shape is baked into results and
surface layers. Under the new model the **formatter owns the envelope**: the
JSON formatter reproduces today's bytes exactly (compatibility), while future
formatters emit their own conventions (plain REST-style problem+json, flat
error tables, etc.). Error *types* remain engine-owned; only their wire shape
moves to the seam. This retroactively fixes REST consumers receiving
GraphQL-shaped errors.

---

## 8. Neutral engine entry & security layer

```go
// New neutral entry point.
func (g *GraphJin) Query(ctx context.Context, lang string,
    query string, vars json.RawMessage, rc *RequestConfig) (*Result, error)

// Backward-compatible thin wrapper: resolves "graphql" internally.
func (g *GraphJin) GraphQL(...) // unchanged signature, same behavior
```

Internal consequences:

- Engine drops its `graph` import entirely; op-name/type extraction goes
  through `FastInfoer`.
- APQ caching keys become `(lang, hash)` pairs.
- Allow-list records gain a `lang` field; lookups use `FastInfoer`. Stored
  queries without a lang field default to `"graphql"` — zero migration.
- dbjoin switches from GraphQL text round-trips to `SubqueryBuilder`
  (defaulting to the graphql implementation where present).

---

## 9. Surface layers

All three surfaces stop being GraphQL programs and become registry clients.

### 9.1 serv

- `routes.go` mounts whatever `HTTPEndpointer`s are registered plus the
  discovery endpoint. `/api/v1/graphql` survives because the graphql adapter
  claims that exact path — legacy clients notice nothing.
- Response writing goes through the formatter registry (§7.1 selection rules).
- Saved-queries API gains a `lang` field on records and filters.

### 9.2 openapi

- Iterates languages implementing `SchemaDescriber`; emits per-language path
  groups and error-envelope schemas from adapter metadata.
- Deletes hardcoded `GraphQLError` components and "GraphQL variable"
  descriptions (`openapi/generate.go:136,395`).
- Drops its direct `core/v3/graph` import.
- Consumes renamed neutral type mapping instead of `ColumnGraphQLType`.

### 9.3 webui

- Console fetches `/api/v1/languages` at boot; the `?endpoint=` override still
  wins when present.
- Endpoint picker lists registered inputs; with one language installed the UX
  is visually identical to today.
- Queries panel groups by language.

### 9.4 core/schema

`ColumnGraphQLType` renames to a neutral scalar mapping (it was always just
DB-type → scalar-name). openapi and the graphql frontend consume the neutral
name; the frontend applies GraphQL naming on top.

---

## 10. Renames

| From | To | Why |
|---|---|---|
| `core/psql` | `core/sqlgen` | It is the generic SQL generator feeding every dialect renderer; "psql" reads as a Postgres client tool and misleads every newcomer. Role per the architecture: QCode IR → SQL AST → rendered SQL via `dialect`. |

Blast radius (verified, small): ~34 Go references outside the package
(`engine/{gstate,engine,arguments,dbjoin,testkit,multidb}.go`, comments in
`dialect/`, one qcode test), directories `core/sqlgen/{bench,tests}` (renamed from `core/psql`),
identifiers `sqlgenCompiler` (was `psqlCompiler`) and `scompile` (was `pcompile`), and doc mentions. One isolated commit;
grep-audited to zero leftover references.

---

## 11. Migration plan

**Status as of this writing:** all phases are complete. The full
IR/lowering split (formerly Phase 4) has landed: `core/qcode` is now a
pure IR package — types and pure methods only, importing nothing but
`sdata` — while `core/lang/graphql` owns the entire frontend: the
compiler, parser-driven lowering, validation-registry registration, and
per-database language binding. Backend packages (`sqlgen`, `dialect`,
`dbjoin`) consume real `qcode` definitions directly; no alias shim
remains. Frontend-only scratch state (parsed payload nodes, tree
building) lives on `graphql.Compiler`, and mutation payloads cross into
the backend exclusively through neutral IR fields (`Mutate.ColVals`,
`IsJSON`, `Array`). CI enforces qcode's protocol-freedom and the
parser's frontend confinement; the remaining transitional `graph`
imports outside `core/lang/graphql` (allow-list metadata extraction,
dbjoin's legacy fan-out encoding, openapi parameter docs) are documented
allowlist entries slated for their own capability seams.

Each phase ships green and independently revertible.

| Phase | Scope | Exit criteria |
|---|---|---|
| **0** | Baseline: full test matrix (core unit + integration w/ postgres+sqlite adapters) | Green recorded |
| **1** | Rename `psql` → `sqlgen`, zero behavior change, docs updated | `grep -ri psql` clean (excluding legit "postgres"); tests green |
| **2** | `langadapter` + `format` registries; first `Language` impl wraps today's `qcode.Compiler` untouched; JSON formatter wraps current rendering | Registries live; no callers changed yet; tests green |
| **3** | Neutral `gj.Query(lang, …)`; engine + allow drop direct parsing via `FastInfoer`; allow-list `lang` field | Engine compiles without importing `graph` |
| **4** | Extract GraphQL frontend into `lang/graphql`; qcode loses its `graph` import | Done: `core/qcode` is pure IR (imports only `sdata`); frontend fully owns parsing and lowering |
| **5** | Runtime edges: dbjoin via `SubqueryBuilder`; introspection delegated as `SchemaDescriber` capability | Done: joins resolve through the language; introspection gated to graphql requests |
| **6** | Output seam activation: formatter registry selected in serv; envelope owned by formatters | Done: byte-identical JSON verified by encoder-equivalence test |
| **7** | Surfaces: `/api/v1/languages` discovery; webui discovery fallback; `ColumnGraphQLType` → `ColumnScalarType` | Done: no hardcoded language endpoints outside adapters |
| **8** | Docs (architecture.md refresh) + CI import audits (no stray `graph`, no `psql`, qcode protocol-free) | Done: ci.yml enforces build, tests, and all four audits |

Deliberate ordering: Phases 2–3 deliver the seam and compatibility shim before
Phase 4 touches internals, so the risky restructuring happens under a stable
interface contract. Surface layers wait for both registries (hence last-but-one).

---

## 12. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Phase 4 is the largest diff (~300 refs across 10 files) | Ships after the seam stabilizes (Phases 2–3); pure mechanical moves verified by identical test output |
| Saved-query storage compatibility | `lang` field defaults to `"graphql"` on read; old stores need no migration |
| Response byte drift | Golden-file tests on default formatter before/after each phase; byte-identical is an exit criterion |
| Public API breakage | `gj.GraphQL()` signature and behavior frozen; `gj.Query()` is additive |
| Second-language scope creep | URL DSL is *not* part of this plan; it is the validation exercise afterwards (§13) |
| Performance regression in dbjoin | Benchmarks exist (`core/sqlgen/bench`, `bench/`); compare before/after Phase 5 |

---

## 13. Payoff walkthrough: adding the URL DSL after this lands

What a contributor writes once the seams exist — and nothing else:

```go
package urldsl

// 1. Compile: parse query params into QCode using sdata path-finding.
type Lang struct{ /* schema injected per-db by engine */ }

func (l *Lang) Name() string { return "urldsl" }
func (l *Lang) Compile(q []byte, vars map[string]json.RawMessage,
    opts langadapter.CompileOptions) (*qcode.QCode, error) {
    // select=, filters (op.val syntax), order=, limit/offset → Select/Exp/Paging
}

// 2. Own the transport: GET-friendly by nature.
func (l *Lang) EndpointPath() string { return "/api/v1/urldsl" }

// 3. Metadata for openapi/webui (optional).
func (l *Lang) Describe(...) (langadapter.Description, error) { ... }
```

Registered via `init()` in its own module — mirroring how `postgres/` and
`sqlite/` register engines. Everything downstream (SQL generation, multi-DB
joins, retries, field crypto, allow-list enforcement, schema watching, openapi
docs, webui listing) is inherited for free. That inheritance list is the entire
argument for the seams: today, a second language would have to reimplement or
fork all of it.

---

## 14. Objections, answered

**"Just support GraphQL forever."** Fine — and this plan makes that cheaper,
not more expensive: fusion taxes (§2.1 items 1–6) are paid regardless, and the
refactor removes them.

**"openapi already solved non-GraphQL access."** For invoking saved queries,
yes (§2.2). For runtime composition over GET, no — that requires a language.

**"YAGNI on formats."** The envelope defect (§2.4) is not YAGNI; it is wrong
output for existing non-GraphQL consumers today. Formats beyond JSON remain
unbuilt until requested — the registry just reserves the seam.

**"Two registries is over-engineering."** Both mirror a pattern the codebase
already endorses (`dbadapter`), each is ~100 lines, and neither adds a runtime
dependency to core. The alternative — the status quo — is eight documented
coupling points that make the architecture doc untrue.
