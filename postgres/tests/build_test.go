package postgres_test

import (
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/postgres/v3"
)

func TestBuildConnSmoke(t *testing.T) {
	dsn, err := postgres.BuildConn(postgres.Options{
		Host:       "db.example",
		Port:       5432,
		User:       "app",
		Schema:     "public",
		AppName:    "unit",
		ConnString: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dsn == "" {
		t.Fatal("empty connector DSN")
	}
}

func TestBuildConnTLSRequiresCert(t *testing.T) {
	_, err := postgres.BuildConn(postgres.Options{
		EnableTLS:  true,
		ServerName: "db.example",
	})
	if err == nil || !strings.Contains(err.Error(), "server_cert") {
		t.Fatalf("err = %v, want missing server_cert", err)
	}
}
