# serv/etags

`etags` provides HTTP ETag middleware and cache validation (`If-None-Match`, `If-Modified-Since`) for GraphJin HTTP endpoints.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/serv/v3/etags
```

## Overview

Intercepts HTTP responses, generates or inspects ETags, and returns `304 Not Modified` when cached client data is fresh.

## Key Functions

```go
func Handler(h http.Handler, weak bool) http.Handler
func Fresh(reqHeader, resHeader http.Header) bool
```
