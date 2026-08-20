package core

import (
	"context"
	"net/http"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/query"
)

// remoteAPI wraps query.RemoteAPI and adapts it to core.Resolver.
type remoteAPI struct {
	inner *query.RemoteAPI
}

func newRemoteAPI(v map[string]interface{}, httpClient *http.Client) (*remoteAPI, error) {
	inner, err := query.NewRemoteAPI(v, httpClient)
	if err != nil {
		return nil, err
	}
	return &remoteAPI{inner: inner}, nil
}

func (r *remoteAPI) Resolve(c context.Context, rr ResolverReq) ([]byte, error) {
	return r.inner.Resolve(c, query.RemoteRequest{ID: rr.ID, Log: rr.Log})
}
