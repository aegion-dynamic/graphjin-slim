package query

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/jsn"
)

// RemoteAPI is a simple HTTP GET remote resolver endpoint.
type RemoteAPI struct {
	httpClient *http.Client
	URL        string
	Debug      bool

	PassHeaders []string
	SetHeaders  []RemoteHeader
}

// RemoteHeader is a fixed request header applied by a remote API resolver.
type RemoteHeader struct {
	Name  string
	Value string
}

// RemoteRequest is the minimal request surface needed by RemoteAPI.Resolve.
type RemoteRequest struct {
	ID  string
	Log *log.Logger
}

// NewRemoteAPI creates a remote API endpoint from resolver props.
func NewRemoteAPI(v map[string]interface{}, httpClient *http.Client) (*RemoteAPI, error) {
	ra := RemoteAPI{
		httpClient: httpClient,
	}

	if v, ok := v["url"].(string); ok {
		ra.URL = v
	}
	if v, ok := v["debug"].(bool); ok {
		ra.Debug = v
	}
	if v, ok := v["pass_headers"].([]string); ok {
		ra.PassHeaders = v
	}
	if v, ok := v["set_headers"].(map[string]string); ok {
		for k, v1 := range v {
			ra.SetHeaders = append(ra.SetHeaders, RemoteHeader{Name: k, Value: v1})
		}
	}

	return &ra, nil
}

// Resolve resolves a remote API request.
func (r *RemoteAPI) Resolve(c context.Context, rr RemoteRequest) ([]byte, error) {
	uri := strings.ReplaceAll(r.URL, "$id", rr.ID)

	req, err := http.NewRequestWithContext(c, "GET", uri, nil)
	if err != nil {
		return nil, err
	}

	for _, v := range r.SetHeaders {
		req.Header.Set(v.Name, v.Value)
	}

	res, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to '%s': %v", uri, err)
	}
	defer res.Body.Close() //nolint:errcheck

	if r.Debug {
		reqDump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			return nil, err
		}

		resDump, err := httputil.DumpResponse(res, true)
		if err != nil {
			return nil, err
		}

		if rr.Log != nil {
			rr.Log.Printf("DBG Remote Request:\n%s\n%s",
				reqDump, resDump)
		}
	}

	if res.StatusCode != 200 {
		return nil,
			fmt.Errorf("server responded with a %d", res.StatusCode)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if err := jsn.ValidateBytes(b); err != nil {
		return nil, err
	}

	return b, nil
}
