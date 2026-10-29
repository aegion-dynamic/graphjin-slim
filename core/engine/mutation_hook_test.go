package engine

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sqlgen"
)

type hookTestState struct {
	mu        sync.Mutex
	queries   []string
	begins    int
	commits   int
	rollbacks int
	result    []byte
}

func (s *hookTestState) snapshot() (queries []string, begins, commits, rollbacks int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.queries...), s.begins, s.commits, s.rollbacks
}

type hookTestConnector struct{ state *hookTestState }

func (c hookTestConnector) Connect(context.Context) (driver.Conn, error) {
	return &hookTestConn{state: c.state}, nil
}
func (c hookTestConnector) Driver() driver.Driver { return hookTestDriver{} }

type hookTestDriver struct{}

func (hookTestDriver) Open(string) (driver.Conn, error) { return nil, errors.New("not supported") }

type hookTestConn struct{ state *hookTestState }

func (c *hookTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c *hookTestConn) Close() error                        { return nil }
func (c *hookTestConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *hookTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	c.state.begins++
	c.state.mu.Unlock()
	return &hookTestTx{state: c.state}, nil
}
func (c *hookTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	c.state.queries = append(c.state.queries, query)
	result := append([]byte(nil), c.state.result...)
	c.state.mu.Unlock()
	return &hookTestRows{data: result}, nil
}

type hookTestTx struct{ state *hookTestState }

func (t *hookTestTx) Commit() error {
	t.state.mu.Lock()
	t.state.commits++
	t.state.mu.Unlock()
	return nil
}
func (t *hookTestTx) Rollback() error {
	t.state.mu.Lock()
	t.state.rollbacks++
	t.state.mu.Unlock()
	return nil
}

type hookTestRows struct {
	data []byte
	done bool
}

func (r *hookTestRows) Columns() []string { return []string{"data"} }
func (r *hookTestRows) Close() error      { return nil }
func (r *hookTestRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.data
	return nil
}

func newHookTestEngine(t *testing.T, state *hookTestState) *graphjinEngine {
	t.Helper()
	db := sql.OpenDB(hookTestConnector{state: state})
	gj := &graphjinEngine{
		conf:        &Config{},
		trace:       NewTracer(),
		prodSec:     true,
		defaultDB:   "default",
		printFormat: []byte("gj-test:"),
		databases: map[string]*dbContext{
			"default": {
				name:   "default",
				db:     db,
				dbtype: "postgres",
				sqlgenCompiler: sqlgen.NewCompiler(sqlgen.Config{
					DBType:                 "postgres",
					CaptureMutationChanges: true,
				}),
			},
		},
	}
	t.Cleanup(func() { _ = db.Close() })
	return gj
}

func runHookTestMutation(t *testing.T, gj *graphjinEngine, op qcode.QType) (*gstate, error) {
	t.Helper()
	s := &gstate{
		gj:       gj,
		r:        GraphqlReq{operation: op},
		database: gj.defaultDB,
	}
	s.compileFn = func() error {
		s.cs.st.sql = "SELECT 1"
		s.cs.st.qc = &qcode.QCode{Type: op}
		return nil
	}
	err := s.compileAndExecute(context.Background())
	return s, err
}

func TestMutationHookRunsBeforeCommit(t *testing.T) {
	state := &hookTestState{result: []byte(`{"__gj_audit":[{"schema":"public","table":"products","action":"INSERT","record_id":"1","old":null,"new":{"id":1}}],"products":{"id":1}}`)}
	gj := newHookTestEngine(t, state)

	var (
		called       bool
		seenChanges  []Change
		beforeCommit bool
	)
	gj.mutationHook = func(_ context.Context, tx *sql.Tx, ev MutationEvent) error {
		called = true
		seenChanges = ev.Changes
		_, begins, commits, _ := state.snapshot()
		beforeCommit = tx != nil && begins == 1 && commits == 0
		return nil
	}

	s, err := runHookTestMutation(t, gj, qcode.QTMutation)
	if err != nil {
		t.Fatal(err)
	}
	if !called || !beforeCommit {
		t.Fatalf("hook called=%v beforeCommit=%v", called, beforeCommit)
	}
	if len(seenChanges) != 1 || seenChanges[0].Action != "INSERT" {
		t.Fatalf("changes = %+v", seenChanges)
	}
	if strings.Contains(string(s.data), mutationAuditField) {
		t.Fatalf("audit envelope leaked into response: %s", s.data)
	}

	queries, begins, commits, rollbacks := state.snapshot()
	if begins != 1 || commits != 1 || rollbacks != 0 {
		t.Fatalf("begins=%d commits=%d rollbacks=%d", begins, commits, rollbacks)
	}
	if len(queries) != 1 || strings.Contains(queries[0], mutationAuditField) {
		t.Fatalf("unexpected queries: %v", queries)
	}
}

