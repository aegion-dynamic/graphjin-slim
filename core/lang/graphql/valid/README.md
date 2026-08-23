# valid

`valid` implements variable constraint validators and input validation directives for GraphQL operations.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/valid
```

## Overview

The `valid` package validates query variables against declarative constraints defined via GraphQL directives (e.g. `@constraint`, `@validate`).

## Built-in Validators

- **Existence & Requirement**: `required`, `requiredIf`, `requiredUnless`, `requiredWith`, `requiredWithout`.
- **Value & Boundary**: `min`, `max`, `equals`, `notEquals`, `oneOf`, `greaterThan`, `lessThan`, `greaterThanOrEquals`, `lessThanOrEquals`.
- **Cross-Field Comparisons**: `equalsField`, `notEqualsField`, `greaterThanField`, `lessThanField`.
- **Format Validation**: `format` (e.g. email, UUID, URL, regex).

## Key Registry

```go
var Validators = map[string]qcode.Validator
```
