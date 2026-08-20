# serv/config

`config` handles configuration parsing, environment variable overrides, mode normalization, and default values for the GraphJin service.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/serv/v3/config
```

## Overview

The package layers HTTP service settings (ports, CORS, TLS, routes) onto the underlying `core.Config` and provides environment variable bindings (prefixed with `GJ_`).

## Features

- **Environment Binding (`environment.go`)**: Binds environment variables to structured configuration fields.
- **Mode Resolution (`mode.go`)**: Normalizes runtime modes (`dev`, `prod`).
- **Defaults (`defaults.go`)**: Sets default HTTP ports, timeouts, CORS headers, and connection parameters.

## Key Types

```go
type Config struct {
    Serv ConfigServ
    Core core.Config
    ...
}
```
