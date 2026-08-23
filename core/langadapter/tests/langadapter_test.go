package langadapter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/langadapter"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
)

type fakeLang struct {
	name string
}

func (f fakeLang) Name() string { return f.name }

func (f fakeLang) New(c *qcode.Compiler) (langadapter.Language, error) {
	return f, nil
}

type fakeFactory struct{ name string }

func (f fakeFactory) Name() string { return f.name }

func (f fakeFactory) New(c *qcode.Compiler) (langadapter.Language, error) {
	return fakeLang{name: f.name}, nil
}

func (f fakeLang) Compile(query []byte, vars map[string]json.RawMessage,
	opts langadapter.CompileOptions,
) (*qcode.QCode, error) {
	return &qcode.QCode{Name: string(query)}, nil
}

func TestRegisterLookup(t *testing.T) {
	langadapter.Register(fakeFactory{name: "testlang-a"})
	langadapter.Register(fakeFactory{name: "testlang-b"})

	f, err := langadapter.Lookup("testlang-a")
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if f.Name() != "testlang-a" {
		t.Fatalf("got %q", f.Name())
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	langadapter.Register(fakeFactory{name: "testlang-dup"})
	langadapter.Register(fakeFactory{name: "testlang-dup"})
}

func TestRegisterNilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil registration")
		}
	}()
	langadapter.Register(nil)
}

func TestLookupUnknownListsAvailable(t *testing.T) {
	_, err := langadapter.Lookup("nope")
	if err == nil {
		t.Fatal("expected error for unknown language")
	}
	if !strings.Contains(err.Error(), "testlang-a") {
		t.Fatalf("error should list available languages, got: %v", err)
	}
}

func TestNamesSorted(t *testing.T) {
	names := langadapter.Names()
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("names not sorted: %v", names)
		}
	}
}
