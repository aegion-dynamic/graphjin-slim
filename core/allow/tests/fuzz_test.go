package allow_test

import (
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
)

func TestFuzzCrashers(t *testing.T) {
	crashers := []string{
		"query",
		"q",
		"que",
	}

	for _, f := range crashers {
		_, _ = graph.FastParse(f)
	}
}
