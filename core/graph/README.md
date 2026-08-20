# graph

`graph` provides a high-performance, low-allocation GraphQL lexer, parser, and AST syntax tree representation.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/graph
```

## Overview

The parser parses raw GraphQL query and mutation strings directly into typed AST nodes without creating unnecessary string allocations.

## Capabilities

- **Zero-Allocation Tokenizer**: Fast lexer scanning GraphQL operations, fields, arguments, directives, and variable definitions.
- **AST Representation**: Compact struct representations for operations, selections, inline fragments, arguments, and directives.
- **Fragment Extraction**: Parses and indexes reusable fragments across multiple operations.
- **Directive Parsing**: Supports standard GraphQL directives (`@include`, `@skip`) as well as GraphJin-specific directives (`@schema`, `@database`, `@through`, `@running`, `@moving`).

## Key Types & Functions

```go
type Operation struct {
    Type       OpType
    Name       string
    Args       []Arg
    Directives []Directive
    Fields     []Field
    ...
}

func Parse(gql []byte) (*Operation, error)
func ParseFragments(gql []byte) ([]Fragment, error)
```
