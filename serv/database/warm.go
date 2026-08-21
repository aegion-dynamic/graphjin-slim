package database

import (
	"context"
	"database/sql"
	"sync"
)

// WarmPool opens and keys up to n physical connections so that per-
// connection initialization cost (with SQLCipher: PBKDF2 key derivation,
// ~160ms at default settings) is paid once at startup instead of on the
// first requests.
//
// It works entirely at the database/sql level and guarantees that every
// warmed connection is a DISTINCT physical connection:
//
//  1. n *sql.Conn handles are acquired and HELD. Because none are released
//     during acquisition, database/sql must create a fresh physical
//     connection for each handle (up to MaxOpenConns).
//  2. Each handle then runs SELECT count(*) FROM sqlite_master. sqlite_master
//     exists in every SQLite database (no application table assumed) and
//     reading it forces page-1 access, which is exactly what triggers lazy
//     key derivation on SQLCipher connections.
//  3. Handles are released back to the pool, leaving them keyed and idle.
//
// Read-only: no writes, no schema assumptions, no driver-specific calls.
// n <= 0 or nil db is a no-op. The first error aborts warming; callers
// decide policy (e.g. log-and-continue for uninitialized databases).
func WarmPool(db *sql.DB, n int) error {
	return WarmPoolContext(context.Background(), db, n)
}

// WarmPoolContext is WarmPool with caller-controlled cancellation.
func WarmPoolContext(ctx context.Context, db *sql.DB, n int) error {
	if db == nil || n <= 0 {
		return nil
	}
	if max := db.Stats().MaxOpenConnections; max > 0 && n > max {
		n = max // never exceed the configured pool ceiling
	}

	// Phase 1: acquire and HOLD n distinct physical connections. Holding is
	// what forces distinctness — released conns could be handed to a later
	// acquisition instead of triggering a new physical open.
	conns := make([]*sql.Conn, 0, n)
	defer func() {
		for _, c := range conns {
			c.Close() //nolint:errcheck
		}
	}()
	for i := 0; i < n; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		conns = append(conns, conn)
	}

	// Phase 2: touch a real page on every connection in parallel so all key
	// derivations overlap (wall time ≈ one derivation, not n).
	errMu := sync.Mutex{}
	var firstErr error
	setErr := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		defer errMu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	var wg sync.WaitGroup
	for _, conn := range conns {
		wg.Add(1)
		go func(conn *sql.Conn) {
			defer wg.Done()
			if err := ctx.Err(); err != nil {
				setErr(err)
				return
			}
			var count int
			if err := conn.
				QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master").
				Scan(&count); err != nil {
				setErr(err)
			}
		}(conn)
	}
	wg.Wait()
	return firstErr
}
