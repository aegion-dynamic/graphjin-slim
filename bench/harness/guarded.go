package harness

import (
	"fmt"
	"time"
)

// guarded runs fn under the scenario timeout so a hung request surfaces as
// a failure instead of freezing the whole suite.
func Guarded(h *H, b Budgets, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(b.Timeout + 10*time.Second):
		return fmt.Errorf("scenario exceeded %s deadline", b.Timeout+10*time.Second)
	}
}
