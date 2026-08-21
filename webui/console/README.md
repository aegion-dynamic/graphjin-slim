# GraphJin Console

Small React (Vite + TypeScript) single-page console for a GraphJin service.
No UI framework; the production bundle is a few hundred KB gzipped.

## Features

- Discovers the GraphQL endpoint from the `?endpoint=` query parameter that
  `webui.Handler` appends on redirect.
- Introspects the schema and lists root query/mutation fields in tabs.
- Clicking a field inserts an editable query skeleton.
- Runs queries (`Ctrl+Enter`) against `/api/v1/graphql` and pretty-prints
  the JSON result.

## Development

```bash
npm ci
npm run dev          # vite dev server; configure endpoint via ?endpoint=
```

## Production build

```bash
npm run build        # emits into ../assets/build (embedded by the Go module)
```

The Go module embeds `assets/build` via `go:embed`. A placeholder
`index.html` is committed so the module always compiles before the first
build; running `npm run build` replaces it.
