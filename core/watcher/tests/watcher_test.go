package watcher_test

import (
	"context"
	"testing"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/watcher"
)

var (
	Start = watcher.Start
)

func TestGraphJinCloseStopsWatcherPromptly(t *testing.T) {
	lc := NewLifecycle()

	stopped := make(chan struct{})
	go func() {
		Start(10*time.Second, lc, func(ctx context.Context) (bool, error) {
			return false, nil
		}, func() error {
			return nil
		})
		<-lc.Done()
		close(stopped)
	}()

	lc.Close()
	lc.Close()

	select {
	case <-stopped:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected watcher to stop promptly after Close")
	}
}
