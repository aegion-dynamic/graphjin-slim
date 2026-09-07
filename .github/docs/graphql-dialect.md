# The GraphJin GraphQL dialect, explained

This is the reference and the explanation for the exact GraphQL dialect that
graphjin-slim understands. Every query and mutation shape in here was
executed against a live Postgres database (the sample event-management
schema shipped in `tests/dialect/schema/`) and the results you see are real
output.

All examples live as runnable `.gql` files in `tests/dialect/`. One command
stands up the database from scratch (docker, schema, seed) and verifies the
whole document:

```bash
tests/dialect/up.sh
# ...applies tests/dialect/schema/backend/*.hcl, seeds tests/dialect/schema/seed.sql,
# then runs the executor:
# 75 run, 0 failed
```

If an example documents a limitation it is marked with
`# expect-error: <message>` in its file, which means: this example is expected
to fail with exactly that error, and the executor passes it when it does. The
limitation examples double as regression markers.

The example database is a small event-management domain: `events` (the core
object), `registrations` (attendees per event), `form_fields` (per-event
registration form definitions), `users`, `profiles`, `taxonomy_terms` (a
self-referencing vocabulary table), `meetings` and a few others. Where the
shape of the data matters to a query, the example says which tables it
touches.

## 1. The mental model: your query is one SQL statement

GraphJin is not a GraphQL server that fans out resolvers. It is a compiler.
It takes your operation, resolves every field against the database schema,
and produces a single SQL statement. The database then returns the entire
JSON response as one row containing one JSON value. There is no second query,
no N+1, and no result stitching in Go (except for cross-database joins).

This single design decision explains almost everything else in this
document, so it is worth seeing it once. Take this query:

```graphql
query {
  events(limit: 1) { id slug title }
}
```

The SQL GraphJin generates is:

```sql
SELECT jsonb_build_object('events', "__sj_0"."json") AS "__root"
FROM ((SELECT true)) AS "__root_x"
LEFT OUTER JOIN LATERAL (
  SELECT COALESCE(jsonb_agg(__sj_0.json), '[]') AS json
  FROM (
    SELECT to_jsonb(__sr_0.*) AS "json"
    FROM (
      SELECT "events_0"."id" AS "id", "events_0"."slug" AS "slug",
             "events_0"."title" AS "title"
      FROM (
        SELECT "events"."id", "events"."slug", "events"."title"
        FROM "application"."events" AS "events"
        LIMIT 20
      ) AS "events_0"
    ) AS "__sr_0"
  ) AS "__sj_0"
) AS "__sj_0" ON 1=1
```

Three things to notice, because they explain the dialect's rules later:

1. Each GraphQL field maps to a column in the innermost SELECT. The layers
   above it shape those rows into one JSON object per row, then aggregate
   them into the response array with `jsonb_agg`.
2. Nesting is done with `LEFT JOIN LATERAL`, which means a nested query is
   physically correlated to its parent (see the next section).
3. Because the response shape is produced by the database, everything the
   response needs (including cursors, `__typename`, and aggregate values)
   must be expressible in that one statement.

Consequences you will hit in practice:

- If a feature cannot be expressed as a single SQL statement, it does not
  exist in the dialect.
- SQL-level errors (bad types, missing columns, syntax errors in generated
  SQL for broken features) surface as GraphQL errors with the Postgres
  message attached.
- The default row limit is 20, applied in that innermost SELECT, which is
  why every collection ends at 20 rows unless you set `limit`.

## 2. How fields and tables relate

### 2.1 Field names are table names, exactly

A top level field resolves directly against a table name. There is no
singular/plural inference. Writing `event { ... }` fails with:

```
table not found: public.event
```

If you want a single object for a table, use the `@object` directive or the
`id:` shorthand (both below). Both return a JSON object instead of a list.

### 2.2 Relations come from foreign keys, and names come from FK columns

You can nest one table inside another exactly when the schema has a foreign
key connecting them. GraphJin discovers these at startup from the database
itself. There is no relation declaration in slim (no `@relationship`
directives, no config section); the FKs are the contract.

The field name you use for nesting depends on direction:

- **Parent to children (one-to-many): use the child table name.**
  From `events`, nest `registrations` and you get the list of attendees.
