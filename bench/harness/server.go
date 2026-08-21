// Package harness spins up real GraphJin services over plain SQLite files
// and drives them black-box over loopback HTTP.
package harness

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/openapi/v3"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

// Budgets caps how heavy a scenario may get so the suite stays
// laptop-friendly. Extreme raises every cap.
type Budgets struct {
	MaxDepth      int
	MaxRows       int
	Concurrency   int
	Timeout       time.Duration
	HealthTimeout time.Duration
}

func DefaultBudgets() Budgets {
	return Budgets{MaxDepth: 12, MaxRows: 5000, Concurrency: 50, Timeout: 15 * time.Second, HealthTimeout: 15 * time.Second}
}

func ExtremeBudgets() Budgets {
	return Budgets{MaxDepth: 20, MaxRows: 50000, Concurrency: 200, Timeout: 60 * time.Second, HealthTimeout: 30 * time.Second}
}

// SeedQuery is written into config/queries before the service starts.
type SeedQuery struct {
	Query string
	Vars  map[string]any
}

type Opts struct {
	Name    string // scenario name (used for the db file)
	Prod    bool   // production mode
	Schema  string // "shop" | "chain" | "blob"
	ChainN  int    // table count for Schema=="chain"
	Seeds   map[string]SeedQuery
	Budgets Budgets
}

// size1MB is the blob payload used by the "blob" schema.
const size1MB = 1 << 20

// H is a live handle to one spun-up service.
type H struct {
	Root    string // scenario working directory (also process cwd)
	BaseURL string
	Port    int

	GDB *bun.DB // ground-truth handle on the scenario sqlite file

	Budgets Budgets

	svc *serv.HttpService
	hc  *http.Client
}

// SpinUp builds the schema, seeds saved queries, starts a real service and
// waits for readiness. The caller must have already chdir'd into its own
// scratch directory.
func SpinUp(o Opts) (*H, error) {
	b := o.Budgets
	if b.Timeout == 0 {
		b = DefaultBudgets()
	}

	dbPath := "bench.db"
	sqldb, derr := sql.Open(database.DriverSQLite, dbPath)
	if derr != nil {
		return nil, derr
	}
	db := bun.NewDB(sqldb, sqlitedialect.New())
	db.SetMaxOpenConns(1)
	defer db.Close()

	switch o.Schema {
	case "", "shop":
		if err := SeedShop(db); err != nil {
			return nil, fmt.Errorf("seed shop: %w", err)
		}
	case "chain":
		n := o.ChainN
		if n == 0 || n > b.MaxDepth {
			n = b.MaxDepth
		}
		if err := SeedChain(db, n); err != nil {
			return nil, fmt.Errorf("seed chain(%d): %w", n, err)
		}
	case "blob":
		if err := SeedBlob(db, size1MB); err != nil {
			return nil, fmt.Errorf("seed blob: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown schema %q", o.Schema)
	}

	qdir := filepath.Join("config", "queries")
	if err := os.MkdirAll(qdir, 0o755); err != nil {
		return nil, err
	}
	for name, sq := range o.Seeds {
		if err := os.WriteFile(filepath.Join(qdir, name+".gql"), []byte(sq.Query), 0o644); err != nil {
			return nil, err
		}
		if len(sq.Vars) != 0 {
			vb, _ := json.Marshal(sq.Vars)
			if err := os.WriteFile(filepath.Join(qdir, name+".json"), vb, 0o644); err != nil {
				return nil, err
			}
		}
	}

	port, err := freePort()
	if err != nil {
		return nil, err
	}

	conf := serv.Config{}
	conf.Serv.AppName = "bench-" + o.Name
	conf.Serv.Production = o.Prod
	conf.Serv.WebUI = !o.Prod
	conf.Serv.OpenAPISpecsDir = "./specs"
	conf.Serv.HostPort = fmt.Sprintf("127.0.0.1:%d", port)
	conf.DB.Type = "sqlite"
	conf.DB.Path = dbPath

	gjs, err := serv.NewGraphJinService(&conf,
		serv.OptionSetOpenAPI(openapi.Generator(openapi.Config{Title: "GraphJin Bench"})),
	)
	if err != nil {
		return nil, fmt.Errorf("new service: %w", err)
	}
	go func() { _ = gjs.Start() }()

	h := &H{
		Root:    mustCwd(),
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		Port:    port,
		GDB:     db,
		svc:     gjs,
		hc:      &http.Client{Timeout: b.Timeout},
		Budgets: b,
	}
	if err := h.waitHealthy(b.HealthTimeout); err != nil {
		h.Stop()
		return nil, err
	}
	return h, nil
}

func (h *H) waitHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(h.BaseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("service not healthy after %s", timeout)
}

// Stop shuts the service down. Safe to call more than once.
func (h *H) Stop() {
	if h == nil || h.svc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = h.svc.Shutdown(ctx)
	_ = h.svc.Close()
	h.svc = nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func mustCwd() string {
	d, _ := os.Getwd()
	return d
}
