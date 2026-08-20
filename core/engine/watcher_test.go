package engine

import (
	"testing"
	"time"
)

func TestGraphJinCloseStopsWatcherPromptly(t *testing.T) {
	g := &GraphJin{lifecycle: NewLifecycle()}

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