- **Child to parent (many-to-one): use the FK-derived name.**
  `registrations.event_id` produces the field `event` (GraphJin strips the
  `_id` suffix and singularizes). Nesting `event { title }` inside
  `registrations` gives you the event row.

```graphql
# one-to-many
query {
  events(where: {slug: {eq: "grid-scale-storage-summit"}}) {
    title
    registrations { name email }
  }
}
# {"events": [{"title": "Grid-Scale Storage Summit",
#              "registrations": [{"name": "Grace Wanjiru", ...}, ...]}]}

# many-to-one
query {
  registrations(limit: 2) {
    name
    event { title }
  }
}
```

Under the hood this is a correlated LATERAL join. The nested SELECT filters
on the parent's key, which is why the parent's primary key is automatically
carried into the inner query even when you did not select it:

```sql
SELECT "registrations"."name", "registrations"."email"
FROM "application"."registrations" AS "registrations"
WHERE (("registrations"."event_id") = ("events_0"."id"))
LIMIT 20
```

### 2.3 When two tables are joined by more than one FK: `@through`

If table A has two FKs into table B, GraphJin cannot know which relationship
you mean. It refuses to guess:

```
ambiguous relationship registrations -> users: multiple foreign keys
(user_id, created_by_user_id). Disambiguate by adding
@through(column: "user_id") on the nested selection
```

The fix is the `@through` directive on the nested field, naming the FK
column to travel by:

```graphql
query {
  users {
    registrations @through(column: "user_id") { name }
  }
}
```

### 2.4 Filtering through relations

A `where` key can name a related table. The filter then applies to the
related rows, and the outer query returns parents that have a match. This is
an EXISTS-style correlated filter:

```graphql
query {
  events(where: {registrations: {email: {eq: "mei.tanaka@asiacleanlab.jp"}}}) {
    title
  }
}
# returns the event that Mei Tanaka registered for
```

This composes with all the operators below, so
`where: {registrations: {created_at: {gte: "..."}}}` works the same way.

## 3. `where`: the filter grammar

The `where` argument is an object. Its keys are one of four kinds of thing:

1. **A column name** whose value is an operator object: `slug: {eq: "..."}`.
2. **A relation name** whose value is another filter object (previous
   section).
3. **A logical operator**: `and`, `or`, `not`.
4. **A bare value** for booleans (see 3.4).

Every comparison operator has snake_case and camelCase spellings; both are
accepted everywhere. The tables in this section list the canonical names.

### 3.1 Comparison operators

| Key | Aliases | SQL |
|---|---|---|
| `eq` | `equals` | `=` |
| `neq` | `notEquals`, `not_equals` | `!=` |
| `gt` | `greaterThan`, `greater_than` | `>` |
| `lt` | `lesserThan`, `lesser_than` | `<` |
| `gte` | `gteq`, `greaterOrEquals`, `greater_or_equals` | `>=` |
| `lte` | `lteq`, `lesserOrEquals`, `lesser_or_equals` | `<=` |
| `is_null` | `isNull` | `IS NULL` / `IS NOT NULL` |
| `ndis` | `not_distinct`, `notDistinct` | `IS NOT DISTINCT FROM` |
| `dis` | `distinct`, | `IS DISTINCT FROM` |

Two rules worth internalizing:

- **Null never goes through `neq`.** Writing `neq: null` is rejected at
  compile time with a message telling you to use `is_null: false`, because
  `!= NULL` is never true in SQL and the error would otherwise be a silent
  empty result.
- **`is_null` takes a boolean.** `is_null: true` produces `IS NULL`,
  `is_null: false` produces `IS NOT NULL`.

```graphql
query {
  standalone: events(where: {week_id: {is_null: true}}, limit: 2) { slug }
  in_week:    events(where: {week_id: {is_null: false}}, limit: 2) { slug }
}
```

### 3.2 Pattern, list, and regex operators

| Key | Aliases | SQL |
|---|---|---|
| `like` / `nlike` | | `LIKE` / `NOT LIKE` |
| `ilike` / `nilike` | `iLike` | `ILIKE` / `NOT ILIKE` |
| `similar` / `nsimilar` | | `SIMILAR TO` / `NOT SIMILAR TO` |
| `regex` / `nregex` | | `~` / `!~` |
| `iregex` / `niregex` | | `~*` / `!~*` |
| `in` / `nin` | `notIn`, `not_in` | `IN (...)` / `NOT IN (...)` |
| `contains` | | `@>` |
| `contained_in` | `containedIn` | `<@` |
| `has_in_common` | `hasInCommon` | `&&` |

