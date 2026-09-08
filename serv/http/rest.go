package http

import (
	"errors"
	"path"
	"strings"
)

// ErrNoQueryName is returned when a REST request path has no saved-query name.
var ErrNoQueryName = errors.New("no query name defined")

// QueryName is the saved GraphQL operation a REST request should run.
//
// Embedders mount this handler at /api/v1/rest/ (Register) or /rest/
// (README). The old code sliced RequestURI at len("/api/v1/rest/"), so a
// /rest/GetOrders request ran query "rs" instead of GetOrders.
//
// The name is the path segment after a directory named "rest". Query names
// do not contain slashes.
func QueryName(urlPath string) (string, error) {
	if n := strings.IndexRune(urlPath, '?'); n != -1 {
		urlPath = urlPath[:n]
	}
	urlPath = strings.TrimRight(urlPath, "/")
	name := path.Base(urlPath)
	dir := path.Dir(urlPath)
	if name == "" || name == "." || path.Base(dir) != "rest" {
		return "", ErrNoQueryName
	}
	return name, nil
}
