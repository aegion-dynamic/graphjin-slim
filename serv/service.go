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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/etags"
	httpapi "github.com/aegion-dynamic/graphjin-slim/serv/v3/http"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/lifecycle"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/logging"
	"github.com/klauspost/compress/gzhttp"
	"github.com/rs/cors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
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

type Mux = httpapi.Mux

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
	s := s1.Load().(*graphjinService)
	h := httpapi.Handlers{
		GraphQL: s1.apiV1GraphQL(ns, nil),
		REST:    s1.apiV1Rest(ns, nil),
	}
	if s.conf.WebUI && s.webUIFn != nil {
		h.WebUIOn = true
		h.WebUI = s.webUIFn("/", httpapi.GraphQLPath)
	}
	if s.openAPIGen != nil {
		h.OpenAPI = s1.openAPIHandler(ns)
	}
	h.Queries = s1.queriesHandler(ns)
	return httpapi.Register(mux, h), nil
}

type graphjinService struct {
	log                 *zap.SugaredLogger // logger
	zlog                *zap.Logger        // faster logger
	logLevel            int                // log level
	conf                *Config            // parsed config
	dbs                 map[string]*sql.DB // named database connections (all equal)
	columnValuesMu      sync.Mutex         // guards the enum-value sampling attempt
	columnValuesSampled bool               // true once an attempt actually ran
	columnValues        map[string][]string
	runtimeCore         *core.Config
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
	configMu            sync.Mutex
	webUIFn             webUIFactory                                                 // optional embedded UI handler factory (nil = unavailable)
	openAPIGen          func(gj *core.GraphJin, ns *string) (json.RawMessage, error) // optional OpenAPI spec generator (nil = unavailable)
}

// webUIFactory builds the embedded web UI handler for a route prefix and
// GraphQL endpoint path. Supplied by the application (e.g. the webui module);
// the slim default is nil, which keeps the UI endpoint unavailable.
type webUIFactory func(routePrefix, gqlEndpoint string) http.Handler

