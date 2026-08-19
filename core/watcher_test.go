package core

import (
	"testing"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/engine"
)

func TestGraphJinCloseStopsWatcherPromptly(t *testing.T) {
	g := &GraphJin{lifecycle: engine.NewLifecycle()}

	stopped := make(chan struct{})
	go func() {
		g.startDBWatcher(10 * time.Second)
		close(stopped)
	}()

	g.Close()
	g.Close()

	select {
	case <-stopped:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected watcher to stop promptly after Close")
	}
}
