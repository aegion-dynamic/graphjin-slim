package engine

import (
	"context"
	"net/http"
)

// Tracer is the optional request tracing hook used by the engine.
type Tracer interface {
	Start(c context.Context, name string) (context.Context, Spaner)
	NewHTTPClient() *http.Client
}

// Spaner is a single tracing span.
type Spaner interface {
	SetAttributesString(attrs ...StringAttr)
	IsRecording() bool
	Error(err error)
	End()
}

// StringAttr is a string key/value span attribute.
type StringAttr struct {
	Name  string
	Value string
}

type defaultTracer struct{}

type span struct{}

// NewTracer returns a no-op tracer.
func NewTracer() Tracer {
	return &defaultTracer{}
}

// Start starts a new trace span.
func (t *defaultTracer) Start(c context.Context, name string) (context.Context, Spaner) {
	return c, &span{}
}

// NewHTTPClient creates a new HTTP client.
func (t *defaultTracer) NewHTTPClient() *http.Client {
	return &http.Client{}
}

// End ends the span.
func (s *span) End() {}

// Error records an error on the span.
func (s *span) Error(err error) {}

// IsRecording reports whether the span is recording.
func (s *span) IsRecording() bool { return false }

// SetAttributesString sets string attributes on the span.
func (s *span) SetAttributesString(attrs ...StringAttr) {}
