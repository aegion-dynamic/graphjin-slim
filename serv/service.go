// Package serv provides an API to include and use the GraphJin service with your own code.
// For detailed documentation visit https://graphjin.com
//
// Example usage:
/*
	package main

	import (
		"database/sql"
		"fmt"
			"github.com/aegion-dynamic/graphjin-slim/core/v3"
		_ "github.com/jackc/pgx/v5/stdlib"
	)

	func main() {
		conf := serv.Config{ AppName: "Test App" }
		conf.DB.Host := "127.0.0.1"
		conf.DB.Port := 5432
		conf.DB.DBName := "test_db"
		conf.DB.User := "postgres"
		conf.DB.Password := "postgres"

		gjs, err := serv.NewGraphJinService(conf)
		if err != nil {
			log.Fatal(err)
		}

	 	if err := gjs.Start(); err != nil {
			log.Fatal(err)
		}
	}
*/
package serv

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	// "path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/cache"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
	httpapi "github.com/aegion-dynamic/graphjin-slim/serv/v3/http"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/internal/logging"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// HandlerFunc is the signature for auth handler functions that run before each request.
// In the slim build, auth is handled externally by the caller.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) (context.Context, error)

type HttpService struct {
	atomic.Value
	opt   []Option
	cpath string
}

type servState int

const (
	servStarted servState = iota + 1
	servListening
)

type HookFn func(*core.Result)

const (
	logLevelNone int = iota
	logLevelInfo
	logLevelWarn
	logLevelError
	logLevelDebug
)

func redactRuntimeStringValue(value string) string { return value }

type ResponseCache = cache.ResponseCache
type Mux = httpapi.Mux

const defaultMemoryCacheSize = cache.DefaultMemoryCacheSize

func NewMemoryCache(conf CachingConfig, maxEntries int) (*cache.MemoryCache, error) {
	return cache.NewMemoryCache(conf, maxEntries)
}

func NewRedisCache(redisURL string, conf CachingConfig) (*cache.RedisCache, error) {
	return cache.NewRedisCache(redisURL, conf)
}

func NewDB(conf *Config, openDB bool, log *zap.SugaredLogger, fs core.FS) (*sql.DB, error) {
	return database.Open(database.Options{Config: conf.DB, AppName: conf.AppName, OpenDBName: openDB, Filesystem: fs, Logger: log, Retry: true})
}

func newDB(conf *Config, openDB, _ bool, log *zap.SugaredLogger, fs core.FS) (*sql.DB, error) {
	return NewDB(conf, openDB, log, fs)
}

func newDBOnce(conf *Config, openDB, _ bool, log *zap.SugaredLogger, fs core.FS) (*sql.DB, error) {
	return database.Open(database.Options{Config: conf.DB, AppName: conf.AppName, OpenDBName: openDB, Filesystem: fs, Logger: log})
}

func routesHandler(s1 *HttpService, mux Mux, ns *string) (Mux, error) {
	gs := s1.Load().(*graphjinService)
	return httpapi.Register(mux, httpapi.Handlers{
		GraphQL: s1.apiV1GraphQL(ns, nil),
		REST:    s1.apiV1Rest(ns, nil),
		WebUI:   s1.WebUI("/", httpapi.GraphQLPath),
		WebUIOn: gs.conf.Serv.WebUI,
	}), nil
}

type graphjinService struct {
	artifactProjectionRefreshes atomic.Int64

	log                 *zap.SugaredLogger // logger
	zlog                *zap.Logger        // faster logger
	logLevel            int                // log level
	conf                *Config            // parsed config
	dbs                 map[string]*sql.DB // named database connections (all equal)
	columnValuesMu      sync.Mutex         // guards the enum-value sampling attempt
	columnValuesSampled bool               // true once an attempt actually ran
	columnValues        map[string][]string
	runtimeCore         *core.Config
	secretStore         *localKeystore
	metadataDB          string
	managedArtifactDB   string
	systemNanoDB        *core.NanoDB
	gj                  *core.GraphJin
	srv                 *http.Server
	srvMu               sync.Mutex // guards srv: written by startHTTP, read by Shutdown
	fs                  core.FS
	coreOptions         []core.Option
	closeFn             func()
	chash               string
	state               servState
	hook                HookFn
	prod                bool
	namespace           *string
	tracer              trace.Tracer
	cache               ResponseCache // Response cache (Redis or in-memory)
	configPreviews      *configPreviewStore
	configMu            sync.Mutex
}

