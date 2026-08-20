# assert

`assert` provides lightweight test assertion helpers for unit tests within the GraphJin compiler.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/assert
```

## Overview

A minimal, dependency-free set of test assertions tailored for GraphJin compiler test suites.

## Functions

```go
func Equals(t *testing.T, exp, got interface{})
func Empty(t *testing.T, got interface{})
func NoError(t *testing.T, err error)
func NoErrorFatal(t *testing.T, err error)
```
