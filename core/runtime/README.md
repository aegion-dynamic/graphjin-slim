# runtime

`runtime` provides execution resilience, field encryption, query error diagnostics, and request tracing utilities for GraphJin.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/runtime
```

## Features

- **Field-Level Encryption (`crypt.go`)**: AES-GCM ciphertext encryption and base64 cursor encoding for sensitive query fields and subscription cursors.
- **Retry Mechanism (`retry.go`)**: Exponential backoff retry loops for transient database lock contention and connection drops (`RetryOperationForDB`).
- **Error Repair Diagnostics (`repair.go`)**: Intelligent query diagnosis and hint suggestions for common GraphQL errors (table not found, null comparison syntax, distinct join shapes).
- **Distributed Tracing (`trace.go`)**: Request tracer interfaces (`Tracer`, `Spaner`) and no-op default tracer.

## Key APIs

```go
func EncryptValues(data, encPrefix, decPrefix, nonce []byte, key [32]byte) ([]byte, error)
func DecryptValues(data, prefix []byte, key [32]byte) ([]byte, error)
func RetryOperationForDB(c context.Context, dbType string, fn func() error) error
func BuildGraphJinErrorRepair(query, errorMsg string) ErrorRepair
func NewTracer() Tracer
```
