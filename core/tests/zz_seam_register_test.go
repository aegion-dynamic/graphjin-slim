package core_test

// Register the graphql language for engine construction in these tests.
// Engines bind languages exclusively through the langadapter registry.
import (
	_ "github.com/aegion-dynamic/graphjin-slim/core/v3/lang/graphql"
)
