package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
)

type queryPolicyContextKey struct{}

func TestApplyQueryPolicyReceivesRequestContextAndConfig(t *testing.T) {
	type principal string
	const caller principal = "caller-1"

	var gotContext context.Context
	gstate := gstate{
		gj: &graphjinEngine{
			queryPolicyHook: func(ctx context.Context, qc *qcode.QCode, rc *RequestConfig) error {
				gotContext = ctx
				if qc.Name != "GetThings" {
					t.Fatalf("query name = %q, want GetThings", qc.Name)
				}
				rc.Vars["__policy_caller"] = ctx.Value(queryPolicyContextKey{})
				return nil
			},
		},
		r:   GraphqlReq{},
		ctx: context.WithValue(context.Background(), queryPolicyContextKey{}, caller),
	}
	qc := &qcode.QCode{Name: "GetThings"}

	if err := gstate.applyQueryPolicy(qc); err != nil {
		t.Fatalf("applyQueryPolicy() error = %v", err)
	}
	if gotContext != gstate.ctx {
		t.Fatal("policy hook did not receive the request context")
	}
	if got := gstate.r.requestconfig.Vars["__policy_caller"]; got != caller {
		t.Fatalf("policy variable = %#v, want %#v", got, caller)
	}
}

func TestApplyQueryPolicyFailsClosed(t *testing.T) {
	wantErr := errors.New("principal unavailable")
	gstate := gstate{
		gj: &graphjinEngine{
			queryPolicyHook: func(context.Context, *qcode.QCode, *RequestConfig) error {
				return wantErr
			},
		},
		ctx: context.Background(),
	}

	err := gstate.applyQueryPolicy(&qcode.QCode{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("applyQueryPolicy() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "query policy:") {
		t.Fatalf("error = %q, want query policy context", err)
	}
}

func TestPolicyHookBypassesProductionCompileCache(t *testing.T) {
	called := 0
	s := &gstate{
		gj: &graphjinEngine{
			prodSec: true,
			queryPolicyHook: func(context.Context, *qcode.QCode, *RequestConfig) error {
				return nil
			},
		},
		compileFn: func() error {
			called++
			return nil
		},
	}

	for range 2 {
		if err := s.compile(); err != nil {
			t.Fatalf("compile() error = %v", err)
		}
	}
	if called != 2 {
		t.Fatalf("compile function called %d times, want once per request", called)
	}
}