// anyDB returns any single connection from the dbs map (for callers
// that just need a live connection, e.g. health checks, DDL, listing).
func (s *graphjinService) anyDB() *sql.DB {
	names := make([]string, 0, len(s.dbs))
	for name := range s.dbs {
		if name != s.managedArtifactDB {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if db := s.dbs[name]; db != nil {
			return db
		}
	}
	return nil
}

// buildCoreOptions returns core.Option slice including OptionSetDatabases.
func (s *graphjinService) buildCoreOptions() []core.Option {
	return s.buildCoreOptionsWithDBs(s.dbs)
}

func (s *graphjinService) buildCoreOptionsWithDBs(dbs map[string]*sql.DB) []core.Option {
	return s.buildCoreOptionsFor(dbs)
}

func (s *graphjinService) buildCoreOptionsFor(dbs map[string]*sql.DB) []core.Option {
	opts := []core.Option{
		core.OptionSetFS(s.fs),
	}
	opts = append(opts, s.coreOptions...)
	return opts
}
func (s *graphjinService) managedSystemRootDatabases(primary string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	add(primary)
	if len(out) == 0 {
		out = append(out, core.DefaultDBName)
	}
	return out
}

type Option func(*graphjinService) error

// NewGraphJinService a new service
func NewGraphJinService(conf *Config, options ...Option) (*HttpService, error) {
	if conf.dirty {
		return nil, errors.New("do not re-use config object")
	}

	s, err := newGraphJinService(conf, nil, options...)
	if err != nil {
		return nil, err
	}

	s1 := &HttpService{opt: options, cpath: conf.ConfigPath}
	s1.Store(s)

	if s.conf.WatchAndReload {
		initConfigWatcher(s1)
	}

	// if s.conf.HotDeploy {
	// 	initHotDeployWatcher(s1)
	// }

	return s1, nil
}

// Close shuts down the in-process service resources owned by HttpService.
// It is useful for embedded use and tests that do not call Start().
func (s *HttpService) Close() error {
	if s == nil {
		return nil
	}
	gs, ok := s.Load().(*graphjinService)
	if !ok || gs == nil {
		return nil
	}
	if gs.gj != nil {
		gs.gj.Close()
	}
	if gs.closeFn != nil {
		gs.closeFn()
	}
	if gs.cache != nil {
		gs.cache.Close() //nolint:errcheck
	}
	for _, db := range gs.dbs {
		if db != nil {
			db.Close() //nolint:errcheck
		}
	}
	return nil
}

// Shutdown gracefully stops the running HTTP server so that a blocking Start
// returns. In-flight requests are drained until the context deadline, after
// which the server is force-closed. It is safe to call more than once and
// before Start (a no-op when the server was never started). Callers that embed
// the service and run their own signal handling use this to unblock Start;
// the service resources (databases, cache, etc.) are then released as Start
// returns, or explicitly via Close.
func (s *HttpService) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	gs, ok := s.Load().(*graphjinService)
	if !ok || gs == nil {
		return nil
	}
	gs.srvMu.Lock()
	srv := gs.srv
	gs.srvMu.Unlock()
	if srv == nil {
		return nil
	}
	if err := srv.Shutdown(ctx); err != nil {
		gs.log.Warnf("graceful shutdown timed out, forcing close: %s", err)
		return srv.Close()
	}
	return nil
}

// closeServResources releases the resources owned by a running service once
// its HTTP listener has stopped: the MCP transport, the user close hook, the
// response cache, runtime event streams and all database connections.
func (s *graphjinService) closeServResources() {
	if s.closeFn != nil {
		s.closeFn()
	}
	if s.gj != nil {
		s.gj.Close()
	}
	if s.cache != nil {
		s.cache.Close() //nolint:errcheck
	}
	for _, db := range s.dbs {
		if db != nil {
			db.Close() //nolint:errcheck
		}
	}
}

