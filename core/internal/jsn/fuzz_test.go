//go:build gofuzz
// +build gofuzz

package jsn_test

import (
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/jsn"
)

var ret int

func TestFuzzCrashers(t *testing.T) {
	for _, f := range crasherJSONInputs {
		ret = jsn.Fuzz([]byte(f))
	}
}
