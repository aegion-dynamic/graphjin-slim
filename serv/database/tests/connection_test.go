package database_test

import (
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
)

func TestConnectionDefaultsToPostgres(t *testing.T) {
	opts := database.Options{Config: database.Config{Host: "db.example", Port: 5432, User: "app"}}
	sc := database.SourceConfigFor(opts)
	if sc.Type != "postgres" {
		t.Fatalf("type = %q, want postgres", sc.Type)
	}
}

func TestConnectionUsesSQLitePath(t *testing.T) {
	opts := database.Options{Config: database.Config{Type: "sqlite", Path: ":memory:"}}
	sc := database.SourceConfigFor(opts)
	if sc.Type != "sqlite" {
		t.Fatalf("type = %q, want sqlite", sc.Type)
	}
	if sc.Flat.Path != ":memory:" {
		t.Fatalf("flat path = %q, want :memory:", sc.Flat.Path)
	}
}

func TestUnknownAdapterLoudError(t *testing.T) {
	opts := database.Options{Config: database.Config{Type: "oracle"}}
	_, err := database.Open(opts)
	if err == nil {
		t.Fatal("expected error for unregistered adapter")
	}
	if !containsAll(err.Error(), `no adapter registered for "oracle"`, "available:") {
		t.Fatalf("error lacks registry guidance: %v", err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
			return false
		}
	}
	return true
}