// OptionSetDB sets a new db client. The connection is stored under the
// DefaultDBName key in the dbs map for backward compatibility.
func OptionSetDB(db *sql.DB) Option {
	return func(s *graphjinService) error {
		if s.dbs == nil {
			s.dbs = make(map[string]*sql.DB)
		}
		s.dbs[core.DefaultDBName] = db
		return nil
	}
}

// OptionSetDatabases sets named database clients for multi-database embeddings.
func OptionSetDatabases(dbs map[string]*sql.DB) Option {
	return func(s *graphjinService) error {
		if len(dbs) == 0 {
			return nil
		}
		if s.dbs == nil {
			s.dbs = make(map[string]*sql.DB, len(dbs))
		}
		for name, db := range dbs {
			if db != nil {
				s.dbs[name] = db
			}
		}
		return nil
	}
}

// OptionSetHookFunc sets a function to be called on every request
func OptionSetHookFunc(fn HookFn) Option {
	return func(s *graphjinService) error {
		s.hook = fn
		return nil
	}
}

// OptionSetNamespace sets service namespace
func OptionSetNamespace(namespace string) Option {
	return func(s *graphjinService) error {
		s.namespace = &namespace
		return nil
	}
}

// OptionSetFS sets service filesystem
func OptionSetFS(fs core.FS) Option {
	return func(s *graphjinService) error {
		s.fs = fs
		return nil
	}
}

// OptionSetRuntimeSchemaDDLDir sets where the core stores generated
// schema-DDL restart snapshots, relative to the service filesystem root.
func OptionSetRuntimeSchemaDDLDir(dir string) Option {
	return func(s *graphjinService) error {
		s.coreOptions = append(s.coreOptions, core.OptionSetRuntimeSchemaDDLDir(dir))
		return nil
	}
}

// OptionDisableQueryLearning suppresses dev-mode named-query and fragment
// auto-save for this embedded service instance. Existing saved queries remain
// discoverable and executable. This is intended for deterministic evaluation
// and other read-only harnesses whose catalog must not mutate while measured.
func OptionDisableQueryLearning() Option {
	return func(s *graphjinService) error {
		s.coreOptions = append(s.coreOptions, core.OptionSetSavedQuerySaveHook(
			func(context.Context, core.SavedQuerySaveRequest) (bool, error) {
				return true, nil
			},
		))
		return nil
	}
}

// OptionSetZapLogger sets service structured logger
func OptionSetZapLogger(zlog *zap.Logger) Option {
	return func(s *graphjinService) error {
		s.zlog = zlog
		s.log = zlog.Sugar()
		return nil
	}
}

// OptionSetLogOutput sets the log output writer (e.g., os.Stderr for MCP stdio mode)
func OptionSetLogOutput(output zapcore.WriteSyncer) Option {
	return func(s *graphjinService) error {
		zlog := logging.NewLoggerWithOutput(s.conf.ShouldUseJSONLogs(), output)
		s.zlog = zlog
		s.log = zlog.Sugar()
		return nil
	}
}

// OptionDeployActive caused the active config to be deployed on
// func OptionDeployActive() Option {
// 	return func(s *graphjinService) error {
// 		s.deployActive = true
// 		return nil
// 	}
// }

