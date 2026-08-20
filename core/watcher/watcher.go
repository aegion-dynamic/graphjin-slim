package watcher

import (
	"context"
	"time"
)

// CheckFunc is invoked periodically to determine if a schema reload is needed.
type CheckFunc func(ctx context.Context) (needsReload bool, err error)

// ReloadFunc is called when a schema change is detected.
type ReloadFunc func() error

// Start launches a background schema watcher loop.
func Start(pollDuration time.Duration, lc *Lifecycle, check CheckFunc, reload ReloadFunc) {
	if lc == nil || check == nil || reload == nil {
		return
	}

	if pollDuration < 1*time.Second {
		return
	}
	if pollDuration < 5*time.Second {
		pollDuration = 10 * time.Second
	}

	go func() {
		ticker := time.NewTicker(pollDuration)
		defer ticker.Stop()

		for {
			select {
			case <-lc.Done():
				return
			case <-ticker.C:
			}

			needsReload, err := check(context.Background())
			if err != nil || !needsReload {
				continue
			}

			_ = reload()
		}
	}()
}
