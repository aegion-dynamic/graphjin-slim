package engine

// Register the graphql language for engine tests: engines bind languages
// exclusively through the langadapter registry.
import (
	_ "github.com/aegion-dynamic/graphjin-slim/core/v3/lang/graphql"
)
