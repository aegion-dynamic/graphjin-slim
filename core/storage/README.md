# storage

`storage` defines the filesystem abstraction layer used by GraphJin for reading configuration, scripts, and allow-list assets.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/storage
```

## Overview

GraphJin uses `storage.FS` to load allow-lists, query files, and configuration without depending strictly on local disk paths.

## Implementations

- **`osFS`**: Standard operating system filesystem provider (`NewOsFS(basePath)`).
- Custom `fs.FS` adapters can be supplied via `core.OptionSetFS`.

## Key Interface

```go
type FS interface {
    Get(path string) ([]byte, error)
    Put(path string, data []byte) error
    Delete(path string) error
    Exists(path string) (bool, error)
    List(path string) ([]string, error)
}
```
