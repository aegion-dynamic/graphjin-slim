package engine

import (
	"encoding/json"
	"testing"
)

func TestParseVarValDecodesStringEscapes(t *testing.T) {
	cases := map[string]string{
		`"plain"`:        "plain",
		`"line1\nline2"`: "line1\nline2",
		`"a\\nb"`:        `a\nb`,
		`"tab\tquote\""`: "tab\tquote\"",
		`"été"`:          "été",
	}
	for raw, want := range cases {
		if got := parseVarVal(json.RawMessage(raw)); got != want {
			t.Errorf("parseVarVal(%s) = %q, want %q", raw, got, want)
		}
	}
}