On an array-typed column, `in` automatically becomes the array overlap
operator instead of `IN`.

### 3.3 Logical operators

`and` and `or` take a list of filter objects, `not` takes a single one. They
nest without limit:

```graphql
where: {and: [{format: {eq: "in_person"}}, {status: {eq: "live"}}]}
where: {or:  [{format: {eq: "virtual"}}, {participation_type: {eq: "invite_only"}}]}
where: {not: {format: {eq: "virtual"}}}
```

### 3.4 Bare values, and the null rule

Booleans compare directly, without an operator object. This reads naturally:

```graphql
query { events(where: {waitlist_open: true}, limit: 2) { slug } }
```

The same shorthand for null is **not** supported. `where: {week_id: null}`
fails with `where: [Where] invalid values for: week_id`. Always write
`{is_null: true}`. The reason: the shorthand path only knows how to compare
scalar values, and null comparison in SQL has the special IS NULL semantics,
so the compiler refuses rather than guessing.

### 3.5 The `id` shorthand

`id: <value>` is exactly `where: {id: {eq: <value>}}`, with one difference:
the result is a single JSON object rather than a list.

```graphql
query { events(id: "44444444-4444-4444-4444-000000000001") { slug } }
# {"events": {"slug": "grid-scale-storage-summit"}}
```

### 3.6 JSON and JSONB columns: read versus filter

Both `json` and `jsonb` columns come back as JSON in results, so reads are
identical. Filtering is where they diverge, because Postgres only implements
certain operators for `jsonb`:

- `has_key`, `has_key_any`, `has_key_all` render the `?`, `?|`, `?&`
  operators, which exist for **jsonb only**. On a `json` column Postgres
  rejects them (`operator does not exist: json ? unknown`).
- `contains` and `has_in_common` on jsonb render `@>` / `&&` with an array
  literal that Postgres also rejects. This is a bug in the current renderer,
  not a Postgres restriction (see the limitation table at the end).
- `contained_in` works on jsonb arrays.

What actually works today, verified:

```graphql
# has_key on a jsonb column (profiles.socials holds {"linkedin": "..."} )
query { profiles(where: {socials: {has_key: "linkedin"}}) { display_name } }

# contained_in on a jsonb array (meetings.channels = ["in_app","email"])
query ($v: JSON!) {
  meetings(where: {channels: {contained_in: $v}}) { status }
}
# vars: {"v": ["in_app", "email"]}

# contains on a true SQL array column (text[]) works fine
query { audit_logs(where: {changed_fields: {contains: ["status"]}}) { id } }
```

Rule of thumb: when a JSON filter misbehaves, check the column type first.
`json` and `jsonb` look identical in results and behave very differently in
filters.

### 3.7 Full-text search

The root-level `search:` argument runs a Postgres tsquery against the
columns the engine has been told are full-text searchable (configured per
table and column, for example `events.description`).

Two things to know:

1. **The value must be a variable.** An inline string is treated as a
   variable *name* and fails as `required variable 'storage' of type 'text'
   must be set`. Pass it like this:

   ```graphql
   # vars: {"q": "storage"}
   query ($q: String!) { events(search: $q) { slug title } }
   ```

2. If no full-text column is configured for the table, the compiler fails
   early: `search: no tsvector column defined on table 'events'`.

## 4. Ordering, pagination, distinct

### 4.1 `order_by`

The object form is the only accepted form. Each key is a column, each value
a direction. Six directions exist because SQL null ordering is configurable:

```
asc, desc, asc_nulls_first, desc_nulls_first, asc_nulls_last, desc_nulls_last
```

```graphql
query { events(order_by: {capacity: desc}, limit: 2) { slug capacity } }
query { events(order_by: {status: asc, start_time: desc}, limit: 2) { slug } }
```

A list form (`order_by: [{capacity: desc}]`) is rejected: `value for
argument 'order_by' must be a object or a variable`.

