package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/sqlite/v3"
)

func TestOpenEncryptedRoundtrip(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(context.Background(), sqlite.Options{
		Path:          filepath.Join(dir, "t.db"),
		EncryptionKey: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE t(v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO t VALUES('x')`); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := db.QueryRow(`SELECT v FROM t`).Scan(&got); err != nil || got != "x" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestOpenPlainHeaderStaysPlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.db")
	db, err := sqlite.Open(context.Background(), sqlite.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE t(v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	hdr, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const magic = "SQLite format 3\x00"
	if string(hdr[:15]) != magic[:15] {
		t.Fatalf("plaintext header corrupted: %q", hdr[:15])
	}
}

func TestBuildDSNRequiresTarget(t *testing.T) {
	if _, err := sqlite.BuildDSN(sqlite.Options{}); err == nil {
		t.Fatal("expected error for empty options")
	}
}

func TestBuildDSNKeepsPragmaOrder(t *testing.T) {
	dsn, err := sqlite.BuildDSN(sqlite.Options{
		Path:          "x.db",
		EncryptionKey: "k",
		Pragmas:       []string{"busy_timeout(5000)", "journal_mode(WAL)"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "x.db?_gj_encryption_key=k&_pragma=busy_timeout%285000%29&_pragma=journal_mode%28WAL%29"
	if dsn != want {
		t.Fatalf("dsn = %q, want %q", dsn, want)
	}
}

var _ = sql.ErrNoRows
