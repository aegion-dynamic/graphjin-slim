package core

import "sync"

// Lifecycle owns the engine shutdown signal and makes closing idempotent.
type Lifecycle struct {
	done     chan bool
	stopOnce sync.Once
}

// NewLifecycle creates an active lifecycle controller.
func NewLifecycle() *Lifecycle { return &Lifecycle{done: make(chan bool)} }

// Done returns the channel closed when the engine stops.
func (l *Lifecycle) Done() chan bool {
	if l == nil {
		return nil
	}
	return l.done
}

// Close signals shutdown once.
func (l *Lifecycle) Close() {
	if l == nil || l.done == nil {
		return
	}
	l.stopOnce.Do(func() { close(l.done) })
}