You can order by an aggregate. This forces the query into GROUP BY mode even
if the selection has no other aggregate:

```graphql
query { events(order_by: {count_id: desc}, limit: 2) { slug count_id } }
```

`count_id` here is the aggregate COUNT over `id` (see section 6 for the
naming rules).

### 4.2 Keyset pagination: first, last, after, before

Because the response is built by the database, the cursor is too: when you
use `first`/`last`, each result object carries a `<root>_cursor` key next to
the data. The cursor encodes the order-column values of the last (or first)
row and is encrypted, so it is opaque to the client.

```graphql
# page 1
query { events(first: 2, order_by: {slug: asc}) { slug } }
# {"events": [{"slug": "clean-capital-pitch-day"},
#             {"slug": "green-hydrogen-webinar"}],
#  "events_cursor": "__gj-enc:..."}

# page 2: feed the cursor back
query ($cursor: String!) {
  events(first: 2, after: $cursor, order_by: {slug: asc}) { slug }
}
# {"events": [{"slug": "grid-scale-storage-summit"},
#             {"slug": "investor-speed-matchmaking"}], ...}
```

`last: N` with `before: $cursor` walks backwards the same way. Two rules:

- An `order_by` is what makes the cursor meaningful. Without it the row
  order is whatever Postgres returns and the cursor encodes nothing useful.
- Keep the ordering identical across pages. The cursor is positional within
  the ordering; changing the sort between pages corrupts the walk.

Plain `limit`/`offset` pagination also exists and composes with filters and
ordering. The default limit is 20.

### 4.3 `distinct`

`distinct` takes a list of columns and renders DISTINCT ON:

```graphql
query { events(distinct: [format]) { format } }
# {"events": [{"format": "in_person"}, {"format": "virtual"}, {"format": "hybrid"}]}
```

## 5. Aggregates and expressions

This is the area with the most history, so it pays to be explicit about what
each spelling actually does. All of these were verified against real data.

### 5.1 What works

| Spelling | Example | What happens |
|---|---|---|
| `count_id` (suffix) | `events { count_id }` | `COUNT(id)`, one global row |
| grouped aggregate | `events { status count_id }` | one row per `status` group |
| function + `column:` arg | `events { sum(column: "capacity") }` | global `SUM(capacity)`, one row |
| expression aggregate | `events { total: sum(expr: {col: "capacity"}) }` | global, one row |
| nested aggregate | `events { slug registrations { count_id } }` | one child row per parent |

The two spellings that look like aggregates but are not:

- **Prefix alias** (`sum: capacity`): GraphQL sees an alias plus a plain
  field. The compiler does not interpret the alias as a function, so the
  SQL is `"events"."capacity" AS "sum"`. You get the raw column back under
  a misleading name.
- **Suffix on a keyed table** (`capacity_sum: capacity`): the suffix naming
  convention is legacy and does not engage here; it also compiles to a plain
  aliased column (`"events"."capacity" AS "capacity_sum"`), returning one
  row per event with the event's own value.

The reason the suffix form works for `count_id` and not for `capacity_sum`
is that COUNT has no meaningful plain-column interpretation, so the compiler
resolves it as an aggregate, while SUM over a column of a keyed table
resolves to the column itself. Do not rely on suffix naming; use the forms
in the table above.

```graphql
# grouped counts
query { events { status count_id } }
# {"events": [{"status": "completed", "count_id": 1},
#             {"status": "live", "count_id": 4},
#             {"status": "draft", "count_id": 5}]}

# global sums
query { events { sum(column: "capacity") } }
# {"events": [{"sum": 2390}]}
```

### 5.2 Nested aggregates

An aggregate-only nested select collapses to one child row per parent. This
is the supported shape for "count of children":

```graphql
query {
  events(order_by: {slug: asc}, limit: 2) {
    slug
    registrations { count_id }
  }
}
# {"events": [{"slug": "clean-capital-pitch-day",
#              "registrations": [{"count_id": 0}]}, ...]}
```

There is no `registrations_agg` or `registrations_aggregate` pseudo table in
slim; those names fail with `table not found`.

### 5.3 Expression aggregates (`expr:`)

`expr:` on an aggregate builds a typed expression tree. The operators are
`add`, `sub`, `mul`, `div`, `mod`, `neg`, `coalesce`, `nullif`, `case`,
`cast`; leaves are `{col: "..."}` references and literals.

