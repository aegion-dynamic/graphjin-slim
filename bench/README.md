# bench

End-to-end scenario harness. Spins up real GraphJin services over plain
SQLite files on loopback and drives them black-box over HTTP — the same
surfaces users hit, asserted the way clients see them.

```bash
go run . e2e                 # full correctness suite (~5s)
go run . e2e -run fk         # scenarios matching a name
go run . e2e -variant prod   # production-security behaviors only
go run . e2e -extreme        # depth 20 / 50k rows / 200 clients
go run . load -total 2000    # throughput + latency percentiles
```

Exit code is CI-ready: 0 = all green, 1 = any failure with a named
scenario and a reproducible request.

## Coverage

| Scenario | What it proves |
| --- | --- |
| `graphql_basic` | reads, orderBy, FK-nested products, aggregates (`count_id`, `sum_price`), singular `usersByID` root |
| `graphql_mutations` | insert mutations + filtered read-back |
| `fk_depth` | FK chains queried at depths 2 / 4 / 8 / max with exact-value stitching at every level |
| `fk_recursion_guard` | queries past the seeded schema fail fast as GraphQL errors, never hang |
| `rest_saved_queries` | full saved-query lifecycle: save → `.gql`+`.json` artifacts → list/load → REST by-name (GET params, POST body) → loud missing-variable error → delete |
| `strict_vars` | typed GET parameters (Int/String coercion); bad types rejected |
| `apq` | automatic persisted queries: store by hash, replay hash-only |
| `openapi_spec` | served spec matches live routes; typed params only (no untyped blob); startup disk export byte-identical; no subscription type |
| `introspection` | valid introspection; reverse relations present in both directions; no Subscription anywhere |
| `prodsec` [prod] | ad-hoc queries rejected; saved query runs with explicit vars; bare call errors loudly; named-op execution through GraphQL works |
| `edge_cases` | unicode round-trip, empty-set shapes, malformed bodies, unknown-field errors |

Variants: every scenario runs in dev mode; `prodsec` additionally runs
against a production-mode service.

## Measured results (reference laptop)

```
e2e (default budgets):  11 passed, 0 failed   (~5s wall)
e2e (-extreme):         depth-20 chains green, 9/11 non-fk + 2/2 fk
load  (1000 reqs):      6299 rps · p50 6.3ms · p95 19.8ms · max 48.1ms
                        mixed traffic: 60% REST by-name, 40% GraphQL reads
```

Latency numbers are informational (no hard gates); correctness failures
always fail the run.

## Safety budgets

Default runs are sized to stay polite on a laptop:

| Guard | Default | `-extreme` |
| --- | --- | --- |
| Max FK chain depth queried | 12 | 20 |
| Rows seeded per scenario | ≤ 5k | ≤ 50k |
| Concurrent clients (load tier) | 50 | 200 |
| Per-scenario deadline | 15s (+10s grace) | 60s |

## Implementation notes

- **Black-box**: scenarios speak HTTP only. Internal packages are never
  imported by tests, so route registration, middleware and serialization
  drift all fail here.
- **Plain SQLite, one driver**: services open files through serv's own
  registered `"sqlite"` driver (mattn adapter, SQLCipher-capable — plain
  files without keys work unchanged). The seeding ORM (bun) uses a shim
  over the same modernc core, avoiding database/sql driver-name
  collisions. CGO is required since the mattn migration.
- **Name normalization**: GraphJin rewrites digit-suffixed table names
  (`c1` → `c_1`) between introspection and SQL, so chain fixtures use
  pure-alpha names (`ta`, `tb`, … skipping `to`). See
  `harness.ChainTable`.
- **Isolation**: each scenario gets a temp working directory that doubles
  as the service's cwd — `config/queries/` seeds and `bench.db` live and
  die with the run.
