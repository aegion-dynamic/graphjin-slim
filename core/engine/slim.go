package engine

import (
	"context"
	"encoding/json"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

type ResolverFn func(v ResolverProps) (Resolver, error)

type resItem struct {
	IDField     []byte
	Path        [][]byte
	Fn          Resolver
	Source      string
	Scope       string
	Fingerprint string
}

type RootLimitInfo struct {
	FieldName, Table, Database   string
	Path                         []string
	Limit                        int32
	NoLimit, Aggregate, Singular bool
}

func rootLimitInfoFromQCode(*qcode.QCode) []RootLimitInfo { return nil }

func (r *Result) RootLimits() []RootLimitInfo {
	if r == nil {
		return nil
	}
	return r.rootLimits
}

var (
	errResolversDisabled     = simpleErr("remote resolvers are not supported")
	errRemoteJoinsDisabled   = simpleErr("remote joins are not supported")
	errSubscriptionsDisabled = simpleErr("subscriptions are not supported")
)

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

func (gj *graphjinEngine) newRTMap() map[string]ResolverFn { return map[string]ResolverFn{} }

func (gj *graphjinEngine) initResolvers() error {
	gj.rmap = map[string]resItem{}
	gj.rtmap = gj.newRTMap()
	if gj.conf != nil && len(gj.conf.Resolvers) != 0 {
		return errResolversDisabled
	}
	return nil
}

func (gj *graphjinEngine) applySourceAccessRules(*sdata.DBInfo, string) error { return nil }

func (s *gstate) execRemoteJoin(context.Context) error {
	if s != nil && s.cs != nil && s.cs.st.qc != nil && s.cs.st.qc.Remotes > 0 {
		return errRemoteJoinsDisabled
	}
	return nil
}

func injectRemoteMarkers(data []byte, _ *qcode.QCode) []byte { return data }
func seedRemotePlaceholders(_ *qcode.QCode) []byte           { return []byte(`{}`) }

type Member struct{}

func (m *Member) Close() {}

func (g *GraphJin) Subscribe(context.Context, string, json.RawMessage, *RequestConfig) (*Member, error) {
	return nil, errSubscriptionsDisabled
}

func (g *GraphJin) SubscribeByName(context.Context, string, json.RawMessage, *RequestConfig) (*Member, error) {
	return nil, errSubscriptionsDisabled
}