```graphql
query {
  events {
    half:  sum(expr: {div: [{col: "capacity"}, 2]})
    plus1: sum(expr: {add: [{col: "capacity"}, 1]})
  }
}
# {"events": [{"half": 1195, "plus1": 2400}]}
```

### 5.4 Aggregates are not filter operands

You cannot write `{gte: {max: capacity}}` to filter on an aggregate result.
The compiler rejects it with an instructive message: query the aggregate
first, then use the returned literal in a second query. Aggregate results
are values, and `where` sees columns, not result sets.

## 6. Mutations

Mutations carry a payload plus (for update/upsert/delete) a `where`. You
select what should come back from the affected rows, and the engine renders
it as a Postgres RETURNING clause, shaped into the same JSON response form
as queries.

The payload may be inline or a variable. For variable payloads the engine
uses a `json_to_record` CTE: the payload JSON lands in `$1` and each column
is extracted with its exact database type, which is why variable payloads
type-check cleanly against the schema:

```sql
WITH "_sg_input" AS (SELECT $1 :: json AS j),
"form_fields" AS (
  INSERT INTO "application"."form_fields" ("options", "event_id", "field_type", "label")
  SELECT "t"."options" :: json, "t"."event_id" :: uuid,
         "t"."field_type" :: application.form_field_type, "t"."label" :: text
  FROM "_sg_input" i, json_to_record(i.j) as t("options" json, "event_id" uuid, ...)
  RETURNING "application"."form_fields".*
)
SELECT ...
```

### 6.1 insert

```graphql
mutation {
  form_fields(insert: {event_id: "...", label: "Probe field", field_type: short_text}) {
    id label
  }
}
```

Bulk insert is the same payload as a JSON array of objects. Nested inserts
work in one direction: a one-to-many **list** under an insert creates the
children and binds their FK to the new parent's id:

```graphql
mutation ($input: eventsInput!) {
  events(insert: $input) { id slug registrations { name } }
}
# vars: {"input": {"slug": "...", "title": "...", "host_user_id": "...",
#        "registrations": [{"name": "Probe Reg", "email": "probe@example.org"}]}}
```

A single nested object child (one-to-one) is also supported. Nested lists
under update/upsert are not (see the limitations table).

### 6.2 JSON columns in payloads

Send real JSON. Since the issue #5 fix, a genuine array or object in the
payload is recognized as a column value for json/jsonb columns and stored
correctly:

```graphql
# vars: {"input": {..., "options": ["Newsletter", "Twitter / X"]}}
mutation ($input: form_fieldsInput!) {
  form_fields(insert: $input) { id label options }
}
# {"form_fields": [{"id": "...", "label": "...",
#                   "options": ["Newsletter", "Twitter / X"]}]}
```

Historical note: before the fix, real arrays failed with
`from edge not found` because the compiler treated every array/object payload
key as a nested relation attempt. The old workaround of stringifying JSON
into a text field still type-checks, but it stores a JSON *string*
(`"[\"XS\",\"S\"]"`) instead of an array, so do not use it with real json
columns. If the value on the wire is a string, that is what gets stored.

### 6.3 update and delete

`where` is required (the engine refuses an unbounded UPDATE/DELETE). The
payload may also contain its own `where:` object instead of the field-level
one.

```graphql
mutation {
  form_fields(update: {label: "Renamed"}, where: {id: {eq: "..."}}) { id label }
}

mutation {
  form_fields(delete: true, where: {id: {eq: "..."}}) { id }
}
```

Note `delete: true` is a marker, not a payload: the selected fields come
back from the deleted rows.

### 6.4 on_conflict: get

Insert-or-return-existing. The dialect implements exactly one conflict
action in slim:

```graphql
mutation {
  users(insert: {id: "...", first_name: "Amara", ...}, on_conflict: get) {
    id first_name
  }
}
```

Restrictions, all enforced with clear errors: exactly one root insert, no
bulk arrays, no nested payloads, and the payload must supply a unique or
primary key. `on_conflict: {update: ...}` is rejected with
`value for 'on_conflict' must be 'get'`.

### 6.5 Multiple root mutations

