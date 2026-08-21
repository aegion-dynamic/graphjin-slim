// Package webui serves the embedded GraphJin web console.
//
// The console is a small React application whose production build lives in
// assets/build. It ships as its own module so that applications which do not
// import it keep zero UI assets in their binary.
//
// Wire it into the service with:
//
//	serv.NewGraphJinService(conf, serv.OptionSetWebUI(webui.Handler))
//
// The service mounts the handler at "/" when config enables it (web_ui).
// The console discovers the GraphQL endpoint from the "?endpoint=" query
// parameter appended by Handler's root redirect.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:assets/build
var buildFS embed.FS

// Handler returns the web UI http.Handler for the given route prefix and
// GraphQL endpoint path (e.g. "/api/v1/graphql").
func Handler(routePrefix string, gqlEndpoint string) http.Handler {
	webRoot, err := fs.Sub(buildFS, "assets/build")
	if err != nil {
		return unavailable()
	}
	fileServer := http.FileServer(http.FS(webRoot))

	if !strings.HasSuffix(routePrefix, "/") {
		routePrefix += "/"
	}
	prefix := routePrefix

	return http.StripPrefix(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Redirect the bare UI root to the configured GraphQL endpoint so
		// the single-page app knows where to send queries.
		if r.URL.Path == "/" && r.URL.RawQuery == "" {
			http.Redirect(w, r, "/?endpoint="+gqlEndpoint, http.StatusMovedPermanently)
			return
		}
		// SPA fallback: unknown extension-less paths render the app shell.
		if !strings.Contains(r.URL.Path, ".") {
			if index, ierr := fs.ReadFile(webRoot, "index.html"); ierr == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(index)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	}))
}

func unavailable() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "web ui assets not available", http.StatusNotFound)
	})
}
