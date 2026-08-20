package core

import (
	"sync"
	"testing"
	"time"
)

func TestLifecycleCloseIsIdempotent(t *testing.T) {
	l := NewLifecycle()
	l.Close()
	l.Close()
	select {
	case <-l.Done():
	case <-time.After(time.Second):
		t.Fatal("lifecycle did not signal shutdown")
	}
}

func TestSchemaCallbacksFireRegisteredObservers(t *testing.T) {
	var callbacks SchemaCallbacks
	var wg sync.WaitGroup
	wg.Add(2)
	callbacks.Register(func(database, hash string) {
		if database != "default" || hash != "abc" {
			t.Errorf("callback values = %q/%q", database, hash)
		}
		wg.Done()
	})
	callbacks.Register(func(_, _ string) { wg.Done() })
	callbacks.Fire("default", "abc")
	wg.Wait()
}