Several root mutates in one operation are supported. The compiler orders
them topologically, so parents are written before children that reference
them:

```graphql
mutation {
  a: form_fields(insert: {...}) { id }
  b: form_fields(insert: {...}) { id }
}
```

### 6.6 Known-broken mutation shapes

Two mutation shapes render structurally invalid SQL on Postgres today. They
are kept as `expect-error` examples in `tests/dialect/mutations/` so the day
they are fixed, the corpus will show it:

- **upsert**: renders `ON CONFLICT` outside the writable CTE.
  Fails with `syntax error at or near "ON"`.
- **connect / disconnect** of one-to-many children under an update: renders
  an empty `SET` clause and a dangling `AND`. Fails with
  `syntax error at or near "FROM"`.

## 7. Directives

Directives are processed at compile time; most of them change the shape of
the query itself rather than the protocol.

| Directive | Where | What it does |
|---|---|---|
| `@skip(if: $v)` / `@include(if: $v)` | field, selector | standard GraphQL conditional selection, verified |
| `@object` | selector | singular result object, implies `limit: 1` |
| `@through(column: ...)` | nested field | picks the FK for an ambiguous path |
| `@schema(name:)` / `@database(name:)` | selector | targets a schema or database |
| `@cacheControl(maxAge:, scope:)` | operation | sets the response Cache-Control header |
| `@rank`, `@denseRank`, `@rowNumber` | field | ranking window functions |
| `@running`, `@moving` | field | running / moving aggregates |
| `@previous`, `@next`, `@first`, `@last` | field | offset windows |
| `@window` | field | removed; the analytics directives replaced it |
| `@validate` / `@constraint` | operation | variable validation; see below |

Verified basics:

```graphql
# vars: {"s": true}
query ($s: Boolean!) { events(limit: 1) { slug title @skip(if: $s) } }
# title comes back null because the field was skipped

query @cacheControl(maxAge: 60) { events(limit: 1) { slug } }
```

`@validate` / `@constraint` exist in the frontend with a full validator
registry (`required`, `format`, length and range validators), but the
registry is not wired through `core.NewGraphJin`. Only an application that
constructs the graphql compiler directly with
`graphql.Config{Validators: ...}` can use them. Through the engine they fail
with `unknown argument`.

## 8. Analytics windows

The analytics directives turn a field into a window function over the
result set. They require `analytics_mode: true` in the engine config;
without it they silently compile to the plain column value, which is the
most confusing failure mode they have, so check the config first.

The window value renders **in place of the field value, under the field's
own name**. To keep both the raw value and the window, alias the field:

```graphql
query {
  events(unrestricted: true, order_by: {slug: asc}, limit: 4) {
    slug
    cap: capacity @rank(order: "desc")
  }
}
# {"events": [{"cap": 9, "slug": "clean-capital-pitch-day"}, ...]}
```

Note the rank is computed over the whole table before `limit` applies, which
is why the numbers are not 1..4.

### 8.1 `@running` and `@moving`

`@running` aggregates cumulatively, `@moving` aggregates over a sliding
frame of `rows` rows. Both need:

- `aggregate`: one of `sum`, `avg`, `count`, `min`, `max` (required)
- `by`: partition column (the running total restarts per partition)
- `orderBy: {col: asc|desc}` (or `order: asc|desc` when the order column is
  the field itself)
- `rows: N` (required for `@moving`, rejected for `@running`)

```graphql
query {
  events(unrestricted: true, where: {status: {eq: "live"}}, order_by: {capacity: asc}) {
    slug
    capacity
    run: capacity @running(aggregate: "sum", by: "status", orderBy: {capacity: asc})
  }
}
# run: 40, 160, 410, 910   (cumulative SUM of capacity)

query {
  events(unrestricted: true, where: {status: {eq: "live"}}, order_by: {capacity: asc}) {
    slug
    capacity
    mv: capacity @moving(aggregate: "avg", by: "status", orderBy: {capacity: asc}, rows: 2)
  }
}
# mv: 40, 80, 185, 375    (2-row moving average)
```

A caution the examples teach: partitioning `by: "slug"` on a table where
slug is unique puts every row in its own partition, so every "running" value
equals the row's own value. Partition by something that repeats.

## 9. The OLAP partition gate

