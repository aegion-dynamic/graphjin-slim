# serv/http

`http` defines the routing registry and standard HTTP API endpoints for the GraphJin service.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/serv/v3/http
```

## Overview

Registers standard HTTP handlers for:
- `/api/v1/graphql`: GraphQL query execution endpoint.
- `/api/v1/rest/*`: REST-mapped query execution endpoint.
- `/health`: Service health check.

## Key Types & Functions

```go
type Handlers struct {
    GraphQL http.Handler
    REST    http.Handler
    WebUI   http.Handler
    WebUIOn bool
}

func Register(mux Mux, h Handlers) Mux
```
