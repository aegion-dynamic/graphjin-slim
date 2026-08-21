# openapi

OpenAPI 3.0 specification generator for GraphJin services. Saved named
queries become REST paths; discovered tables become reusable schema
components.

The generator is pure: it consumes a plain snapshot produced by the engine
(`core.OpenAPIInputs`) and renders a document. It depends only on public
compiler packages (`qcode`, `sdata`, `schema`, `graph`, `allow`) — no engine
internals, no database access, no network.

## Usage

```go
import "github.com/aegion-dynamic/graphjin-slim/openapi/v3"

gjs, err := serv.NewGraphJinService(conf,
    serv.OptionSetOpenAPI(openapi.Generator(openapi.Config{Title: "My API"})),
)
```

This serves an OpenAPI document at `/api/v1/openapi.json`. Public handlers:
`HttpService.OpenAPI()` and `HttpService.OpenAPIWithNS(ns)` are available for
custom routers.

Saved query variables become endpoint defaults: a caller omitting a
variable receives the value stored in `<name>.json`, and an explicit
parameter always wins. This applies identically in dev and production
(the queries directory ships with the deployment).

Setting `openapi_specs_dir` in the service config writes the spec to disk at
startup (as `openapi.json`, or `<ns>.openapi.json` for namespaced services)
for SDK codegen pipelines. Requires a registered generator; failures are
logged and do not block startup.

## Behavior

- Queries that fail to compile are skipped.
- In production-security mode the allow list is the complete set of
  executable queries, so generated specs never expose more than production
  allows.
- HTTP methods map from operation type: queries get GET+POST; inserts POST;
  updates/upserts PUT+POST; deletes DELETE+POST.
- With multiple databases attached, non-default databases' tables are
  prefixed in component names to avoid collisions.

## API

- `Generate(inputs core.OpenAPIInputs, cfg Config) (*Document, error)` — pure
  rendering from a snapshot.
- `GenerateJSON(gj *core.GraphJin, ns *string, cfg Config) (json.RawMessage, error)`
  — collects inputs from the engine and renders indented JSON.
- `Generator(cfg Config)` — returns a `serv.OptionSetOpenAPI`-compatible
  function.