With `analytics_mode: true`, the engine protects large tables from full
scans. If a table has a temporal column (`created_at`, `updated_at`,
`ingested_at` are the built-in candidates), queries against it must filter
on that column:

```
table "events" requires a filter on temporal column "created_at".
Add one of: where: { created_at: { gte: "<date>" } };
or pass unrestricted: true to override.
```

The two exits are exactly what the message says:

```graphql
query { events(where: {created_at: {gte: "2026-01-01"}}, limit: 2) { slug } }
query { events(unrestricted: true, limit: 1) { slug } }
```

Details that matter: without analytics mode the gate applies only to tables
with an explicit partition key (warehouse tables); `unrestricted` is per
selector, so a nested selector that triggers the gate needs its own
`unrestricted: true`.

## 10. Recursive queries: `find:`

A table with a self-FK (like `taxonomy_terms.parent_id`) can walk its own
hierarchy. The rules:

- nest the table **by its own name**, not by the FK-derived name
  (`taxonomy_terms` inside `taxonomy_terms`, not `parent`);
- pass `find: "parents"` or `find: "children"` to choose the direction
  (singular `parent`/`child` are rejected);
- the outer select must include the primary key (`id`), because the
  recursive CTE joins on it;
- `find:` only works on a nested selector, never at the top level.

```graphql
query {
  taxonomy_terms(unrestricted: true, where: {slug: {eq: "renewable-energy"}}) {
    id
    slug
    taxonomy_terms(find: "children", unrestricted: true) { slug }
  }
}
# {"taxonomy_terms": [{"id": "...", "slug": "renewable-energy",
#   "taxonomy_terms": [{"slug": "energy-storage"}]}]}
```

## 11. Introspection

Standard GraphQL introspection runs only when the operation is literally
named `IntrospectionQuery`. The engine checks the operation name and serves
a cached, generated introspection document before any table resolution
happens. Using `__schema` inside any other operation fails with
`table not found: public.__schema`, because the field name goes down the
normal table-resolution path.

```graphql
query IntrospectionQuery { __schema { queryType { name } } }
```

## 12. Limitations at a glance

| Thing | Status | What you get instead |
|---|---|---|
| singular top-level names (`event`) | not supported | table names plus `@object` or `id:` |
| variable defaults (`$v: Int = 5`) | not supported | always supply every variable |
| `null` direct value in where | rejected | `is_null: true` / `false` |
| `order_by` list form | rejected | object form only |
| `contains` on jsonb | broken SQL | `contained_in` on jsonb, `contains` on `text[]` |
| `has_in_common` on jsonb | broken SQL | none today |
| upsert on Postgres | broken SQL | `on_conflict: get`, or update by unique key |
| connect / disconnect on update | broken SQL | explicit update of the child rows |
| `on_conflict: {update: ...}` | rejected | only `on_conflict: get` |
| aggregates as where operands | rejected | two-stage query |
| `registrations_agg` style tables | do not exist | aggregate-only nested select |
| `@validate` / `@constraint` via the engine | not wired | direct graphql compiler construction |
| `__schema` outside `IntrospectionQuery` | fails | name the operation `IntrospectionQuery` |
| JSON-path filter operators | not exposed | none today |

## 13. Selector argument reference

| Argument | Applies to | Value |
|---|---|---|
| `where` | any selector | filter object (section 3) |
| `id` | any selector | value; implies singular result |
| `search` | root | variable holding tsquery text |
| `order_by` (`orderBy`, `order`) | any | `{col: asc|desc|...}` |
| `limit`, `offset` | any | integers |
| `first`, `last`, `after`, `before` | any | integer; cursor variable for after/before |
| `distinct` (`distinct_on`, `distinctOn`) | any | `[col, ...]` |
| `find` | nested self-reference | `parents` or `children` |
| `unrestricted` | any | `true` unlocks the partition gate |
| `insert`, `update`, `upsert`, `delete` | mutation root | payload object or variable |
| `on_conflict` (`onConflict`) | insert root | `get` |
| `args` | DB function fields | `{name: value, ...}` |

Operator names accept snake_case and camelCase aliases everywhere;
`orderBy`/`order_by`/`order` and `distinct`/`distinct_on`/`distinctOn`
accept all their spellings too.
