# serv/logging

`logging` initializes structured zap loggers for development and production modes.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/serv/v3/logging
```

## Overview

Configures fast, zero-allocation structured logging using `go.uber.org/zap`:
- **Development**: Console-formatted colored output.
- **Production**: Structured JSON log encoder with standard log levels (`debug`, `info`, `warn`, `error`).

## Key Functions

```go
func New(production bool, logLevel string) (*zap.SugaredLogger, *zap.Logger, error)
```
