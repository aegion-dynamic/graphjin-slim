package allow

import (
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/graph"
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
