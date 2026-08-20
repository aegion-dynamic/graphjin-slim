# serv/lifecycle

`lifecycle` provides HTTP server startup and graceful shutdown lifecycle management.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/serv/v3/lifecycle
```

## Overview

Wraps `http.Server` with safe graceful shutdown routines and traps operating system interrupt signals (`SIGINT`, `SIGTERM`).

## Key Functions

```go
func NewServer(addr string, handler http.Handler) *http.Server
func WatchSignals(log *zap.SugaredLogger, shutdown func() error)
```
