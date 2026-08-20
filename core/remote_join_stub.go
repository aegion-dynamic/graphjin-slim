package core

import (
	"context"
	"errors"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/qcode"
)

var errRemoteJoinsDisabled = errors.New("remote joins are not supported")

// Remote joins (HTTP/OpenAPI resolvers) are not supported in slim.
// Cross-database SQL joins remain in database_join.go.

func (s *gstate) execRemoteJoin(c context.Context) error {
	if s == nil || s.cs == nil || s.cs.st.qc == nil {
		return nil
	}
	if s.cs.st.qc.Remotes > 0 {
		return errRemoteJoinsDisabled
	}
	return nil
}

func injectRemoteMarkers(data []byte, _ *qcode.QCode) []byte { return data }

func seedRemotePlaceholders(_ *qcode.QCode) []byte { return []byte(`{}`) }
