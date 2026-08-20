package core

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/allow"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/jsn"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
)

var (
	decPrefix   = []byte(`__gj-enc:`)
	ErrNotFound = errors.New("not found in prepared statements")
)

type OpType int

const (
	OpUnknown OpType = iota
	OpQuery
	OpSubscription
	OpMutation
)

func (gj *graphjinEngine) getIntroResult() (data json.RawMessage, err error) {
	var ok bool
	if data, ok = gj.cache.Get("_intro"); ok {
		return
	}
	if data, err = gj.introQuery(); err != nil {
		return
	}
	gj.cache.Set("_intro", data)
	return
}

func (gj *graphjinEngine) initIntro() (err error) {
	if !gj.prod && gj.conf.EnableIntrospection {
		var introJSON json.RawMessage
		introJSON, err = gj.getIntroResult()
		if err != nil {
			return
		}
		err = gj.fs.Put("intro.json", []byte(introJSON))
		if err != nil {
			return
		}
	}
	return
}

func (gj *graphjinEngine) executeRoleQuery(c context.Context,
	conn *sql.Conn,
	vmap map[string]json.RawMessage,
	rc *RequestConfig,
) (role string, trustedReservedRole bool, err error) {
	return defaultCompileRole, true, nil
}

// Returns the operation type for the query result
func (r *Result) Operation() OpType {
	switch r.operation {
	case qcode.QTQuery:
		return OpQuery

	case qcode.QTMutation, qcode.QTInsert, qcode.QTUpdate, qcode.QTUpsert, qcode.QTDelete:
		return OpMutation

	default:
		return -1
	}
}

// Returns the namespace for the query result
func (r *Result) Namespace() string {
	return r.namespace
}

// Returns the operation name for the query result
func (r *Result) OperationName() string {
	return r.operation.String()
}

// Returns the query name for the query result
func (r *Result) QueryName() string {
	return r.name
}

// Returns the role used to execute the query
func (r *Result) Role() string {
	return r.role
}

// SubscriptionCursors returns the server-side cursor checkpoints advanced by a
// subscription result, keyed by the GraphQL cursor variable name.
func (r *Result) SubscriptionCursors() map[string]string {
	if r == nil || len(r.subCursors) == 0 {
		return nil
	}
	out := make(map[string]string, len(r.subCursors))
	for k, v := range r.subCursors {
		out[k] = v
	}
	return out
}

// Returns the SQL query string for the query result
func (r *Result) SQL() string {
	return r.sql
}

// Returns the cache control header value for the query result
func (r *Result) CacheControl() string {
	return r.cacheControl
}

// CacheHit returns true if the response was served from cache
func (r *Result) CacheHit() bool {
	return r.cacheHit
}

// debugLogStmt logs the query statement for debugging
func (s *gstate) debugLogStmt() {
	st := s.cs.st

	if st.qc == nil {
		return
	}

	for _, sel := range st.qc.Selects {
		if sel.SkipRender == qcode.SkipTypeUserNeeded {
			s.gj.log.Printf("Field skipped, requires $user_id or table not added to anon role: %s", sel.FieldName)
		}
		if sel.SkipRender == qcode.SkipTypeBlocked {
			s.gj.log.Printf("Field skipped, blocked: %s", sel.FieldName)
		}
	}
}

// Saved the query qcode to the allow list
func (gj *graphjinEngine) saveToAllowList(ctx context.Context, qc *qcode.QCode, ns string) (err error) {
	if qc == nil || gj.conf.DisableAllowList {
		return nil
	}

	item := allow.Item{
		Namespace: ns,
		Name:      qc.Name,
		Query:     qc.Query,
		Fragments: make([]allow.Fragment, len(qc.Fragments)),
	}

	if len(qc.ActionVal) != 0 {
		var buf bytes.Buffer
		if err = jsn.Clear(&buf, qc.ActionVal); err != nil {
			return
		}
		item.ActionJSON = map[string]json.RawMessage{
			qc.ActionVar: json.RawMessage(buf.Bytes()),
		}
	}

	for i, f := range qc.Fragments {
		item.Fragments[i] = allow.Fragment{Name: f.Name, Value: f.Value}
	}

	if gj.savedQuerySaveHook != nil {
		req := SavedQuerySaveRequest{
			Namespace:  item.Namespace,
			Name:       item.Name,
			Operation:  strings.ToLower(qc.Type.String()),
			Query:      append([]byte(nil), item.Query...),
			Fragments:  make([]SavedQueryFragment, len(item.Fragments)),
			ActionJSON: item.ActionJSON,
		}
		for i, f := range item.Fragments {
			req.Fragments[i] = SavedQueryFragment{Name: f.Name, Value: append([]byte(nil), f.Value...)}
		}
		handled, err := gj.savedQuerySaveHook(ctx, req)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
	}

	return gj.allowList.Set(item)
}

// Starts tracing with the given name
func (gj *graphjinEngine) spanStart(c context.Context, name string) (context.Context, Spaner) {
	return gj.trace.Start(c, name)
}

// Retry operation with the default policy.
func retryOperation(c context.Context, fn func() error) (err error) {
	return RetryOperationForDB(c, "", fn)
}
