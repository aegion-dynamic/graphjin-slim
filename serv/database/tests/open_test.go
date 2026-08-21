package database_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
)

func TestConnectionDefaultsToPostgres(t *testing.T) {
	driver, dsn, err := database.Connection(database.Options{Config: database.Config{Host: "db.example", Port: 5432, User: "app"}})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "pgx" {
		t.Fatalf("driver = %q, want pgx", driver)
	}
	if dsn == "" {
		t.Fatal("postgres connection returned an empty registered DSN")
	}
}

func TestConnectionUsesSQLitePath(t *testing.T) {
	driver, dsn, err := database.Connection(database.Options{Config: database.Config{Type: "sqlite", Path: ":memory:"}})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || dsn != ":memory:" {
		t.Fatalf("connection = (%q, %q), want (sqlite, :memory:)", driver, dsn)
	}
}

func TestOpenCoreRejectsUnsupportedDatabase(t *testing.T) {
	_, err := database.OpenCore(context.Background(), "analytics", core.DatabaseConfig{Type: "mysql"})
	if err == nil || !strings.Contains(err.Error(), "supported types are postgres, sqlite") {
		t.Fatalf("OpenCore error = %v, want unsupported database error", err)
	}
}

func TestOpenCoreRejectsMissingSQLitePath(t *testing.T) {
	_, err := database.OpenCore(context.Background(), "local", core.DatabaseConfig{Type: "sqlite"})
	if err == nil || !strings.Contains(err.Error(), "path or conn_string") {
		t.Fatalf("OpenCore error = %v, want missing path error", err)
	}
}
