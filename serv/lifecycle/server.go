// Package lifecycle owns service startup and shutdown mechanics.
package lifecycle

import (
	"net/http"
	"time"
)

// NewServer creates the service HTTP server with the standard safety limits.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ReadHeaderTimeout: 10 * time.Second,
	}
}
