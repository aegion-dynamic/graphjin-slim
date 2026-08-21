package sqlite

/*
#cgo CFLAGS: -DSQLITE_HAS_CODEC -DSQLITE_TEMP_STORE=2 -DSQLITE_THREADSAFE=1
#cgo CFLAGS: -DSQLITE_EXTRA_INIT=sqlcipher_extra_init -DSQLITE_EXTRA_SHUTDOWN=sqlcipher_extra_shutdown
#cgo CFLAGS: -DSQLCIPHER_CRYPTO_OPENSSL -DSQLITE_ENABLE_FTS5 -DSQLITE_ENABLE_MATH_FUNCTIONS
#cgo LDFLAGS: -lcrypto -lm
#include "sqlite3.h"
#include <stdlib.h>
#include <string.h>

// Bind helpers: SQLITE_TRANSIENT makes SQLite copy the buffer, keeping cgo
// pointer-passing rules satisfied without exposing the destructor type.
static int bind_text_transient(sqlite3_stmt *stmt, int i, const char *p, int n) {
	return sqlite3_bind_text(stmt, i, p, n, SQLITE_TRANSIENT);
}
static int bind_blob_transient(sqlite3_stmt *stmt, int i, const void *p, int n) {
	return sqlite3_bind_blob(stmt, i, p, n, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// DriverName is the database/sql registration name.
const DriverName = "sqlite"

func init() {
	sql.Register(DriverName, &Driver{})
}

// Driver is the SQLCipher-backed database/sql driver.
type Driver struct{}

// Open implements driver.Driver.
func (d *Driver) Open(dsn string) (driver.Conn, error) {
	return newConn(dsn)
}

// Version returns the underlying SQLite library version string.
func Version() string {
	return C.GoString(C.sqlite3_libversion())
}

// ---------------------------------------------------------------------------
// connection

type conn struct {
	db     *C.sqlite3
	mu     sync.Mutex // serializes statement execution on this connection
	closed bool
}

var (
	_ driver.Conn           = (*conn)(nil)
	_ driver.ExecerContext  = (*conn)(nil)
	_ driver.QueryerContext = (*conn)(nil)
	_ driver.Pinger         = (*conn)(nil)
)

func newConn(dsn string) (*conn, error) {
	base, key, pragmas := splitDSN(dsn)

	cPath := C.CString(base)
	defer C.free(unsafe.Pointer(cPath))

	flags := C.int(C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE |
		C.SQLITE_OPEN_FULLMUTEX | C.SQLITE_OPEN_URI)
	var db *C.sqlite3
	if rc := C.sqlite3_open_v2(cPath, &db, flags, nil); rc != C.SQLITE_OK {
		err := lastError(db, rc)
		if db != nil {
			C.sqlite3_close_v2(db)
		}
		return nil, err
	}
	c := &conn{db: db}

	// Initialization order is explicit and owned by this driver:
	//   open_v2 -> PRAGMA key (first statement when configured)
	//           -> remaining pragmas -> normal use
	if key != "" {
		if err := c.execRaw(applyKey(key) + ";"); err != nil {
			c.Close()
			return nil, fmt.Errorf("sqlite: apply encryption key: %w", err)
		}
	}
	for _, p := range pragmas {
		if err := c.execRaw("PRAGMA " + sanitizePragma(p) + ";"); err != nil {
			c.Close()
			return nil, fmt.Errorf("sqlite: pragma %q: %w", p, err)
		}
	}
	return c, nil
}

func (c *conn) Ping(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.execRaw("SELECT 1;")
}

func (c *conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	rc := C.sqlite3_close_v2(c.db)
	c.closed = true
	if rc != C.SQLITE_OK {
		return lastError(c.db, rc)
	}
	return nil
}

// execRaw runs one or more statements with no bound parameters.
func (c *conn) execRaw(sqlText string) error {
	cSQL := C.CString(sqlText)
	defer C.free(unsafe.Pointer(cSQL))
	var errMsg *C.char
	rc := C.sqlite3_exec(c.db, cSQL, nil, nil, &errMsg)
	if errMsg != nil {
		err := &Error{Code: int(rc), Msg: C.GoString(errMsg)}
		C.sqlite3_free(unsafe.Pointer(errMsg))
		return err
	}
	if rc != C.SQLITE_OK {
		return lastError(c.db, rc)
	}
	return nil
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(args) == 0 {
		// Multi-statement scripts are supported for parameterless execution,
		// mirroring the previous driver's behavior for setup/teardown paths.
		if err := c.execRaw(query); err != nil {
			return nil, err
		}
		return &result{
			lastInsertID: int64(C.sqlite3_last_insert_rowid(c.db)),
			rowsAffected: int64(C.sqlite3_changes64(c.db)),
		}, nil
	}

	s, err := c.prepareLocked(query)
	if err != nil {
		return nil, err
	}
	defer s.finalize()
	if err := s.bind(args); err != nil {
		return nil, err
	}
	if err := s.stepDone(); err != nil {
		return nil, err
	}
	return &result{
		lastInsertID: int64(C.sqlite3_last_insert_rowid(c.db)),
		rowsAffected: int64(C.sqlite3_changes64(c.db)),
	}, nil
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	s, err := c.prepareLocked(query)
	if err != nil {
		return nil, err
	}
	if err := s.bind(args); err != nil {
		s.finalize()
		return nil, err
	}
	r := &rows{conn: c, stmt: s}
	if err := r.prefetch(); err != nil {
		s.finalize()
		return nil, err
	}
	return r, nil
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, err := c.prepareLocked(query)
	if err != nil {
		return nil, err
	}
	return &stmtHandle{parent: c, stmt: s}, nil
}

func (c *conn) prepareLocked(query string) (*stmt, error) {
	cSQL := C.CString(query)
	defer C.free(unsafe.Pointer(cSQL))
	var s *C.sqlite3_stmt
	var tail *C.char
	if rc := C.sqlite3_prepare_v2(c.db, cSQL, -1, &s, &tail); rc != C.SQLITE_OK {
		return nil, lastError(c.db, rc)
	}
	return &stmt{db: c.db, st: s}, nil
}

func (c *conn) Begin() (driver.Tx, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.execRaw("BEGIN;"); err != nil {
		return nil, err
	}
	return &tx{c: c}, nil
}

// ---------------------------------------------------------------------------
// transactions

type tx struct{ c *conn }

var _ driver.Tx = (*tx)(nil)

func (t *tx) Commit() error   { return t.finish("COMMIT;") }
func (t *tx) Rollback() error { return t.finish("ROLLBACK;") }

func (t *tx) finish(op string) error {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	return t.c.execRaw(op)
}

// ---------------------------------------------------------------------------
// prepared statements (driver.Stmt path)

type stmtHandle struct {
	parent *conn
	stmt   *stmt
}

var _ driver.Stmt = (*stmtHandle)(nil)

func (h *stmtHandle) Close() error {
	h.parent.mu.Lock()
	defer h.parent.mu.Unlock()
	h.stmt.finalize()
	return nil
}

func (h *stmtHandle) NumInput() int {
	h.parent.mu.Lock()
	defer h.parent.mu.Unlock()
	return int(C.sqlite3_bind_parameter_count(h.stmt.st))
}

func namedFromValues(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, v := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return out
}

func (h *stmtHandle) exec(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.parent.mu.Lock()
	defer h.parent.mu.Unlock()
	if err := h.stmt.bind(args); err != nil {
		return nil, err
	}
	if err := h.stmt.stepDone(); err != nil {
		return nil, err
	}
	return &result{
		lastInsertID: int64(C.sqlite3_last_insert_rowid(h.stmt.db)),
		rowsAffected: int64(C.sqlite3_changes64(h.stmt.db)),
	}, nil
}

func (h *stmtHandle) query(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.parent.mu.Lock()
	defer h.parent.mu.Unlock()
	if err := h.stmt.bind(args); err != nil {
		return nil, err
	}
	r := &rows{conn: h.parent, stmt: h.stmt}
	if err := r.prefetch(); err != nil {
		h.stmt.finalize()
		return nil, err
	}
	return r, nil
}

func (h *stmtHandle) Exec(args []driver.Value) (driver.Result, error) {
	return h.exec(context.Background(), namedFromValues(args))
}

func (h *stmtHandle) Query(args []driver.Value) (driver.Rows, error) {
	return h.query(context.Background(), namedFromValues(args))
}

func (h *stmtHandle) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return h.exec(ctx, args)
}

func (h *stmtHandle) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return h.query(ctx, args)
}

// ---------------------------------------------------------------------------
// prepared statement internals

type stmt struct {
	db *C.sqlite3
	st *C.sqlite3_stmt
}

func (s *stmt) finalize() {
	if s.st != nil {
		C.sqlite3_finalize(s.st)
		s.st = nil
	}
}

func (s *stmt) bind(args []driver.NamedValue) error {
	for _, a := range args {
		if err := bindValue(s, a.Ordinal, a.Value); err != nil {
			return err
		}
	}
	return nil
}

func (s *stmt) stepDone() error {
	for {
		rc := C.sqlite3_step(s.st)
		switch rc {
		case C.SQLITE_DONE:
			return nil
		case C.SQLITE_ROW:
			continue // drain rows produced by triggers etc.
		default:
			return lastError(s.db, rc)
		}
	}
}

func checkRC(rc C.int, db *C.sqlite3) error {
	if rc != C.SQLITE_OK {
		return lastError(db, rc)
	}
	return nil
}

func bindValue(s *stmt, i int, v driver.Value) error {
	switch val := v.(type) {
	case nil:
		return checkRC(C.sqlite3_bind_null(s.st, C.int(i)), s.db)
	case int64:
		return checkRC(C.sqlite3_bind_int64(s.st, C.int(i), C.sqlite3_int64(val)), s.db)
	case float64:
		return checkRC(C.sqlite3_bind_double(s.st, C.int(i), C.double(val)), s.db)
	case bool:
		b := 0
		if val {
			b = 1
		}
		return checkRC(C.sqlite3_bind_int(s.st, C.int(i), C.int(b)), s.db)
	case []byte:
		if len(val) == 0 {
			return checkRC(C.bind_blob_transient(s.st, C.int(i), nil, 0), s.db)
		}
		return checkRC(C.bind_blob_transient(s.st, C.int(i), unsafe.Pointer(&val[0]), C.int(len(val))), s.db)
	case string:
		if len(val) == 0 {
			return checkRC(C.bind_text_transient(s.st, C.int(i), nil, 0), s.db)
		}
		return checkRC(C.bind_text_transient(s.st, C.int(i),
			(*C.char)(unsafe.Pointer(unsafe.StringData(val))), C.int(len(val))), s.db)
	case time.Time:
		return bindValue(s, i, val.Format(time.RFC3339Nano))
	default:
		return fmt.Errorf("sqlite: unsupported bind type %T", v)
	}
}

// ---------------------------------------------------------------------------
// results / rows

type result struct {
	lastInsertID int64
	rowsAffected int64
}

func (r *result) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r *result) RowsAffected() (int64, error) { return r.rowsAffected, nil }

type rows struct {
	conn    *conn
	stmt    *stmt
	cols    []string
	pending []driver.Value
	done    bool
}

var _ driver.Rows = (*rows)(nil)

func (r *rows) Columns() []string {
	if r.cols == nil {
		n := int(C.sqlite3_column_count(r.stmt.st))
		r.cols = make([]string, n)
		for i := 0; i < n; i++ {
			r.cols[i] = C.GoString(C.sqlite3_column_name(r.stmt.st, C.int(i)))
		}
	}
	return r.cols
}

func (r *rows) Close() error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()
	r.stmt.finalize()
	return nil
}

// prefetch steps once so the first row's values are available before return.
func (r *rows) prefetch() error {
	rc := C.sqlite3_step(r.stmt.st)
	switch rc {
	case C.SQLITE_ROW:
		r.pending = r.readRow()
		return nil
	case C.SQLITE_DONE:
		r.done = true
		return nil
	default:
		return lastError(r.stmt.db, rc)
	}
}

func (r *rows) Next(dest []driver.Value) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	if r.pending == nil && r.done {
		return io.EOF
	}
	copy(dest, r.pending)
	r.pending = nil

	rc := C.sqlite3_step(r.stmt.st)
	switch rc {
	case C.SQLITE_ROW:
		r.pending = r.readRow()
	case C.SQLITE_DONE:
		r.done = true
	default:
		return lastError(r.stmt.db, rc)
	}
	return nil
}

func (r *rows) readRow() []driver.Value {
	n := int(C.sqlite3_column_count(r.stmt.st))
	out := make([]driver.Value, n)
	for i := 0; i < n; i++ {
		switch C.sqlite3_column_type(r.stmt.st, C.int(i)) {
		case C.SQLITE_INTEGER:
			out[i] = int64(C.sqlite3_column_int64(r.stmt.st, C.int(i)))
		case C.SQLITE_FLOAT:
			out[i] = float64(C.sqlite3_column_double(r.stmt.st, C.int(i)))
		case C.SQLITE_TEXT:
			ptr := unsafe.Pointer(C.sqlite3_column_text(r.stmt.st, C.int(i)))
			size := C.int(C.sqlite3_column_bytes(r.stmt.st, C.int(i)))
			out[i] = strings.Clone(C.GoStringN((*C.char)(ptr), size))
		case C.SQLITE_BLOB:
			size := C.int(C.sqlite3_column_bytes(r.stmt.st, C.int(i)))
			blob := make([]byte, size)
			if size > 0 {
				ptr := C.sqlite3_column_blob(r.stmt.st, C.int(i))
				blob = C.GoBytes(ptr, size)
			}
			out[i] = blob
		default: // SQLITE_NULL
			out[i] = nil
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// DSN handling + errors

// splitDSN separates the base DSN from the internal encryption-key parameter
// and ordered _pragma parameters. The key parameter never reaches SQLite as
// a DSN token; it is applied explicitly as the first statement in newConn.
func splitDSN(dsn string) (base, key string, pragmas []string) {
	i := strings.IndexByte(dsn, '?')
	if i < 0 {
		return dsn, "", nil
	}
	base, query := dsn[:i], dsn[i+1:]
	for _, kv := range strings.Split(query, "&") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || v == "" {
			continue
		}
		if dec, err := url.QueryUnescape(v); err == nil {
			v = dec
		}
		switch k {
		case "_gj_encryption_key":
			key = v
		case "_pragma":
			pragmas = append(pragmas, v)
		}
	}
	return base, key, pragmas
}

// sanitizePragma strips characters that could terminate a PRAGMA early.
func sanitizePragma(v string) string {
	return strings.NewReplacer(";", "", "\x00", "").Replace(v)
}

// Error is a SQLite error. Message text matches upstream SQLite exactly
// (e.g. "database is locked"), which GraphJin's retry detection relies on.
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// ErrorCode returns the raw SQLite result code.
func (e *Error) ErrorCode() int { return e.Code }

func lastError(db *C.sqlite3, rc C.int) error {
	msg := "unknown sqlite error"
	if db != nil {
		msg = C.GoString(C.sqlite3_errmsg(db))
	}
	return &Error{Code: int(rc), Msg: msg}
}
