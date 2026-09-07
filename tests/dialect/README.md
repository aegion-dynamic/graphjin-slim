# tests/dialect - the GraphJin GraphQL dialect corpus

A runnable corpus of GraphQL examples that documents and verifies the exact
GraphQL dialect graphjin-slim understands. See
`.github/docs/graphql-dialect.md` for the full explanation; every example in
that document is backed by a `.gql` file here.

## Layout

- `basics/`, `nesting/`, `filters/`, `ordering/`, `aggregation/`,
  `pagination/`, `directives/`, `mutations/`, `advanced/` - one `.gql` file
  per example, filename prefixes control run order.
- `executor/` - the Go program that reads the `.gql` files and runs them.
- `schema/` - the Atlas HCL schema this corpus runs on
  (`backend/schema-application.hcl` + `schema-nimbus.hcl`, a standalone copy
  of the application schema, with one change: `form_fields.options` is a
  real `json` column), plus `seed.sql` with the sample data the examples
  reference by id.

## Header directives in .gql files

```graphql
# vars: {"slug": "..."}        variables passed to the operation
# use_cursor: <file-name>      bind $cursor to a cursor captured earlier
# analytics: on                run against the analytics_mode engine
# expect-error: <substring>    example must fail with this error (limitation)
# note: <text>                 printed with the result
```

Every example runs inside a transaction that is always rolled back, so
mutations never change the seed data.

## Running

```bash
tests/dialect/up.sh                 # docker + schema + seed + full corpus run
tests/dialect/up.sh --no-fresh      # keep existing data, just run

# or run the executor directly against an already-seeded database:
GJ_TEST_PG='postgres://postgres:postgres@localhost:5433/gjtest?sslmode=disable' \
  go run ./tests/dialect/executor -dir tests/dialect

# subset:
GJ_TEST_PG='...' go run ./tests/dialect/executor -dir tests/dialect -filter filters/
```

The executor exits non-zero when any example fails, so the corpus can gate
CI.
