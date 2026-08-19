package serv

import "net/http"

func webuiHandler(routePrefix string, gqlEndpoint string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "web ui not available in slim build", http.StatusNotFound)
	})
}
