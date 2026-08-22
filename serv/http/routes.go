// Package http contains the service HTTP routing contract.
package http

import (
	stdhttp "net/http"

	"github.com/aegion-dynamic/graphjin-slim/serv/v3/module"
)

// Mux is the minimal multiplexer required by the service routes.
type Mux interface {
	Handle(string, stdhttp.Handler)
	ServeHTTP(stdhttp.ResponseWriter, *stdhttp.Request)
}

// Handlers supplies application-specific endpoints to Register. Optional
// product surfaces arrive as ModuleRoutes; this struct knows nothing about
// which modules exist.
type Handlers struct {
	GraphQL      stdhttp.Handler
	REST         stdhttp.Handler
	ModuleRoutes []module.Route
	Queries      stdhttp.Handler
}

const (
	GraphQLPath = "/api/v1/graphql"
	RESTPath    = "/api/v1/rest/"
	QueriesPath = "/api/v1/queries"
)

// Register installs the standard GraphJin endpoints followed by any routes
// contributed by mounted modules. A module route is only mounted when its
// handler is non-nil, matching the slim-build default of absent product
// surfaces.
func Register(mux Mux, handlers Handlers) Mux {
	mux.Handle("/health", stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	mux.Handle(GraphQLPath, handlers.GraphQL)
	mux.Handle(RESTPath, handlers.REST)
	if handlers.Queries != nil {
		mux.Handle(QueriesPath, handlers.Queries)
	}
	for _, r := range handlers.ModuleRoutes {
		if r.Path == "" || r.Handler == nil {
			continue
		}
		mux.Handle(r.Path, r.Handler)
	}
	return mux
}