// newGraphJinService creates a new service
func newGraphJinService(conf *Config, dbs map[string]*sql.DB, options ...Option) (*graphjinService, error) {
	var err error
	if conf == nil {
		conf = &Config{Core: Core{Debug: true}}
	}
	if err := normalizeConfigMode(conf); err != nil {
		return nil, err
	}

	zlog := logging.NewLogger(conf.ShouldUseJSONLogs())
	prod := conf.Serv.Production

	s := &graphjinService{
		conf:           conf,
		zlog:           zlog,
		log:            zlog.Sugar(),
		dbs:            dbs,
		configPreviews: newConfigPreviewStore(),
		chash:          conf.hash,
		prod:           prod,
		tracer:         otel.Tracer("graphjin.com/serv"),
	}
	if s.dbs == nil {
		s.dbs = make(map[string]*sql.DB)
	}

	if err := s.initConfig(); err != nil {
		return nil, err
	}

	applySourceCapabilityMCPDefaults(s.conf)

	if err := s.initFS(); err != nil {
		return nil, err
	}

	for _, op := range options {
		if err := op(s); err != nil {
			return nil, err
		}
	}

	initLogLevel(s)
	if err := validateConf(s); err != nil {
		return nil, err
	}

	if err := s.initDB(); err != nil {
		return nil, err
	}

	if err := s.initManagedArtifactStore(); err != nil {
		s.log.Warnf("artifact store init error: %s", err)
	}

	// Initialize Redis cache (non-fatal if unavailable)
	if err := s.initResponseCache(); err != nil {
		s.log.Warnf("response cache init error: %s", err)
	}

	err = s.normalStart()
	if err != nil {
		if !s.conf.Serv.Production {
			s.gj = nil
			s.log.Warnf("GraphJin core initialization failed: %s", err)
		} else {
			return nil, err
		}
	}

	s.state = servStarted
	return s, nil
}

// normalStart starts the service in normal mode
func (s *graphjinService) normalStart() error {
	// Skip GraphJin core initialization if no database is configured (dev mode only)
	if len(s.dbs) == 0 && !s.conf.Serv.Production {
		s.log.Info("GraphJin core not initialized - waiting for database configuration")
		return nil
	}
	if len(s.dbs) == 0 {
		return fmt.Errorf("no database source configured")
	}

	coreConf := &s.conf.Core
	if s.runtimeCore != nil {
		coreConf = s.runtimeCore
	}
	opts := s.buildCoreOptions()

	var err error
	s.gj, err = core.NewGraphJin(coreConf, s.anyDB(), opts...)
	if err != nil {
		return err
	}
	return nil
}

// hotStart starts the service in hot-deploy mode
// func (s *graphjinService) hotStart() error {
// 	ab, err := fetchActiveBundle(s.db)
// 	if err != nil {
// 		if strings.Contains(err.Error(), "_graphjin.") {
// 			return fmt.Errorf("please run 'graphjin init' to setup database for hot-deploy")
// 		}
// 		return err
// 	}
//
// 	if ab == nil {
// 		return s.normalStart()
// 	}
//
// 	cf := s.conf.viper.ConfigFileUsed()
// 	cf = filepath.Base(strings.TrimSuffix(cf, filepath.Ext(cf)))
// 	cf = filepath.Join("/", cf)
//
// 	bfs, err := bundle2Fs(ab.name, ab.hash, cf, ab.bundle)
// 	if err != nil {
// 		return err
// 	}
// 	s.conf = bfs.conf
// 	s.chash = bfs.conf.hash
//
// 	if err := s.initConfig(); err != nil {
// 		return err
// 	}
//
// 	opts := []core.Option{
// 		core.OptionSetFS(newAferoFS(bfs.fs, "/")),
// 		core.OptionSetTrace(trace.NewNoopTracerProvider().Tracer("noop")),
// 	}
//
// 	if s.namespace != nil {
// 		opts = append(opts,
// 			core.OptionSetNamespace(*s.namespace))
// 	}
// 	// Add response cache if enabled
// 	if s.cache != nil {
// 		opts = append(opts, core.OptionSetResponseCache(s.cache))
// 	}
//
// 	s.gj, err = core.NewGraphJin(&s.conf.Core, s.db, opts...)
// 	return err
// }

// Deploy a new configuration
func (s *HttpService) Deploy(conf *Config, options ...Option) error {
	var err error
	os := s.Load().(*graphjinService)

	if conf == nil {
		return nil
	}

	s1, err := newGraphJinService(conf, os.dbs, options...)
	if err != nil {
		return err
	}
	s1.srv = os.srv
	s1.namespace = os.namespace
	if os.closeFn != nil {
		os.closeFn()
	}

	s.Store(s1)
	return nil
}

// Start the service listening on the configured port
func (s *HttpService) Start() error {
	startHTTP(s)
	return nil
}

