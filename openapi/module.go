package openapi

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/aegion-dynamic/graphjin-slim/serv/v3/module"
)

// mod adapts the spec generator to the service module seam.
type mod struct {
	cfg Config
}

// SpecPath is the HTTP path the spec endpoint is mounted at.
const SpecPath = "/api/v1/openapi.json"

// Module returns the OpenAPI export as an optional service module. Mount it
// with:
//
//	serv.NewGraphJinService(conf,
//	    serv.OptionSetModule(openapi.Module(openapi.Config{Title: "My API"})))
//
// Recognized Settings keys (config section `modules.openapi:`):
//
//	specs_dir string   write openapi.json (or <namespace>.openapi.json) to
//	                   this directory at startup, for SDK codegen pipelines
func Module(cfg Config) module.Module { return mod{cfg: cfg} }

// Name matches the `modules:` config section key.
func (m mod) Name() string { return "openapi" }

// Mount resolves settings and builds the export contribution. When no
// engine is available (dev mode without a database) or no specs_dir is set,
// the module stays inert apart from serving the live endpoint.
//
// The spec file is written once per mount; failures are logged and never
// block startup, matching codegen-pipeline semantics.
func (m mod) Mount(ctx module.Context) (module.Contribution, error) {
	if !ctx.Enabled() {
		return module.Contribution{}, nil
	}

	ns := ctx.Namespace
	if dir, ok := ctx.String("specs_dir"); ok && dir != "" && ctx.Engine != nil {
		m.writeSpecs(ctx, dir, ns)
	}
	if ctx.Engine == nil {
		return module.Contribution{}, nil // nothing to describe yet
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusOK)
			return
		case http.MethodGet:
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		spec, err := GenerateJSON(ctx.Engine, ns, m.cfg)
		if err != nil {
			ctx.Logger.Errorf("failed to generate OpenAPI spec: %s", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(spec); err != nil {
			ctx.Logger.Errorf("failed to write OpenAPI spec: %s", err)
		}
	})

	return module.Contribution{
		Routes: []module.Route{{Path: SpecPath, Handler: handler}},
	}, nil
}

// writeSpecs writes the spec document into dir at mount time.
func (m mod) writeSpecs(ctx module.Context, dir string, ns *string) {
	name := "openapi.json"
	if ns != nil && *ns != "" {
		name = *ns + ".openapi.json"
	}
	spec, err := GenerateJSON(ctx.Engine, ns, m.cfg)
	if err != nil {
		ctx.Logger.Errorf("failed to generate OpenAPI spec for %s: %s", name, err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		ctx.Logger.Errorf("failed to create OpenAPI specs dir %q: %s", dir, err)
		return
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, spec, 0o644); err != nil {
		ctx.Logger.Errorf("failed to write OpenAPI spec %q: %s", path, err)
		return
	}
	ctx.Logger.Infof("OpenAPI spec written to %s", path)
}
