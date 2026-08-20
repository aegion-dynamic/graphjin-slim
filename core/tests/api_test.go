package core_test

import (
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
)

func TestPublicAPIFacade(t *testing.T) {
	gj, err := core.NewTestGraphJin(map[string]string{
		"default": "default",
	})
	if err != nil {
		t.Fatalf("failed to create test engine: %v", err)
	}
	defer gj.Close()

	if gj.DefaultDatabase() != "default" {
		t.Errorf("got default db %q, want %q", gj.DefaultDatabase(), "default")
	}

	names := gj.DatabaseNames()
	if len(names) != 1 || names[0] != "default" {
		t.Errorf("got database names %v, want [default]", names)
	}
}
