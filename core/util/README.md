# util

`util` provides specialized, low-allocation data structures and graph algorithms used across the compiler.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/core/v3/util
```

## Overview

A collection of optimized helper structures designed for compiler performance:

- **Stacks (`stackinf.go`, `stackint32.go`)**: Fast unbounded and `int32` slice-backed stacks for recursive traversal.
- **Priority Queue (`heap.go`)**: Binary min/max heap.
- **Graph Traversal (`graph.go`)**: Graph structures and pathfinding algorithms used for relationship resolution.