func TestMutationHookMissingEnvelopeFailsClosed(t *testing.T) {
	state := &hookTestState{result: []byte(`{"products":{"id":1}}`)}
	gj := newHookTestEngine(t, state)
	gj.mutationHook = func(context.Context, *sql.Tx, MutationEvent) error {
		t.Fatal("hook called without an audit envelope")
		return nil
	}

	if _, err := runHookTestMutation(t, gj, qcode.QTMutation); err == nil {
		t.Fatal("expected a capture error")
	}
	_, _, commits, rollbacks := state.snapshot()
	if commits != 0 || rollbacks != 1 {
		t.Fatalf("commits=%d rollbacks=%d", commits, rollbacks)
	}
}

func TestMutationHookNoChangesStillCommits(t *testing.T) {
	state := &hookTestState{result: []byte(`{"__gj_audit":[],"products":[]}`)}
	gj := newHookTestEngine(t, state)
	gj.mutationHook = func(context.Context, *sql.Tx, MutationEvent) error {
		t.Fatal("hook called without any changes")
		return nil
	}

	if _, err := runHookTestMutation(t, gj, qcode.QTMutation); err != nil {
		t.Fatal(err)
	}
	_, _, commits, rollbacks := state.snapshot()
	if commits != 1 || rollbacks != 0 {
		t.Fatalf("commits=%d rollbacks=%d", commits, rollbacks)
	}
}

func TestMutationHookErrorRollsBack(t *testing.T) {
	state := &hookTestState{result: []byte(`{"__gj_audit":[{"schema":"public","table":"products","action":"INSERT","record_id":"1","old":null,"new":{"id":1}}],"products":{"id":1}}`)}
	gj := newHookTestEngine(t, state)
	gj.mutationHook = func(context.Context, *sql.Tx, MutationEvent) error {
		return errors.New("audit write failed")
	}

	if _, err := runHookTestMutation(t, gj, qcode.QTMutation); err == nil {
		t.Fatal("expected hook error")
	}
	_, _, commits, rollbacks := state.snapshot()
	if commits != 0 || rollbacks != 1 {
		t.Fatalf("commits=%d rollbacks=%d", commits, rollbacks)
	}
}

func TestMutationHookPanicRollsBack(t *testing.T) {
	state := &hookTestState{result: []byte(`{"__gj_audit":[{"schema":"public","table":"products","action":"INSERT","record_id":"1","old":null,"new":{"id":1}}],"products":{"id":1}}`)}
	gj := newHookTestEngine(t, state)
	gj.mutationHook = func(context.Context, *sql.Tx, MutationEvent) error {
		panic("boom")
	}

	_, err := runHookTestMutation(t, gj, qcode.QTMutation)
	if err == nil || !strings.Contains(err.Error(), "mutation hook panic") {
		t.Fatalf("err = %v", err)
	}
	_, _, commits, rollbacks := state.snapshot()
	if commits != 0 || rollbacks != 1 {
		t.Fatalf("commits=%d rollbacks=%d", commits, rollbacks)
	}
}

func TestMutationHookUsesCallerTransaction(t *testing.T) {
	state := &hookTestState{result: []byte(`{"__gj_audit":[{"schema":"public","table":"products","action":"INSERT","record_id":"1","old":null,"new":{"id":1}}],"products":{"id":1}}`)}
	gj := newHookTestEngine(t, state)

	ctx := context.Background()
	tx, err := gj.databases["default"].db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck

	var hookTx *sql.Tx
	gj.mutationHook = func(_ context.Context, tx *sql.Tx, _ MutationEvent) error {
		hookTx = tx
		return nil
	}

	s := &gstate{
		gj:       gj,
		r:        GraphqlReq{operation: qcode.QTMutation, requestconfig: &RequestConfig{Tx: tx}},
		database: gj.defaultDB,
	}
	s.compileFn = func() error {
		s.cs.st.sql = "SELECT 1"
		s.cs.st.qc = &qcode.QCode{Type: qcode.QTMutation}
		return nil
	}
	if err := s.compileAndExecute(ctx); err != nil {
		t.Fatal(err)
	}
	if hookTx != tx {
		t.Fatal("hook did not receive the caller transaction")
	}
	_, _, commits, rollbacks := state.snapshot()
	if commits != 0 || rollbacks != 0 {
		t.Fatalf("engine touched caller transaction: commits=%d rollbacks=%d", commits, rollbacks)
	}
}

func TestMutationHookNotCalledForReads(t *testing.T) {
	state := &hookTestState{result: []byte(`{"products":[{"id":1}]}`)}
	gj := newHookTestEngine(t, state)
	gj.mutationHook = func(context.Context, *sql.Tx, MutationEvent) error {
		t.Fatal("hook called for a read")
		return nil
	}

	if _, err := runHookTestMutation(t, gj, qcode.QTQuery); err != nil {
		t.Fatal(err)
	}
	_, begins, commits, rollbacks := state.snapshot()
	if begins != 0 || commits != 0 || rollbacks != 0 {
		t.Fatalf("begins=%d commits=%d rollbacks=%d", begins, commits, rollbacks)
	}
}
