package database_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
)

// warmOpts builds adapter options for a fresh encrypted-or-plain database
// with contention parity across driver builds.
func warmOpts(t *testing.T) (string, database.Options) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "warm.db")
	cs := path + "?_pragma=busy_timeout(2000)"
	cfg := database.Config{Type: "sqlite", ConnString: cs}
	if hasSQLCipherSupport(t) {
		cfg.EncryptionKey = testKey
	}
	return path, database.Options{Config: cfg}
}

func openT(t *testing.T, opts database.Options) *sql.DB {
	t.Helper()
	db, err := database.Open(opts)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func TestWarmPoolNoop(t *testing.T) {
	_, opts := warmOpts(t)
	db := openT(t, opts)
	defer db.Close()
	seedWarmTable(t, db)

	before := db.Stats().OpenConnections
	if err := database.WarmPool(db, 0); err != nil {
		t.Fatalf("n=0: %v", err)
	}
	if err := database.WarmPool(db, -3); err != nil {
		t.Fatalf("n<0: %v", err)
	}
	if got := db.Stats().OpenConnections; got != before {
		t.Fatalf("no-op warm changed OpenConnections %d -> %d", before, got)
	}
}

func TestWarmPoolInitializesNPhysicalConnections(t *testing.T) {
	encrypted := hasSQLCipherSupport(t)
	_, opts := warmOpts(t)
	db := openT(t, opts)
	defer db.Close()
	seedWarmTable(t, db)

	const n = 4
	start := time.Now()
	if err := database.WarmPoolContext(context.Background(), db, n); err != nil {
		t.Fatalf("warm: %v", err)
	}
	warmWall := time.Since(start)

	st := db.Stats()
	if encrypted {
		// KDF holds each connection open for ~150ms, so all n physical
		// connections are provably distinct and simultaneously live.
		if st.OpenConnections < n {
			t.Fatalf("OpenConnections=%d, want >= %d after warming", st.OpenConnections, n)
		}
	} else {
		t.Logf("plain build: OpenConnections=%d (conns recycle too fast to pin %d)", st.OpenConnections, n)
	}

	qStart := time.Now()
	if err := firstQuery(db); err != nil {
		t.Fatal(err)
	}
	firstQ := time.Since(qStart)

	t.Logf("warm(%d) wall=%v first_query_after=%v (encrypted=%v)",
		n, warmWall, firstQ, encrypted)

	if encrypted {
		if warmWall < 100*time.Millisecond {
			t.Logf("note: warm wall=%v below typical KDF window", warmWall)
		}
		if firstQ > 10*time.Millisecond {
			t.Fatalf("post-warm query took %v; expected µs-scale", firstQ)
		}
	}
}

func TestChurnVersusRetention(t *testing.T) {
	encrypted := hasSQLCipherSupport(t)
	path := filepath.Join(t.TempDir(), "churn.db")

	mk := func() *sql.DB {
		opts := database.Options{Config: database.Config{Type: "sqlite", Path: path}}
		if encrypted {
			opts.Config.EncryptionKey = testKey
		}
		return openT(t, opts)
	}

	seed := mk()
	seedWarmTable(t, seed)
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)

	// retained: one handle, pool keeps the keyed conn alive
	retained := mk()
	defer retained.Close()
	retained.SetMaxIdleConns(1)
	retained.SetMaxOpenConns(1)
	if err := firstQuery(retained); err != nil {
		t.Fatal(err)
	}
	rStart := time.Now()
	if err := firstQuery(retained); err != nil {
		t.Fatal(err)
	}
	retainedLatency := time.Since(rStart)

	// churned: brand-new handle per request (worst case)
	cStart := time.Now()
	fresh := mk()
	if err := firstQuery(fresh); err != nil {
		t.Fatal(err)
	}
	fresh.Close()
	churnLatency := time.Since(cStart)

	if retainedLatency > 10*time.Millisecond {
		t.Fatalf("retained second query took %v; expected µs-scale", retainedLatency)
	}
	t.Logf("retained_query=%v churn_cycle=%v (ratio %.0fx, encrypted=%v)",
		retainedLatency, churnLatency,
		float64(churnLatency)/float64(retainedLatency+time.Microsecond), encrypted)
}

func TestReopenWithWarmingExistingDB(t *testing.T) {
	encrypted := hasSQLCipherSupport(t)
	path := filepath.Join(t.TempDir(), "existing.db")

	mk := func() *sql.DB {
		opts := database.Options{Config: database.Config{Type: "sqlite", Path: path}}
		if encrypted {
			opts.Config.EncryptionKey = testKey
		}
		return openT(t, opts)
	}

	w := mk()
	seedWarmTable(t, w)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)

	reopened := mk()
	defer reopened.Close()
	if err := database.WarmPool(reopened, 2); err != nil {
		t.Fatalf("warm after reopen: %v", err)
	}
	var v string
	if err := reopened.QueryRow(`SELECT v FROM bt LIMIT 1`).Scan(&v); err != nil || v != "x" {
		t.Fatalf("post-reopen read=%q err=%v", v, err)
	}
}
