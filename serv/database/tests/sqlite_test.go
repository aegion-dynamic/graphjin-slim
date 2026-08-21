package database_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
)

const testKey = "correct horse battery staple"

var (
	sqlcipherOnce sync.Once
	hasSQLCipher  bool
	probePath     = filepath.Join(os.TempDir(), "gj_sqlcipher_probe.db")
)

// hasSQLCipherSupport detects at runtime whether this binary links SQLCipher
// (no build tags): an encrypted open only works when PRAGMA cipher_version
// returns a row.
func hasSQLCipherSupport(t *testing.T) bool {
	t.Helper()
	sqlcipherOnce.Do(func() {
		os.Remove(probePath)
		db, err := database.Open(database.Options{Config: database.Config{
			Type: "sqlite", Path: probePath, EncryptionKey: "probe",
		}})
		if err != nil {
			return
		}
		defer db.Close()
		var ver string
		if err := db.QueryRow("PRAGMA cipher_version").Scan(&ver); err == nil && strings.TrimSpace(ver) != "" {
			hasSQLCipher = true
		}
	})
	return hasSQLCipher
}

func openPlain(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := database.Open(database.Options{Config: database.Config{Type: "sqlite", Path: path}})
	if err != nil {
		t.Fatalf("open plain %s: %v", path, err)
	}
	return db
}

func openEncrypted(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := database.Open(database.Options{Config: database.Config{
		Type: "sqlite", Path: path, EncryptionKey: testKey,
	}})
	if err != nil {
		t.Fatalf("open encrypted %s: %v", path, err)
	}
	return db
}

func TestSQLitePlainLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.db")
	db := openPlain(t, path)
	if _, err := db.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO t VALUES ('hello')`); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := db.QueryRow(`SELECT v FROM t`).Scan(&got); err != nil || got != "hello" {
		t.Fatalf("roundtrip=%q err=%v", got, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	hdr, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hdr) < 15 || string(hdr[:15]) != "SQLite format 3" {
		n := 15
		if len(hdr) < n {
			n = len(hdr)
		}
		t.Fatalf("plaintext db header corrupted: %q", hdr[:n])
	}
}

func TestSQLiteEncryptedLifecycle(t *testing.T) {
	if !hasSQLCipherSupport(t) {
		t.Skip("binary links plain SQLite; encryption unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "enc.db")

	db := openEncrypted(t, path)
	if _, err := db.Exec(`CREATE TABLE secret (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO secret VALUES ('topsecret')`); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := db.QueryRow(`SELECT v FROM secret LIMIT 1`).Scan(&got); err != nil || got != "topsecret" {
		t.Fatalf("roundtrip=%q err=%v", got, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	hdr, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hdr) >= 15 && string(hdr[:15]) == "SQLite format 3" {
		t.Fatal("encrypted db has plaintext SQLite header")
	}

	// fresh handle against existing encrypted file (cross-process semantics)
	db2 := openEncrypted(t, path)
	defer db2.Close()
	if err := db2.QueryRow(`SELECT v FROM secret LIMIT 1`).Scan(&got); err != nil || got != "topsecret" {
		t.Fatalf("reopen roundtrip=%q err=%v", got, err)
	}
}

func TestSQLiteWrongKeyFails(t *testing.T) {
	if !hasSQLCipherSupport(t) {
		t.Skip("binary links plain SQLite; encryption unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "enc.db")

	db := openEncrypted(t, path)
	if _, err := db.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	wdb, err := database.Open(database.Options{Config: database.Config{
		Type: "sqlite", Path: path, EncryptionKey: "not-the-key",
	}})
	if err != nil {
		// Fail-fast: with the ConnectHook ordering patch the key is applied
		// before mattn's built-in pragmas, so a wrong key can abort Open.
		t.Logf("wrong key rejected at Open (fail-fast): %v", err)
		return
	}
	defer wdb.Close()
	if _, err := wdb.Query(`SELECT count(*) FROM t`); err == nil {
		t.Fatal("query with wrong key succeeded")
	}
}

func TestPoolRetentionGuard(t *testing.T) {
	db := openPlain(t, filepath.Join(t.TempDir(), "guard.db"))
	defer db.Close()
	seedWarmTable(t, db)
	if err := firstQuery(db); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for db.Stats().Idle == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("idle connection never retained (Idle=0 after query); MaxIdleConns=0 regression")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func firstQuery(db *sql.DB) error {
	var v string
	return db.QueryRow(`SELECT v FROM bt LIMIT 1`).Scan(&v)
}

func seedWarmTable(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE bt (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO bt VALUES ('x')`); err != nil {
		t.Fatal(err)
	}
}
