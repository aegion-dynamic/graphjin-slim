package http

import stdhttp "net/http"

const (
	ServerName      = "GraphJin"
	DefaultHostPort = "0.0.0.0:8080"
)

// WithServerHeader adds the GraphJin server header to a handler.
func WithServerHeader(handler stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Server", ServerName)
		handler.ServeHTTP(w, r)
	})
}