// anyDB returns any single connection from the dbs map (for callers
// that just need a live connection, e.g. health checks, DDL, listing).
func (s *graphjinService) anyDB() *sql.DB {
	names := make([]string, 0, len(s.dbs))
	for name := range s.dbs {
		if true {
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

// OptionSetWebUI registers a factory that builds the embedded web UI handler.
// The application supplies the factory (typically from the webui module); the
// service only mounts the UI when config enables it (web_ui). Without this
// option the web UI endpoint stays unavailable.
func OptionSetWebUI(fn func(routePrefix, gqlEndpoint string) http.Handler) Option {
	return func(s *graphjinService) error {
		s.webUIFn = fn
		return nil
	}
}

// OptionSetOpenAPI registers the OpenAPI specification generator. The
// generator receives the engine and an optional namespace, and returns the
// spec document as JSON. Without this option the OpenAPI endpoint stays
// unavailable.
func OptionSetOpenAPI(gen func(gj *core.GraphJin, ns *string) (json.RawMessage, error)) Option {
	return func(s *graphjinService) error {
		s.openAPIGen = gen
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
		conf:   conf,
		zlog:   zlog,
		log:    zlog.Sugar(),
		dbs:    dbs,
		chash:  conf.hash,
		prod:   prod,
		tracer: otel.Tracer("graphjin.com/serv"),
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
	s.writeOpenAPISpecs()
	return nil
}

// writeOpenAPISpecs writes the OpenAPI specification to the configured
// directory (serv.OpenAPISpecsDir) once at startup. It is intended for SDK
// codegen pipelines that consume the spec from source control or CI.
// Requires a generator registered via OptionSetOpenAPI. Failures are logged
// and do not block startup.
func (s *graphjinService) writeOpenAPISpecs() {
	dir := s.conf.Serv.OpenAPISpecsDir
	if dir == "" || s.openAPIGen == nil || s.gj == nil {
		return
	}
	name := "openapi.json"
	if s.namespace != nil && *s.namespace != "" {
		name = *s.namespace + ".openapi.json"
	}
	spec, err := s.openAPIGen(s.gj, s.namespace)
	if err != nil {
		s.log.Errorf("failed to generate OpenAPI spec for %s: %s", name, err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.log.Errorf("failed to create OpenAPI specs dir %q: %s", dir, err)
		return
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, spec, 0o644); err != nil {
		s.log.Errorf("failed to write OpenAPI spec %q: %s", path, err)
		return
	}
	s.log.Infof("OpenAPI spec written to %s", path)
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
// 	}
//
// 	s.gj, err = core.NewGraphJin(&s.conf.Core, s.db, opts...)
// 	return err
// }

// Deploy a new configuration
func (s *HttpService) Deploy(conf *Config, options ...Option) error {
	return fmt.Errorf("deploy is not supported in slim build")
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

func (s *HttpService) apiHandler(ns *string, ah HandlerFunc, rest bool) http.Handler {
	var h http.Handler
	if rest {
		h = s.apiV1Rest(ns, ah)
	} else {
		h = s.apiV1GraphQL(ns, ah)
	}
	return apiV1Handler(s, ns, h, ah)
}

// WebUI is the http handler the web ui endpoint. When no web UI factory was
// registered via OptionSetWebUI, the slim-build unavailable handler is
// returned.
func (s *HttpService) WebUI(routePrefix, gqlEndpoint string) http.Handler {
	s1 := s.Load().(*graphjinService)
	if s1.webUIFn != nil {
		return s1.webUIFn(routePrefix, gqlEndpoint)
	}
	return httpapi.UnavailableWebUI(routePrefix, gqlEndpoint)
}

// OpenAPI is the http handler for the OpenAPI specification endpoint.
func (s *HttpService) OpenAPI() http.Handler {
	return s.openAPIHandler(nil)
}

// OpenAPIWithNS is the http handler for the namespaced OpenAPI specification endpoint
func (s *HttpService) OpenAPIWithNS(ns string) http.Handler {
	return s.openAPIHandler(&ns)
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

// initLogLevel initializes the log level
func initLogLevel(s *graphjinService) {
	switch s.conf.LogLevel {
	case "debug":
		s.logLevel = logLevelDebug
	case "error":
		s.logLevel = logLevelError
	case "warn":
		s.logLevel = logLevelWarn
	case "info":
		s.logLevel = logLevelInfo
	default:
		s.logLevel = logLevelNone
	}
}

// validateConf validates the configuration
func validateConf(s *graphjinService) error {

	return nil
}

// initFS initializes the file system
func (s *graphjinService) initFS() error {
	basePath, err := s.basePath()
	if err != nil {
		return err
	}

	err = OptionSetFS(core.NewOsFS(basePath))(s)
	if err != nil {
		return err
	}
	return nil
}

// initConfig initializes the configuration
func (s *graphjinService) initConfig() error {
	c := s.conf
	c.dirty = true
	if err := normalizeConfigMode(c); err != nil {
		return err
	}

	// copy over db_type from database.type
	if c.DBType == "" {
		c.DBType = c.DB.Type
	}

	hp := strings.SplitN(s.conf.HostPort, ":", 2)

	if len(hp) == 2 {
		if s.conf.Host != "" {
			hp[0] = s.conf.Host
		}

		if s.conf.Port != "" {
			hp[1] = s.conf.Port
		}

		s.conf.hostPort = fmt.Sprintf("%s:%s", hp[0], hp[1])
	}

	if s.conf.hostPort == "" {
		s.conf.hostPort = defaultHP
	}

	return nil
}

// ErrGraphJinNotInitialized is returned when GraphJin core is not initialized
var ErrGraphJinNotInitialized = errors.New("GraphJin not initialized - no database configured")

// checkGraphJinInitialized returns an error if GraphJin core is not initialized
func (s *graphjinService) checkGraphJinInitialized() error {
	if s.gj == nil {
		return ErrGraphJinNotInitialized
	}
	return nil
}

// isDatabaseConfigured checks if a database connection is configured
func (s *graphjinService) isDatabaseConfigured() bool {
	// Check if connection string is provided
	if s.conf.DB.ConnString != "" {
		return true
	}
	if s.conf.DB.Path != "" {
		return true
	}
	// Check if host and dbname are provided (minimal required fields for auto-connect)
	if s.conf.DB.Host != "" && s.conf.DB.DBName != "" {
		return true
	}
	// Check if multi-database configs exist with actual connection info
	for _, dbConf := range s.conf.Core.Databases {
		if dbConf.ConnString != "" || dbConf.Host != "" || dbConf.Path != "" {
			return true
		}
	}
	return false
}

// initDB initializes database connections for all entries in conf.Core.Databases.
func (s *graphjinService) initDB() error {
	runtimeCore := cloneCoreConfig(s.conf.Core)
	s.runtimeCore = &runtimeCore

	if len(s.dbs) > 0 && !s.hasDatabaseConfigs() {
		return nil
	}

	// In dev mode, allow starting without a database configured
	if !s.conf.Serv.Production && !s.isDatabaseConfigured() {
		s.log.Warn("No databases configured. Use MCP to add a database configuration.")
		return nil
	}

	// In sources used, absence of SQL/CodeSQL connection details means there is
	// no legacy database to fall back to. Virtual/system sources get a small
	// host database in normalStart when needed.
	if s.conf.Core.IsSourcesUsed() && !s.hasDatabaseConfigs() {
		return nil
	}

	// If there are entries in conf.Core.Databases with connection info, use them.
	// Otherwise fall back to the legacy single-DB path via conf.DB.
	if s.hasDatabaseConfigs() {
		return s.initAllDBs()
	}

	// Legacy single-DB path: create one connection from conf.DB
	return s.initLegacyDB()
}

// hasDatabaseConfigs returns true if any entry in conf.Core.Databases
// has enough info to create a connection.
func (s *graphjinService) hasDatabaseConfigs() bool {
	for _, dbConf := range s.conf.Core.Databases {
		if dbConf.ConnString != "" || dbConf.Host != "" || dbConf.Path != "" {
			return true
		}
	}
	return false
}

// initAllDBs creates connections for every entry in conf.Core.Databases.
func (s *graphjinService) initAllDBs() error {
	dbNames := make([]string, 0, len(s.conf.Core.Databases))
	for name := range s.conf.Core.Databases {
		dbNames = append(dbNames, name)
	}
	sort.Strings(dbNames)
	for _, name := range dbNames {
		dbConf := s.conf.Core.Databases[name]
		runtimeDBConf := dbConf
		if s.runtimeCore != nil && s.runtimeCore.Databases != nil {
			if hydrated, ok := s.runtimeCore.Databases[name]; ok {
				runtimeDBConf = hydrated
			}
		}
		if _, ok := s.dbs[name]; ok {
			// runtime event removed
			continue
		}
		db, err := s.newDBFromDatabaseConfigInto(name, runtimeDBConf, s.runtimeCore)
		if err != nil {
			// runtime event removed
			if s.conf.Serv.Production {
				return fmt.Errorf("database %s: %s", name, redactRuntimeStringValue(err.Error()))
			}
			s.log.Warnf("Database '%s' connection failed: %s. Skipping.", name, redactRuntimeStringValue(err.Error()))
			continue
		}
		s.dbs[name] = db
		// runtime event removed
	}
	// Sync legacy conf.DB from first database for code that still reads it
	if len(s.dbs) > 0 {
		syncRuntimeDBFromDatabases(s.conf, s.runtimeCore)
	}
	return nil
}

// initLegacyDB creates a single connection from the legacy conf.DB fields.
func (s *graphjinService) initLegacyDB() error {
	if isCodeSQLType(s.conf.DB.Type) || isCodeSQLType(s.conf.DBType) {
		return fmt.Errorf("codesql databases are not supported in slim build")
	}

	var db *sql.DB
	var err error

	if s.conf.Serv.Production {
		db, err = newDB(s.conf, true, true, s.log, s.fs)
		if err != nil {
			// runtime event removed
			return fmt.Errorf("%s", redactRuntimeStringValue(err.Error()))
		}
	} else {
		db, err = newDBOnce(s.conf, true, true, s.log, s.fs)
		if err != nil {
			// runtime event removed
			s.log.Warnf("Database connection failed: %s. Server starting without database — use MCP to configure.", redactRuntimeStringValue(err.Error()))
			return nil
		}
	}

	// Store under the first Databases key (sorted for determinism)
	name := core.DefaultDBName
	if len(s.conf.Core.Databases) > 0 {
		names := make([]string, 0, len(s.conf.Core.Databases))
		for n := range s.conf.Core.Databases {
			names = append(names, n)
		}
		sort.Strings(names)
		name = names[0]
	}
	s.dbs[name] = db
	// runtime event removed
	return nil
}

// newDBFromDatabaseConfig creates a *sql.DB from a core.DatabaseConfig.
func (s *graphjinService) newDBFromDatabaseConfig(name string, dbConf core.DatabaseConfig) (*sql.DB, error) {
	return s.newDBFromDatabaseConfigInto(name, dbConf, &s.conf.Core)
}

func (s *graphjinService) newDBFromDatabaseConfigInto(name string, dbConf core.DatabaseConfig, runtimeCore *core.Config) (*sql.DB, error) {
	return database.OpenCore(context.Background(), name, dbConf)
}

// basePath returns the base path
func (s *graphjinService) basePath() (string, error) {
	if s.conf.ConfigPath == "" {
		if cp, err := os.Getwd(); err == nil {
			return filepath.Join(cp, "config"), nil
		} else {
			return "", err
		}
	}
	return s.conf.ConfigPath, nil
}

// cloneCoreConfig creates a copy of a core.Config.
func cloneCoreConfig(c core.Config) core.Config {
	return c
}

// syncRuntimeDBFromDatabases syncs the legacy conf.DB from the first database.
func syncRuntimeDBFromDatabases(conf *Config, runtimeCore *core.Config) {}

// isCodeSQLType checks if the database type is codesql.
func isCodeSQLType(t string) bool {
	return false
}

const dbTypeCodeSQL = "codesql"

// normalizeServiceSources is a no-op in slim build.
func normalizeServiceSources(c *Config) error { return nil }

var version string

const (
	defaultHP = httpapi.DefaultHostPort
)

// Initialize the watcher for the graphjin config file
func initConfigWatcher(s1 *HttpService) {}

// Initialize the hot deploy watcher
// func initHotDeployWatcher(s1 *HttpService) {
// 	s := s1.Load().(*graphjinService)
// 	go func() {
// 		err := startHotDeployWatcher(s1)
// 		if err != nil {
// 			s.log.Fatalf("error in hot deploy watcher: %s", err)
// 		}
// 	}()
// }

// Start the HTTP server
func startHTTP(s1 *HttpService) {
	s := s1.Load().(*graphjinService)

	r := http.NewServeMux()
	routes, err := routesHandler(s1, r, s.namespace)
	if err != nil {
		s.log.Fatalf("error setting up routes: %s", err)
	}

	srv := lifecycle.NewServer(s.conf.hostPort, routes)
	// Publish srv under srvMu so a concurrent Shutdown (signal handler or an
	// external caller, e.g. demo mode) observes it safely.
	s.srvMu.Lock()
	s.srv = srv
	s.srvMu.Unlock()

	// Standalone graceful shutdown: catch SIGINT/SIGTERM and stop the server
	// so Serve (below) returns. Callers that manage their own lifecycle
	// (e.g. demo mode) drive this via HttpService.Shutdown instead; running
	// both paths together is safe since Shutdown is idempotent.
	lifecycle.WatchSignals(s.log, s1.Shutdown)

	ver := version
	// dep := s.conf.name

	if ver == "" {
		ver = "not-set"
	}

	fields := []zapcore.Field{
		zap.String("version", ver),
		zap.String("host-port", s.conf.hostPort),
		zap.String("app-name", s.conf.AppName),
		zap.String("env", os.Getenv("GO_ENV")),
		// zap.Bool("hot-deploy", s.conf.HotDeploy),
		zap.Bool("production", s.conf.Core.Production),
		zap.String("server", "graphjin-slim"),
	}

	if s.namespace != nil {
		fields = append(fields, zap.String("namespace", *s.namespace))
	}

	// if s.conf.HotDeploy {
	// 	fields = append(fields, zap.String("deployment-name", dep))
	// }

	s.zlog.Info("GraphJin started", fields...)
	printDevModeInfo(s)

	l, err := net.Listen("tcp", s.conf.hostPort)
	if err != nil {
		s.log.Fatalf("failed to init port: %s", err)
	}

	// signal we are open for business.
	s.state = servListening

	if err := srv.Serve(l); err != http.ErrServerClosed {
		s.log.Fatalf("failed to start: %s", err)
	}

	// Serve returned because Shutdown (signal handler above or an external
	// HttpService.Shutdown) was requested. Release the service resources.
	s.closeServResources()
	s.log.Info("shutdown complete")
}

// printDevModeInfo prints useful development information on startup
func printDevModeInfo(s *graphjinService) {
	if s.conf.Serv.Production {
		return
	}

	hostPort := s.conf.hostPort
	displayHost := hostPort
	if strings.HasPrefix(hostPort, "0.0.0.0:") {
		displayHost = "localhost" + hostPort[7:]
	}

	fmt.Println()
	fmt.Println("Development Server URLs")
	fmt.Println("───────────────────────")

	if s.conf.WebUI {
		fmt.Printf("  Web UI:      http://%s/\n", displayHost)
	}
	fmt.Printf("  GraphQL:     http://%s/api/v1/graphql\n", displayHost)
	fmt.Printf("  REST API:    http://%s/api/v1/rest/<name>\n", displayHost)
	fmt.Println()
}

const (
	maxReadBytes = 100000 // 100Kb
)

var errUnauthorized = errors.New("not authorized")

type extensions struct {
	Persisted apqExt `json:"persistedQuery"`
}

type apqExt struct {
	Version    int    `json:"version"`
	Sha256Hash string `json:"sha256Hash"`
}

type gqlReq struct {
	OpName string          `json:"operationName"`
	Query  string          `json:"query"`
	Vars   json.RawMessage `json:"variables"`
	Ext    extensions      `json:"extensions"`
}

type errorResp struct {
	Errors []string `json:"errors"`
}

// apiV1Handler is the main handler for all API requests
func apiV1Handler(s1 *HttpService, ns *string, h http.Handler, ah HandlerFunc) http.Handler {
	s := s1.Load().(*graphjinService)

	if len(s.conf.AllowedOrigins) != 0 {
		allowedHeaders := []string{
			"Origin", "Accept", "Content-Type", "X-Requested-With", "Authorization",
		}

		if len(s.conf.AllowedHeaders) != 0 {
			allowedHeaders = s.conf.AllowedHeaders
		}

		c := cors.New(cors.Options{
			AllowedOrigins:   s.conf.AllowedOrigins,
			AllowedHeaders:   allowedHeaders,
			AllowCredentials: true,
			Debug:            s.conf.DebugCORS,
		})
		h = c.Handler(h)
	}

	h = etags.Handler(h, false)

	if s.conf.HTTPGZip {
		gz, err := gzhttp.NewWrapper(
			gzhttp.CompressionLevel(6),
			gzhttp.ExceptContentTypes([]string{"text/event-stream"}),
		)
		if err != nil {
			s.log.Fatalf("api: error with compression: %s", err)
		}
		h = gz(h)
	}

	return h
}

// apiV1GraphQLHandler handles the GraphQL API requests
func (s1 *HttpService) apiV1GraphQL(ns *string, ah HandlerFunc) http.Handler {
	dtrace := otel.GetTextMapPropagator()

	h := func(w http.ResponseWriter, r *http.Request) {
		var err error

		start := time.Now()
		s := s1.Load().(*graphjinService)

		w.Header().Set("Content-Type", "application/json")

		var req gqlReq

		ctx, opts := newDTrace(dtrace, r)
		ctx, span := s.spanStart(ctx, "GraphQL Request", opts...)
		defer span.End()

		switch r.Method {
		case "POST":
			var b []byte
			b, err = io.ReadAll(io.LimitReader(r.Body, maxReadBytes))
			if err == nil {
				defer r.Body.Close() //nolint:errcheck
				err = json.Unmarshal(b, &req)
			}

		case "GET":
			q := r.URL.Query()
			req.Query = q.Get("query")
			req.OpName = q.Get("operationName")
			req.Vars = json.RawMessage(q.Get("variables"))

			if ext := q.Get("extensions"); ext != "" {
				err = json.Unmarshal([]byte(ext), &req.Ext)
			}
		}

		if err != nil {
			spanError(span, err)
			renderErr(w, err)
			return
		}

		var rc core.RequestConfig

		if req.apqEnabled() {
			rc.APQKey = (req.OpName + req.Ext.Persisted.Sha256Hash)
		}

		if rc.Vars == nil && len(s.conf.HeaderVars) != 0 {
			rc.Vars = s.setHeaderVars(r)
		}

		if ns != nil {
			rc.SetNamespace(*ns)
		}
		if req.OpName == "subscription" {
			err := errors.New("subscriptions not supported in slim build")
			spanError(span, err)
			return
		}

		if err := s.checkGraphJinInitialized(); err != nil {
			renderErr(w, err)
			return
		}

		res, err := s.gj.GraphQL(ctx, req.Query, req.Vars, &rc)
		if res == nil && err != nil {
			renderErr(w, err)
			return
		}

		s.responseHandler(
			ctx,
			w,
			r,
			start,
			rc,
			res,
			err)

		if span.IsRecording() {
			span.SetAttributes(
				attribute.String("http.path", r.RequestURI),
				attribute.String("http.method", r.Method),
				attribute.Bool("query.apq", req.apqEnabled()))
		}

		if err != nil {
			spanError(span, err)
		}
	}
	return http.HandlerFunc(h)
}

// apiV1Rest returns a handler that handles the REST API requests
func (s1 *HttpService) apiV1Rest(ns *string, ah HandlerFunc) http.Handler {
	rLen := len(httpapi.RESTPath)
	dtrace := otel.GetTextMapPropagator()

	h := func(w http.ResponseWriter, r *http.Request) {
		var err error

		start := time.Now()
		s := s1.Load().(*graphjinService)

		w.Header().Set("Content-Type", "application/json")

		var vars json.RawMessage
		var span trace.Span

		ctx, opts := newDTrace(dtrace, r)
		ctx, span = s.spanStart(ctx, "REST Request", opts...)
		defer span.End()

		if len(r.RequestURI) <= rLen {
			err := errors.New("no query name defined")
			spanError(span, err)
			renderErr(w, err)
			return
		}

		queryName := r.RequestURI[rLen:]
		if n := strings.IndexRune(queryName, '?'); n != -1 {
			queryName = queryName[:n]
		}

		switch r.Method {
		case "POST":
			vars, err = parseBody(r)

		case "GET":
			// Variables arrive as individual parameters matching the
			// saved query's variable names, exactly as advertised in the
			// generated OpenAPI spec.
			q := r.URL.Query()
			m := make(map[string]any, len(q))
			for k := range q {
				raw := q.Get(k)
				var vv any
				if json.Unmarshal([]byte(raw), &vv) == nil {
					m[k] = vv
				} else {
					m[k] = raw
				}
			}
			if len(m) != 0 {
				vars, err = json.Marshal(m)
			}
		}

		if err != nil {
			spanError(span, err)
			renderErr(w, err)
			return
		}

		var rc core.RequestConfig

		if rc.Vars == nil && len(s.conf.HeaderVars) != 0 {
			rc.Vars = s.setHeaderVars(r)
		}

		if ns != nil {
			rc.SetNamespace(*ns)
		}

		if err := s.checkGraphJinInitialized(); err != nil {
			renderErr(w, err)
			return
		}

		res, err := s.gj.GraphQLByName(ctx, queryName, vars, &rc)
		s.responseHandler(
			ctx,
			w,
			r,
			start,
			rc,
			res,
			err)

		if span.IsRecording() {
			span.SetAttributes(
				attribute.String("http.path", r.RequestURI),
				attribute.String("http.method", r.Method))
		}

		if err != nil {
			spanError(span, err)
		}
	}
	return http.HandlerFunc(h)
}

// responseHandler handles the response from the GraphQL API
func (s *graphjinService) responseHandler(ct context.Context,
	w http.ResponseWriter,
	r *http.Request,
	start time.Time,
	rc core.RequestConfig,
	res *core.Result,
	err error,
) {
	if s.hook != nil {
		s.hook(res)
	}

	if err == nil && r.Method == "GET" && res.Operation() == core.OpQuery {
		switch {
		case res.CacheControl() != "":
			w.Header().Set("Cache-Control", res.CacheControl())

		case s.conf.CacheControl != "":
			w.Header().Set("Cache-Control", s.conf.CacheControl)
		}

		w.Header().Set("ETag", hex.EncodeToString(res.Hash[:]))
	}

	if err := json.NewEncoder(w).Encode(res); err != nil {
		renderErr(w, err)
		return
	}

	rt := time.Since(start).Milliseconds()

	if s.logLevel >= logLevelInfo {
		s.reqLog(res, rc, rt, err)
	}

	if s.conf.ServerTiming {
		b := []byte("DB;dur=")
		b = strconv.AppendInt(b, rt, 10)
		w.Header().Set("Server-Timing", string(b))
	}
}

// reqLog logs the request details
func (s *graphjinService) reqLog(res *core.Result, rc core.RequestConfig, resTimeMs int64, err error) {
	var fields []zapcore.Field
	var sql string

	if res != nil {
		sql = res.SQL()
		fields = []zapcore.Field{
			zap.String("op", res.OperationName()),
			zap.String("name", res.QueryName()),
			zap.Int64("responseTimeMs", resTimeMs),
			zap.Bool("cacheHit", res.CacheHit()),
		}
	}

	if ns, ok := rc.GetNamespace(); ok {
		fields = append(fields, zap.String("namespace", ns))
	}

	if res != nil && res.Vars != nil && s.conf.LogVars {
		var vars map[string]interface{}
		err := json.Unmarshal(res.Vars, &vars)
		if err != nil {
			s.log.Error("failed to unmarshal sql vars", zap.Error(err))
		}
		fields = append(fields, zap.Any("vars", vars))
	}

	if sql != "" && s.logLevel >= logLevelDebug {
		fields = append(fields, zap.String("sql", sql))
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		s.zlog.Error("query failed", fields...)
	} else {
		s.zlog.Info("query", fields...)
	}
}

// setHeaderVars sets the header variables
func (s *graphjinService) setHeaderVars(r *http.Request) map[string]interface{} {
	vars := make(map[string]interface{})
	for k, v := range s.conf.HeaderVars {
		vars[k] = func() string {
			if v1, ok := r.Header[v]; ok {
				return v1[0]
			}
			return ""
		}
	}
	return vars
}

// apqEnabled checks if the APQ is enabled
func (r gqlReq) apqEnabled() bool {
	return r.Ext.Persisted.Sha256Hash != ""
}

// renderErr renders the error response
func renderErr(w http.ResponseWriter, err error) {
	if err == errUnauthorized {
		w.WriteHeader(http.StatusUnauthorized)
	}

	err1 := json.NewEncoder(w).Encode(errorResp{[]string{err.Error()}})
	if err1 != nil {
		panic(fmt.Errorf("%s: %w", err, err1))
	}
}

// parseBody parses the request body
func parseBody(r *http.Request) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r.Body, maxReadBytes))
	if err != nil {
		return nil, err
	}
	defer r.Body.Close() //nolint:errcheck
	return b, nil
}

// newDTrace creates a new DTrace
func newDTrace(dtrace propagation.TextMapPropagator, r *http.Request) (context.Context, []trace.SpanStartOption) {
	ctx := dtrace.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	atrr := []attribute.KeyValue{
		semconv.ServiceName("Graphjin"),
	}

	if v := r.Header.Get("User-Agent"); v != "" {
		atrr = append(atrr, semconv.HTTPUserAgent(v))
	}
	if v := r.Method; v != "" {
		atrr = append(atrr, semconv.HTTPMethod(v))
	}
	if v := r.URL.Path; v != "" {
		atrr = append(atrr, semconv.HTTPURL(v))
	}
	if v := r.URL.Scheme; v != "" {
		atrr = append(atrr, semconv.HTTPScheme(v))
	}

	opts := []trace.SpanStartOption{
		trace.WithAttributes(atrr...),
		trace.WithSpanKind(trace.SpanKindServer),
	}

	return ctx, opts
}

// openAPIHandler serves the OpenAPI specification generated by the generator
// registered through OptionSetOpenAPI. Without a generator it responds 404.
func (s1 *HttpService) openAPIHandler(ns *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := s1.Load().(*graphjinService)

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

		if s.openAPIGen == nil {
			http.Error(w, "openapi export not available in slim build", http.StatusNotFound)
			return
		}

		spec, err := s.openAPIGen(s.gj, ns)
		if err != nil {
			s.log.Errorf("failed to generate OpenAPI spec: %s", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(spec); err != nil {
			s.log.Errorf("failed to write OpenAPI spec: %s", err)
		}
	})
}
