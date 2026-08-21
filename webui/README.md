# webui

Embedded web console for a GraphJin service, shipped as its own Go module so
applications that do not import it keep zero UI assets in their binary.

## Usage

```go
import "github.com/aegion-dynamic/graphjin-slim/webui/v3"

gjs, err := serv.NewGraphJinService(conf,
    serv.OptionSetWebUI(webui.Handler),
)
```

The service mounts `webui.Handler` at `/` when config enables it (`web_ui`,
default on in dev mode). On a bare request to `/` the handler redirects to
`/?endpoint=<graphql path>`; the single-page app reads that parameter to find
the GraphQL endpoint.

The console discovers the schema through introspection and offers root-field
browsing, query editing, and execution against `/api/v1/graphql`.

## Assets

The compiled console bundle under `assets/build` is committed, so `go get`
consumers never need Node.js or npm — building the Go module is enough.

To change the UI:

```bash
cd console
npm install        # once per clone
npm run dev        # dev server (Vite+, HMR)
npm run check      # format + lint + type check (Oxfmt/Oxlint/tsgo)
npm run build      # emits into ../assets/build, then commit the result
```

Tooling is Vite+ (`vp`, wrapping Vite/Rolldown/Oxlint/Oxfmt) configured in
`console/vite.config.ts`; formatting rules live there (`fmt:` section).
Component sources are shadcn-style copies from the Base UI variant of
[shadcn/ui](https://github.com/shadcn-ui/ui) in `console/src/components/ui`,
styled with Tailwind v4 tokens defined in `console/src/index.css`.
