package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const mutationAuditField = "__gj_audit"

// extractMutationAudit removes the engine's reserved audit envelope from a
// mutation result and returns the remaining response plus the captured changes.
// The envelope is never exposed to GraphQL clients.
func extractMutationAudit(data []byte) ([]byte, []Change, error) {
	if len(data) == 0 {
		return data, nil, errors.New("mutation audit: empty result")
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return data, nil, fmt.Errorf("mutation audit: decode result: %w", err)
	}
	if tok != json.Delim('{') {
		return data, nil, fmt.Errorf("mutation audit: expected object result, got %v", tok)
	}

	var (
		audit json.RawMessage
		out   bytes.Buffer
	)
	out.WriteByte('{')
	first := true
	for dec.More() {
		tok, err = dec.Token()
		if err != nil {
			return data, nil, fmt.Errorf("mutation audit: decode key: %w", err)
		}
		key, ok := tok.(string)
		if !ok {
			return data, nil, fmt.Errorf("mutation audit: expected object key, got %v", tok)
		}

		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return data, nil, fmt.Errorf("mutation audit: decode field %q: %w", key, err)
		}
		if key == mutationAuditField {
			audit = raw
			continue
		}

		if !first {
			out.WriteByte(',')
		}
		keyJSON, _ := json.Marshal(key)
		out.Write(keyJSON)
		out.WriteByte(':')
		out.Write(raw)
		first = false
	}
	if _, err := dec.Token(); err != nil {
		return data, nil, fmt.Errorf("mutation audit: decode object close: %w", err)
	}
	out.WriteByte('}')

	if len(audit) == 0 {
		return data, nil, errors.New("mutation audit: result is missing the audit envelope")
	}
	var changes []Change
	if err := json.Unmarshal(audit, &changes); err != nil {
		return data, nil, fmt.Errorf("mutation audit: decode changes: %w", err)
	}
	for i := range changes {
		if isJSONNull(changes[i].Old) {
			changes[i].Old = nil
		}
		if isJSONNull(changes[i].New) {
			changes[i].New = nil
		}
	}
	return out.Bytes(), changes, nil
}

func isJSONNull(b []byte) bool {
	return len(bytes.TrimSpace(b)) == 4 && string(bytes.TrimSpace(b)) == "null"
}
