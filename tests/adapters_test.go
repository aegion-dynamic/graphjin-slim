package tests_test

// Adapter imports: this integration suite consumes the engines through the
// serv/database seam exactly like a real application would.

import (
	_ "github.com/aegion-dynamic/graphjin-slim/postgres/v3"
	_ "github.com/aegion-dynamic/graphjin-slim/sqlite/v3"
)