// Attach route to the internal http service
func (s *HttpService) Attach(mux Mux) error {
	return s.attach(mux, nil)
}

// AttachWithNS a namespaced route to the internal http service
func (s *HttpService) AttachWithNS(mux Mux, namespace string) error {
	return s.attach(mux, &namespace)
}

// attach attaches the service to the router
func (s *HttpService) attach(mux Mux, ns *string) error {
	if _, err := routesHandler(s, mux, ns); err != nil {
		return err
	}

	s1 := s.Load().(*graphjinService)

	ver := version
	dep := s1.conf.name

	if ver == "" {
		ver = "not-set"
	}

	fields := []zapcore.Field{
		zap.String("version", ver),
		zap.String("app-name", s1.conf.AppName),
		zap.String("deployment-name", dep),
		zap.String("env", os.Getenv("GO_ENV")),
		// zap.Bool("hot-deploy", s1.conf.HotDeploy),
		zap.Bool("production", s1.conf.Core.Production),
	}

	if s1.namespace != nil {
		fields = append(fields, zap.String("namespace", *s1.namespace))
	}

	// if s1.conf.HotDeploy {
	// 	fields = append(fields, zap.String("deployment-name", dep))
	// }

	s1.zlog.Info("GraphJin attached to router", fields...)
	return nil
}

// GraphQLis the http handler the GraphQL endpoint
func (s *HttpService) GraphQL(ah HandlerFunc) http.Handler {
	return s.apiHandler(nil, ah, false)
}

// GraphQLWithNS is the http handler the namespaced GraphQL endpoint
func (s *HttpService) GraphQLWithNS(ah HandlerFunc, ns string) http.Handler {
	return s.apiHandler(&ns, ah, false)
}

// REST is the http handler the REST endpoint
func (s *HttpService) REST(ah HandlerFunc) http.Handler {
	return s.apiHandler(nil, ah, true)
}

// RESTWithNS is the http handler the namespaced REST endpoint
func (s *HttpService) RESTWithNS(ah HandlerFunc, ns string) http.Handler {
	return s.apiHandler(&ns, ah, true)
}

// OpenAPI is the http handler for the OpenAPI specification endpoint
func (s *HttpService) OpenAPI() http.Handler {
	return s.openAPIHandler(nil)
}

// OpenAPIWithNS is the http handler for the namespaced OpenAPI specification endpoint
func (s *HttpService) OpenAPIWithNS(ns string) http.Handler {
	return s.openAPIHandler(&ns)
}

func (s *HttpService) apiHandler(ns *string, ah HandlerFunc, rest bool) http.Handler {
	var h http.Handler
	if rest {
		h = s.apiV1Rest(ns, ah)
	} else {
		h = s.apiV1GraphQL(ns, ah)
	}
	return apiV1Handler(s, ns, h, ah)
}

// WebUI is the http handler the web ui endpoint
func (s *HttpService) WebUI(routePrefix, gqlEndpoint string) http.Handler {
	return webuiHandler(routePrefix, gqlEndpoint)
}

// GetGraphJin fetching internal GraphJin core
func (s *HttpService) GetGraphJin() *core.GraphJin {
	s1 := s.Load().(*graphjinService)
	return s1.gj
}

// GetDB fetching internal db client (returns any connection from the pool)
func (s *HttpService) GetDB() *sql.DB {
	s1 := s.Load().(*graphjinService)
	return s1.anyDB()
}

// Reload re-runs database discover and reinitializes service.
func (s *HttpService) Reload() error {
	s1 := s.Load().(*graphjinService)
	if s1.gj == nil {
		return errors.New("graphjin: engine not initialized")
	}
	return s1.gj.Reload()
}

// spanStart starts the tracer
func (s *graphjinService) spanStart(c context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if s.tracer == nil {
		return otel.Tracer("graphjin").Start(c, name, opts...)
	}
	return s.tracer.Start(c, name, opts...)
}

// spanError records an error in the span
func spanError(span trace.Span, err error) {
	if span.IsRecording() {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// applySourceCapabilityMCPDefaults is a no-op in slim build.
func applySourceCapabilityMCPDefaults(c *Config) {}
