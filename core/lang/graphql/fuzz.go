//go:build gofuzz
// +build gofuzz

package graphql

import (
	"encoding/json"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// FuzzerEntrypoint for Fuzzbuzz
func Fuzz(data []byte) int {
	qt := GetQType(graph.ParserType(string(data)))

	schema := &sdata.DBSchema{}
	qcompile, _ := NewCompiler(schema, Config{})
	_, err := qcompile.Compile(data, map[string]json.RawMessage{}, "")
	if err != nil {
		return 0
	}

	if qt > qcode.QTUpsert {
		panic("qt > QTUpsert")
	}

	return 1
}
