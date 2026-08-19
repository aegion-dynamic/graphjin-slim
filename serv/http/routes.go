// Package http contains the service HTTP routing contract.
package http

import stdhttp "net/http"

// UnavailableWebUI returns the slim-build response for the removed embedded UI.
func UnavailableWebUI(_ string, _ string) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		stdhttp.Error(w, "web ui not available in slim build", stdhttp.StatusNotFound)
	})
}

// Mux is the minimal multiplexer required by the service routes.
type Mux interface {
	Handle(string, stdhttp.Handler)
	ServeHTTP(stdhttp.ResponseWriter, *stdhttp.Request)
}

// Handlers supplies application-specific endpoints to Register.
type Handlers struct {
	GraphQL stdhttp.Handler
	REST    stdhttp.Handler
	WebUI   stdhttp.Handler
	WebUIOn bool
}

const (
	GraphQLPath = "/api/v1/graphql"
	RESTPath    = "/api/v1/rest/"
	OpenAPIPath = "/api/v1/openapi.json"
)

// Register installs the standard GraphJin endpoints.
func Register(mux Mux, handlers Handlers) Mux {
	mux.Handle("/health", stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	mux.Handle(GraphQLPath, handlers.GraphQL)
	mux.Handle(RESTPath, handlers.REST)
	if handlers.WebUIOn && handlers.WebUI != nil {
		mux.Handle("/", handlers.WebUI)
	}
	return mux
}
