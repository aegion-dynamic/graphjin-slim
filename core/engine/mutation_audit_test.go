package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractMutationAudit(t *testing.T) {
	in := `{"__gj_audit":[{"schema":"application","table":"orders","action":"UPDATE","record_id":"42","old":{"id":42,"status":"new"},"new":{"id":42,"status":"paid"}}],"orders":{"id":42}}`
	got, changes, err := extractMutationAudit([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), mutationAuditField) {
		t.Fatalf("audit envelope leaked: %s", got)
	}
	if want := `{"orders":{"id":42}}`; string(got) != want {
		t.Fatalf("response = %s, want %s", got, want)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(changes))
	}
	c := changes[0]
	if c.Schema != "application" || c.Table != "orders" || c.Action != "UPDATE" {
		t.Fatalf("unexpected change: %+v", c)
	}
	if c.RecordID == nil || *c.RecordID != "42" {
		t.Fatalf("record id = %v", c.RecordID)
	}
	var oldRow, newRow map[string]any
	if err := json.Unmarshal(c.Old, &oldRow); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(c.New, &newRow); err != nil {
		t.Fatal(err)
	}
	if oldRow["status"] != "new" || newRow["status"] != "paid" {
		t.Fatalf("old/new = %v/%v", oldRow, newRow)
	}
}

func TestExtractMutationAuditWithoutEnvelope(t *testing.T) {
	if _, _, err := extractMutationAudit([]byte(`{"orders":{"id":1}}`)); err == nil {
		t.Fatal("expected an error when the audit envelope is missing")
	}
}

func TestExtractMutationAuditNullRecordID(t *testing.T) {
	in := `{"__gj_audit":[{"schema":"public","table":"pairs","action":"DELETE","record_id":null,"old":{"a":1,"b":2},"new":null}]}`
	got, changes, err := extractMutationAudit([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{}` {
		t.Fatalf("response = %s, want {}", got)
	}
	if len(changes) != 1 || changes[0].RecordID != nil {
		t.Fatalf("unexpected changes: %+v", changes)
	}
	if changes[0].New != nil {
		t.Fatalf("new = %s, want nil", changes[0].New)
	}
}

func TestExtractMutationAuditFieldLast(t *testing.T) {
	in := `{"orders":{"id":42},"__gj_audit":[{"schema":"application","table":"orders","action":"DELETE","record_id":"42","old":{"id":42},"new":null}]}`
	got, changes, err := extractMutationAudit([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"orders":{"id":42}}` {
		t.Fatalf("response = %s", got)
	}
	if len(changes) != 1 || changes[0].New != nil {
		t.Fatalf("changes = %+v", changes)
	}
}
