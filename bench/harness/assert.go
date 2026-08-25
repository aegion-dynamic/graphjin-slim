package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ErrKnownBug marks a failure caused by a documented engine defect
// rather than a regression. Runners turn it into a visible skip.
var ErrKnownBug = errors.New("known engine bug")

// GQL posts a query to the GraphQL endpoint and returns the full decoded
// response (data + errors keys as present).
func (h *H) GQL(query string, vars map[string]any) (map[string]any, error) {
	return h.GQLFull(query, vars, nil)
}

// GQLFull posts with optional variables and extensions (APQ etc.).
func (h *H) GQLFull(query string, vars map[string]any, ext map[string]any) (map[string]any, error) {
	body := map[string]any{"query": query}
	if vars != nil {
		body["variables"] = vars
	}
	if ext != nil {
		body["extensions"] = ext
	}
	return h.postJSON(h.BaseURL+"/api/v1/graphql", body)
}

// MustData is GQL but fails when the response carries GraphQL errors.
func (h *H) MustData(query string, vars map[string]any) (map[string]any, error) {
	resp, err := h.GQL(query, vars)
	if err != nil {
		return nil, err
	}
	if errs, ok := resp["errors"]; ok && errs != nil {
		eb, _ := json.Marshal(errs)
		return nil, fmt.Errorf("graphql errors: %s", eb)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("no data object in response: %v", resp)
	}
	return data, nil
}

// Rest calls a saved-query REST endpoint. q may carry GET parameters; body
// is JSON-encoded for POST/PUT/DELETE when non-nil.
func (h *H) Rest(method, name string, q url.Values, body any) (int, map[string]any, []byte, error) {
	full := h.BaseURL + "/api/v1/rest/" + name
	if len(q) != 0 {
		full += "?" + q.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, full, rd)
	if err != nil {
		return 0, nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.hc.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, nil, err
	}
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded) // body may be non-JSON on hard errors
	return resp.StatusCode, decoded, raw, nil
}

// Console talks to the saved-query management API.
func (h *H) Console(method, path string, body any) (int, map[string]any, error) {
	full := h.BaseURL + path
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, full, rd)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	var decoded map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	return resp.StatusCode, decoded, nil
}

// PostRaw posts a raw body (no JSON encoding) and returns the status.
func (h *H) PostRaw(path string, body string) (int, []byte, error) {
	resp, err := http.Post(h.BaseURL+path, "application/json", strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, err
}

func (h *H) postJSON(target string, body any) (map[string]any, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := h.hc.Post(target, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("non-JSON response from %s: %w", target, err)
	}
	return decoded, nil
}

// Get fetches any URL on the service and returns raw bytes with status.
func (h *H) Get(path string) (int, []byte, error) {
	resp, err := h.hc.Get(h.BaseURL + path)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, err
}

// Walk descends nested JSON following dotted keys ("c1.c2.c3"). Numeric
// segments index into arrays ("users.0.full_name").
func Walk(m map[string]any, path string) (any, error) {
	cur := any(m)
	for _, key := range splitDots(path) {
		if idx, err := strconv.Atoi(key); err == nil {
			list, ok := cur.([]any)
			if !ok || idx < 0 || idx >= len(list) {
				return nil, fmt.Errorf("walk %q: %v is not a list of length >%s", path, cur, key)
			}
			cur = list[idx]
			continue
		}
		asMap, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("walk %q: %T at %q is not an object", path, cur, key)
		}
		next, ok := asMap[key]
		if !ok {
			return nil, fmt.Errorf("walk %q: missing key %q", path, key)
		}
		cur = next
	}
	return cur, nil
}

func splitDots(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
