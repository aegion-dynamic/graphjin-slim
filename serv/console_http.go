package serv

import (
	"encoding/json"
	"net/http"

	core "github.com/aegion-dynamic/graphjin-slim/core/v3"
)

// queriesHandler serves the saved-query management API used by the web
// console: list, save (with default variables), and delete.
func (s1 *HttpService) queriesHandler(ns *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := s1.Load().(*graphjinService)
		if s.gj == nil {
			http.Error(w, "graphjin engine not initialized", http.StatusServiceUnavailable)
			return
		}
		gj := s.gj

		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			switch {
			case r.URL.Query().Get("name") != "":
				d, err := gj.SavedQuery(r.URL.Query().Get("name"))
				if err != nil {
					httpError(w, err)
					return
				}
				_ = json.NewEncoder(w).Encode(d)
			default:
				list, err := gj.SavedQueries()
				if err != nil {
					httpError(w, err)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"queries": list})
			}

		case http.MethodPost:
			var d core.SavedQueryDetails
			if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
				http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
				return
			}
			if ns != nil && d.Namespace == "" {
				d.Namespace = *ns
			}
			if err := gj.SaveQuery(&d); err != nil {
				httpError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))

		case http.MethodDelete:
			name := r.URL.Query().Get("name")
			if name == "" {
				http.Error(w, "missing query parameter: name", http.StatusBadRequest)
				return
			}
			if err := gj.DeleteSavedQuery(name); err != nil {
				httpError(w, err)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))

		default:
			w.Header().Set("Allow", "GET, POST, DELETE")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func httpError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
