package serv

import "net/http"

// Mux is the interface for HTTP multiplexers used by GraphJin.
type Mux interface {
	Handle(string, http.Handler)
	ServeHTTP(http.ResponseWriter, *http.Request)
}

const (
	routeGraphQL = "/api/v1/graphql"
	routeREST    = "/api/v1/rest/"
	routeOpenAPI = "/api/v1/openapi.json"
)

// routesHandler registers the core API routes on the provided mux.
func routesHandler(s1 *HttpService, mux Mux, ns *string) (Mux, error) {
	// Health check (always available)
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// GraphQL endpoint
	gqlHandler := s1.apiV1GraphQL(ns, nil)
	mux.Handle(routeGraphQL, gqlHandler)

	// REST endpoint
	restHandler := s1.apiV1Rest(ns, nil)
	mux.Handle(routeREST, restHandler)

	// WebUI (dev only)
	gs := s1.Load().(*graphjinService)
	if gs.conf.Serv.WebUI {
		mux.Handle("/", s1.WebUI("/", routeGraphQL))
	}

	return mux, nil
}
