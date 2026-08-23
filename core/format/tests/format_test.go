package format_test

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/format"
)

func TestJSONFormatterPassthrough(t *testing.T) {
	in := json.RawMessage(`{"products":[{"name":"a"}]}`)
	f, err := format.Lookup("json")
	if err != nil {
		t.Fatalf("json formatter must always resolve: %v", err)
	}
	var buf bytes.Buffer
	if err := f.Format(&buf, format.Payload{Data: in}); err != nil {
		t.Fatalf("format failed: %v", err)
	}
	if buf.String() != string(in) {
		t.Fatalf("output not byte-identical:\n got: %s\nwant: %s", buf.String(), in)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	format.Register(jsonDup{})
	format.Register(jsonDup{})
}

type jsonDup struct{}

func (jsonDup) Name() string                        { return "testdup" }
func (jsonDup) Format(w io.Writer, p format.Payload) error { return nil }
