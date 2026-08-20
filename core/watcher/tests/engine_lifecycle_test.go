package watcher_test

import (
	"sync"
	"testing"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/watcher"
)

type SchemaCallbacks = watcher.SchemaCallbacks

var NewLifecycle = watcher.NewLifecycle

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
