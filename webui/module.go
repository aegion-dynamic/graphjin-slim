package webui

import (
	servhttp "github.com/aegion-dynamic/graphjin-slim/serv/v3/http"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/module"
)

// mod adapts the embedded console to the service module seam.
type mod struct{}

// Module returns the console as an optional service module. Mount it with:
//
//	serv.NewGraphJinService(conf, serv.OptionSetModule(webui.Module()))
//
// Recognized Settings keys (config section `modules.webui:`):
//
//	enabled bool   mount the console route (default false)
func Module() module.Module { return mod{} }

// Name matches the `modules:` config section key.
func (mod) Name() string { return "webui" }

// Mount resolves settings and builds the console contribution. Without an
// enabled section it stays inert: no routes, no error.
func (mod) Mount(ctx module.Context) (module.Contribution, error) {
	if !ctx.Enabled() {
		return module.Contribution{}, nil
	}
	return module.Contribution{
		Routes: []module.Route{
			{Path: "/", Handler: Handler("/", servhttp.GraphQLPath)},
		},
		DevLinks: []module.DevLink{{Label: "Web UI", Path: "/"}},
	}, nil
}
