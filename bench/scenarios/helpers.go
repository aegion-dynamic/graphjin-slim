package scenarios

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"fmt"
	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
	"net/url"
	"os"
	"path/filepath"
)

func fileExists(root, rel string) (bool, error) {
	_, err := os.Stat(filepath.Join(root, rel))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func fileBytes(root, rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, rel))
}

func listContains(listResp map[string]any, name string) bool {
	qs, _ := listResp["queries"].([]any)
	for _, q := range qs {
		if m := q.(map[string]any); m["name"] == name {
			return true
		}
	}
	return false
}

func urlVals(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

// firstError returns the message of the first GraphQL-style error in a
// decoded response, or "" when there are none.
func firstError(body map[string]any) string {
	errs, _ := body["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	switch e := errs[0].(type) {
	case string:
		return e
	case map[string]any:
		if m, ok := e["message"].(string); ok {
			return m
		}
	}
	return "?"
}

func parseJSON(raw []byte) map[string]any {
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

var _ = fmt.Sprintf // keep fmt imported for scenario files that may not use it

// listLen treats a field as a list of rows; single-object responses
// (some mutation shapes) count as one row.
func listLen(v any) int {
	switch t := v.(type) {
	case []any:
		return len(t)
	case map[string]any:
		return 1
	}
	return 0
}

// firstRowField returns key from the first row of data[key], accepting
// both list and object response shapes.
func firstRowField(data map[string]any, key, field string) (any, error) {
	v, err := harness.Walk(data, key)
	if err != nil {
		return nil, err
	}
	var row map[string]any
	switch t := v.(type) {
	case []any:
		if len(t) == 0 {
			return nil, fmt.Errorf("%s: empty rows", key)
		}
		row = t[0].(map[string]any)
	case map[string]any:
		row = t
	default:
		return nil, fmt.Errorf("%s: unexpected shape %T", key, v)
	}
	return row[field], nil
}
