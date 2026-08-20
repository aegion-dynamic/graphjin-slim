// Package core provides an API to include and use the GraphJin compiler with your own code.
// For detailed documentation visit https://graphjin.com
package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_log "log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/allow"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/graph"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/psql"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/sdata"
)

const (
	APQ_PX = "_apq"
)

// dbContext holds per-database state for multi-database support.
// Each database gets its own connection pool, schema discovery, and SQL compiler.
type dbContext struct {
	name          string          // Database name (key in Config.Databases)
	db            *sql.DB         // Connection pool for this database
	dbtype        string          // Database type (postgres, mysql, sqlite, etc.)
	dbinfo        *sdata.DBInfo   // Raw schema metadata
	schema        *sdata.DBSchema // Processed schema with relationships
	qcodeCompiler *qcode.Compiler // GraphQL to QCode compiler (validates against this DB's schema)
	psqlCompiler  *psql.Compiler  // QCode to SQL compiler (generates this DB's dialect)
}

// GraphJin struct is an instance of the GraphJin engine it holds all the required information like
// datase schemas, relationships, etc that the GraphQL to SQL compiler would need to do it's job.
type graphjinEngine struct {
	conf                       *Config
	log                        *_log.Logger
	fs                         FS
	trace                      Tracer
	allowList                  *allow.List
	savedQuerySaveHook         SavedQuerySaveHook
	encryptionKey              [32]byte
	encryptionKeySet           bool
	cache                      Cache
	queries                    sync.Map
	sqliteConflictGetMu        sync.Mutex
	roles                      map[string]*Role
	roleStatement              string
	roleStatementMetadata      psql.Metadata
	roleQueryMode              roleQueryMode
	roleGraphQLStmt            stmt
	roleGraphQLMatches         []compiledRoleMatch
	tmap                       map[string]qcode.TConfig
	rtmap                      map[string]ResolverFn
	rmap                       map[string]resItem
	abacEnabled                bool
	subs                       sync.Map
	prod                       bool
	prodSec                    bool
	learn                      bool
	namespace                  string
	printFormat                []byte
	opts                       []Option
	runtimeSchemaDDLDir        string
	runtimeSchemaCacheFirst    bool
	runtimeSchemaCacheRequired bool
	disableDBSchemaWatcher     bool
	done                       chan bool

	// All databases (including the primary/default) live here.
	databases map[string]*dbContext
	// Name of the default database (used as the map key for the primary DB)
	defaultDB string
}

// primaryDB returns the default database context.
func (gj *graphjinEngine) primaryDB() *dbContext {
	if ctx, ok := gj.databases[gj.defaultDB]; ok {
		return ctx
	}
	return nil
}

// anyDatabaseReady returns true if at least one database has an initialized schema.
func (gj *graphjinEngine) anyDatabaseReady() bool {
	for _, ctx := range gj.databases {
		if ctx.schema != nil {
			return true
		}
	}
	return false
}

type GraphJin struct {
	atomic.Value
	lifecycle *Lifecycle
	reloadMu  sync.Mutex // serializes reload operations

	// Schema change callbacks
	schemaCallbacks SchemaCallbacks
}

type Option func(*graphjinEngine) error

// SavedQueryFragment is a fragment captured while saving a named query.
type SavedQueryFragment struct {
	Name  string
	Value []byte
}

// SavedQuerySaveRequest is passed to SavedQuerySaveHook before dev-mode named
// query auto-save writes to the configured filesystem allow-list.
type SavedQuerySaveRequest struct {
	Namespace  string
	Name       string
	Operation  string
	Query      []byte
	Fragments  []SavedQueryFragment
	ActionJSON map[string]json.RawMessage
}

// SavedQuerySaveHook lets an embedding service redirect dev-mode named-query
// saves. Returning handled=false preserves the default filesystem allow-list
// behavior.
type SavedQuerySaveHook func(context.Context, SavedQuerySaveRequest) (handled bool, err error)

// OnSchemaChange registers a callback that fires when the database schema changes.
// The callback receives the database name and a hex-encoded hash of the schema.
// Callbacks also fire once at startup after initial schema discovery.
func (g *GraphJin) OnSchemaChange(fn func(dbName string, hash string)) {
	g.schemaCallbacks.Register(fn)
}

// fireSchemaCallbacks invokes all registered schema change callbacks.
// Runs each callback in a goroutine to avoid blocking the caller (which may hold reloadMu).
func (g *GraphJin) fireSchemaCallbacks(dbName string, hash string) {
	g.schemaCallbacks.Fire(dbName, hash)
}

// DefaultDatabase returns the name of the default (primary) database.
func (g *GraphJin) DefaultDatabase() string {
	gj, err := g.getEngine()
	if err != nil {
		return ""
	}
	return gj.defaultDB
}

// DatabaseNames returns the names of all configured databases.
func (g *GraphJin) DatabaseNames() []string {
	gj, err := g.getEngine()
	if err != nil {
		return nil
	}
	return gj.sortedDatabaseNames()
}

// fireAllSchemaCallbacks fires schema change callbacks for all databases with initialized schemas.
func (g *GraphJin) fireAllSchemaCallbacks() {
	gj, err := g.getEngine()
	if err != nil {
		return
	}
	for name, ctx := range gj.databases {
		if ctx.dbinfo != nil {
			g.fireSchemaCallbacks(name, fmt.Sprintf("%x", ctx.dbinfo.Hash()))
		}
	}
}

// NewGraphJin creates the GraphJin struct, this involves querying the database to learn its
// schemas and relationships
func NewGraphJin(conf *Config, db *sql.DB, options ...Option) (g *GraphJin, err error) {
	fs, err := getFS(conf)
	if err != nil {
		return
	}

	g = &GraphJin{lifecycle: NewLifecycle()}
	if err = g.newGraphJin(conf, db, nil, fs, options...); err != nil {
		g = nil
		return
	}

	if err = g.initDBWatcher(); err != nil {
		g = nil
		return
	}

	g.fireAllSchemaCallbacks()
	return
}

// NewGraphJinWithFS creates the GraphJin struct, this involves querying the database to learn its
func NewGraphJinWithFS(conf *Config, db *sql.DB, fs FS, options ...Option) (g *GraphJin, err error) {
	g = &GraphJin{lifecycle: NewLifecycle()}
	if err = g.newGraphJin(conf, db, nil, fs, options...); err != nil {
		g = nil
		return
	}

	if err = g.initDBWatcher(); err != nil {
		g = nil
		return
	}

	g.fireAllSchemaCallbacks()
	return
}

var errEngineNotInitialized = errors.New("graphjin: engine not initialized")

func (g *GraphJin) getEngine() (*graphjinEngine, error) {
	v := g.Load()
	if v == nil {
		return nil, errEngineNotInitialized
	}
	gj, ok := v.(*graphjinEngine)
	if !ok || gj == nil {
		return nil, errEngineNotInitialized
	}
	return gj, nil
}

// InvalidateCacheRefs invalidates response-cache entries associated with the
// supplied dependency refs. It is a no-op when response caching is disabled.
func (g *GraphJin) InvalidateCacheRefs(ctx context.Context, refs []RowRef) error {
	return nil
}

// Close stops GraphJin background tasks. It is safe to call multiple times.
func (g *GraphJin) Close() {
	if g == nil || g.lifecycle == nil {
		return
	}
	g.lifecycle.Close()
}

// newGraphJinWithDBInfo creates the GraphJin struct, this involves querying the database to learn its
// it all starts here
func (g *GraphJin) newGraphJin(conf *Config,
	db *sql.DB,
	dbinfo *sdata.DBInfo,
	fs FS,
	options ...Option,
) (err error) {
	if conf == nil {
		conf = &Config{Debug: true}
	}

	// Deep-copy mutable slices/maps so that init never mutates the caller's
	// Config. Without this, NormalizeDatabases, finalizeDatabaseSchema, and
	// ensureDiscoveredTablesInConfig would accumulate side-effects across
	// repeated NewGraphJin calls that share the same *Config.
	conf = conf.clone()
	if err := conf.NormalizeMode(); err != nil {
		return err
	}

	t := time.Now()

	gj := &graphjinEngine{
		conf:        conf,
		log:         _log.New(os.Stderr, "", 0),
		prod:        conf.Production,
		prodSec:     conf.Production,
		learn:       !conf.Production,
		printFormat: []byte(fmt.Sprintf("gj-%x:", t.UnixNano())),
		opts:        options,
		fs:          fs,
		trace:       NewTracer(),
		done:        g.lifecycle.Done(),
	}

	if gj.conf.DisableProdSecurity {
		gj.prodSec = false
	}

	// ordering of these initializer matter, do not re-order!

	if err = gj.initCache(); err != nil {
		return
	}

	if err = gj.initConfig(); err != nil {
		return
	}

	// Set defaultDB from the normalized config, preferring application sources
	// over service-owned runtime databases.
	if gj.defaultDB == "" {
		gj.defaultDB = gj.conf.defaultDatabaseName()
	}

	// Determine dbtype for the primary database
	dbtype := conf.DBType
	if dbtype == "" {
		dbtype = "postgres"
	}

	// Store the primary DB as a bare context in gj.databases.
	// Always create the entry even when db is nil (e.g. MockDB mode).
	gj.databases = make(map[string]*dbContext)
	gj.databases[gj.defaultDB] = &dbContext{
		name:   gj.defaultDB,
		db:     db, // may be nil for MockDB
		dbtype: dbtype,
		dbinfo: dbinfo, // may be preset from watcher/tests
	}

	for _, op := range options {
		if err = op(gj); err != nil {
			return
		}
	}
	// Catalog configuration revisions describe the normalized caller config,
	// not derived table entries added during schema finalization. Keeping this
	// snapshot stable also makes live- and cache-discovered engines comparable.

	// Phase 1: Discover all databases (get raw schema metadata)
	if err = gj.discoverAllDatabases(); err != nil {
		return
	}

	// Phase 2: Resolvers (adds remote tables to primary DB's dbinfo)
	if err = gj.initResolvers(); err != nil {
		return
	}

	// Phase 3: Managed service-owned tables (adds synthetic gj_* roots)

	// Phase 4: Finalize schemas and compilers for all databases
	if err = gj.finalizeAllDatabases(); err != nil {
		return
	}

	// Only initialize dependent features if at least one database has a schema
	if gj.anyDatabaseReady() {
		if err = gj.initAllowList(); err != nil {
			return
		}

		if err = gj.prepareRoleStmt(); err != nil {
			return
		}

		if err = gj.initIntro(); err != nil {
			return
		}
	}

	if conf.SecretKey != "" {
		sk := sha256.Sum256([]byte(conf.SecretKey))
		gj.encryptionKey = sk
		gj.encryptionKeySet = true
	}

	g.Store(gj)
	return
}

func OptionSetNamespace(namespace string) Option {
	return func(s *graphjinEngine) error {
		s.namespace = namespace
		return nil
	}
}

// OptionSetFS sets the file system to be used by GraphJin
func OptionSetFS(fs FS) Option {
	return func(s *graphjinEngine) error {
		s.fs = fs
		return nil
	}
}

// OptionSetSavedQuerySaveHook installs a service hook for dev-mode named-query
// auto-save. If the hook returns handled=false, GraphJin writes to the
// configured filesystem allow-list as before.
func OptionSetSavedQuerySaveHook(h SavedQuerySaveHook) Option {
	return func(s *graphjinEngine) error {
		s.savedQuerySaveHook = h
		return nil
	}
}

// OptionSetRuntimeSchemaDDLDir overrides where generated schema restart
// snapshots are read from and written to. Relative and absolute paths are
// passed through to the configured filesystem implementation unchanged.
func OptionSetRuntimeSchemaDDLDir(dir string) Option {
	return func(s *graphjinEngine) error {
		s.runtimeSchemaDDLDir = strings.TrimSpace(dir)
		return nil
	}
}

// OptionSetRuntimeSchemaCacheFirst makes runtime discovery snapshots the first
// source consulted during initialization. Direct core users retain live-first
// discovery unless they opt in.
func OptionSetRuntimeSchemaCacheFirst(enabled bool) Option {
	return func(s *graphjinEngine) error {
		s.runtimeSchemaCacheFirst = enabled
		return nil
	}
}

// OptionSetRuntimeSchemaCacheRequired prevents a cache-initialized engine from
// falling through to live discovery when an activated generation is incomplete.
func OptionSetRuntimeSchemaCacheRequired(required bool) Option {
	return func(s *graphjinEngine) error {
		s.runtimeSchemaCacheRequired = required
		return nil
	}
}

// OptionSetDBSchemaWatcherDisabled lets a service-level coordinator own schema
// polling so horizontally scaled replicas do not all introspect independently.
func OptionSetDBSchemaWatcherDisabled(disabled bool) Option {
	return func(s *graphjinEngine) error {
		s.disableDBSchemaWatcher = disabled
		return nil
	}
}

// OptionSetTrace sets the tracer to be used by GraphJin
func OptionSetTrace(trace Tracer) Option {
	return func(s *graphjinEngine) error {
		s.trace = trace
		return nil
	}
}

// OptionSetResolver sets the resolver function to be used by GraphJin
func OptionSetResolver(name string, fn ResolverFn) Option {
	return func(s *graphjinEngine) error { return errResolversDisabled }
}

type Error struct {
	Message    string         `json:"message"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// Result struct contains the output of the GraphQL function this includes resulting json from the
// database query and any error information
type Result struct {
	namespace    string
	operation    qcode.QType
	name         string
	sql          string
	role         string
	cacheControl string
	cacheHit     bool
	subCursors   map[string]string
	rootLimits   []RootLimitInfo
	Vars         json.RawMessage   `json:"-"`
	Data         json.RawMessage   `json:"data,omitempty"`
	Hash         [sha256.Size]byte `json:"-"`
	Errors       []Error           `json:"errors,omitempty"`
	Validation   []qcode.ValidErr  `json:"validation,omitempty"`
	// Extensions   *extensions     `json:"extensions,omitempty"`
}

// RequestConfig is used to pass request specific config values to the GraphQL and Subscribe functions. Dynamic variables can be set here.
type RequestConfig struct {
	ns *string

	// APQKey is set when using GraphJin with automatic persisted queries
	APQKey string

	// Pass additional variables complex variables such as functions that return string values.
	Vars map[string]interface{}

	// Execute this query as part of a transaction
	Tx *sql.Tx
}

// SubscriptionRootInfo describes a root selected by a subscription member.
// Filter contains the resolved user filter and excludes GraphJin's synthetic
// cursor seek predicate.
type SubscriptionRootInfo struct {
	FieldName string
	Table     string
	Database  string
	CursorVar string
	Filter    map[string]any
}

// SetNamespace is used to set namespace requests within a single instance of GraphJin. For example queries with the same name
func (rc *RequestConfig) SetNamespace(ns string) {
	rc.ns = &ns
}

// GetNamespace is used to get the namespace requests within a single instance of GraphJin
func (rc *RequestConfig) GetNamespace() (string, bool) {
	if rc.ns != nil {
		return *rc.ns, true
	}
	return "", false
}

// GraphQL function is our main function it takes a GraphQL query compiles it
// to SQL and executes returning the resulting JSON.
//
// In production mode the compiling happens only once and from there on the compiled queries
// are directly executed.
//
// In developer mode all named queries are saved into the queries folder and in production mode only
// queries from these saved queries can be used.
func (g *GraphJin) GraphQL(c context.Context,
	query string,
	vars json.RawMessage,
	rc *RequestConfig,
) (res *Result, err error) {
	gj, err := g.getEngine()
	if err != nil {
		return
	}

	c1, span := gj.spanStart(c, "GraphJin Query")
	defer span.End()

	var queryBytes []byte
	var inCache bool

	// get query from apq cache if apq key exists
	if rc != nil && rc.APQKey != "" {
		queryBytes, inCache = gj.cache.Get(APQ_PX + rc.APQKey)
	}

	// query not found in apq cache so use original query
	if len(queryBytes) == 0 {
		queryBytes = []byte(query)
	}

	// fast extract name and query type from query
	h, err := graph.FastParseBytes(queryBytes)
	if err != nil {
		return
	}
	r := gj.newGraphqlReq(rc, h.Operation, h.Name, queryBytes, vars)

	// if production security enabled then get query and metadata
	// from allow list
	if gj.prodSec {
		var item allow.Item
		item, err = gj.allowList.GetByName(h.Name, true)
		if err != nil {
			err = fmt.Errorf("%w: %s", err, h.Name)
			return
		}
		r.Set(item)
	}

	// do the query
	resp, err := gj.query(c1, r)
	res = &resp.res
	if err != nil {
		return
	}

	// save to apq cache is apq key exists and not already in cache
	if !inCache && rc != nil && rc.APQKey != "" {
		gj.cache.Set((APQ_PX + rc.APQKey), r.query)
	}

	// Development learning saves named queries to the allow list. Agentic mode
	// keeps dynamic authoring enabled but does not mint permanent query entries.
	if gj.learn && r.name != "" && r.name != "IntrospectionQuery" {
		if err = gj.saveToAllowList(c, resp.qc, resp.res.namespace); err != nil {
			return
		}
	}
	return
}

// GraphQLTx is similiar to the GraphQL function except that it can be used
// within a database transactions.
func (g *GraphJin) GraphQLTx(c context.Context,
	tx *sql.Tx,
	query string,
	vars json.RawMessage,
	rc *RequestConfig,
) (res *Result, err error) {
	if rc == nil {
		rc = &RequestConfig{Tx: tx}
	} else {
		rc.Tx = tx
	}
	return g.GraphQL(c, query, vars, rc)
}

// GraphQLByName is similar to the GraphQL function except that queries saved
// in the queries folder can directly be used just by their name (filename).
func (g *GraphJin) GraphQLByName(c context.Context,
	name string,
	vars json.RawMessage,
	rc *RequestConfig,
) (res *Result, err error) {
	gj, err := g.getEngine()
	if err != nil {
		return
	}

	c1, span := gj.spanStart(c, "GraphJin Query")
	defer span.End()

	item, err := gj.allowList.GetByName(name, gj.prod)
	if err != nil {
		err = fmt.Errorf("%w: %s", err, name)
		return
	}

	r := gj.newGraphqlReq(rc, "", name, nil, vars)
	r.Set(item)

	res, err = gj.queryWithResult(c1, r)
	return
}

// GraphQLBySavedQuery executes a saved query definition supplied by the
// embedding service. It is intended for trusted stores that have already
// resolved the saved query and its imports.
func (g *GraphJin) GraphQLBySavedQuery(c context.Context,
	details *SavedQueryDetails,
	vars json.RawMessage,
	rc *RequestConfig,
) (res *Result, err error) {
	if details == nil {
		return nil, errors.New("saved query details are required")
	}
	if strings.TrimSpace(details.Name) == "" {
		return nil, errors.New("saved query name is required")
	}
	if strings.TrimSpace(details.Query) == "" {
		return nil, errors.New("saved query query is required")
	}
	gj, err := g.getEngine()
	if err != nil {
		return
	}

	c1, span := gj.spanStart(c, "GraphJin Query")
	defer span.End()

	operation := strings.TrimSpace(details.Operation)
	if operation == "" {
		h, parseErr := graph.FastParseBytes([]byte(details.Query))
		if parseErr != nil {
			return nil, parseErr
		}
		operation = h.Operation
	}

	item := allow.Item{
		Namespace:  details.Namespace,
		Name:       details.Name,
		Operation:  operation,
		Query:      []byte(details.Query),
		ActionJSON: savedQueryVariablesToRaw(details.Variables),
	}
	r := gj.newGraphqlReq(rc, "", details.Name, nil, vars)
	r.Set(item)

	res, err = gj.queryWithResult(c1, r)
	return
}

// GraphQLByNameTx is similiar to the GraphQLByName function except
// that it can be used within a database transactions.
func (g *GraphJin) GraphQLByNameTx(c context.Context,
	tx *sql.Tx,
	name string,
	vars json.RawMessage,
	rc *RequestConfig,
) (res *Result, err error) {
	if rc == nil {
		rc = &RequestConfig{Tx: tx}
	} else {
		rc.Tx = tx
	}
	return g.GraphQLByName(c, name, vars, rc)
}

type GraphqlReq struct {
	namespace     string
	operation     qcode.QType
	name          string
	query         []byte
	vars          json.RawMessage
	aschema       map[string]json.RawMessage
	requestconfig *RequestConfig
}

type GraphqlResponse struct {
	res Result
	qc  *qcode.QCode
}

// newGraphqlReq creates a new GraphQL request
func (gj *graphjinEngine) newGraphqlReq(rc *RequestConfig,
	op string,
	name string,
	query []byte,
	vars json.RawMessage,
) (r GraphqlReq) {
	r = GraphqlReq{
		operation: qcode.GetQTypeByName(op),
		name:      name,
		query:     query,
		vars:      vars,
	}

	if rc != nil {
		r.requestconfig = rc
	}
	if rc != nil && rc.ns != nil {
		r.namespace = *rc.ns
	} else {
		r.namespace = gj.namespace
	}
	return
}

// Set is used to set the namespace, operation type, name and query for the GraphQL request
func (r *GraphqlReq) Set(item allow.Item) {
	r.namespace = item.Namespace
	r.operation = qcode.GetQTypeByName(item.Operation)
	r.name = item.Name
	r.query = item.Query
	r.aschema = item.ActionJSON
}

// GraphQL function is our main function it takes a GraphQL query compiles it
func (gj *graphjinEngine) queryWithResult(c context.Context, r GraphqlReq) (res *Result, err error) {
	resp, err := gj.query(c, r)
	return &resp.res, err
}

// GraphQL function is our main function it takes a GraphQL query compiles it
func (gj *graphjinEngine) query(c context.Context, r GraphqlReq) (
	resp GraphqlResponse, err error,
) {
	resp.res = Result{
		namespace: r.namespace,
		operation: r.operation,
		name:      r.name,
	}

	if !gj.prodSec && r.name == "IntrospectionQuery" {
		resp.res.Data, err = gj.getIntroResult()
		return
	}

	if r.operation == qcode.QTSubscription {
		err = errors.New("subscriptions are not supported")
		return
	}

	if !gj.anyDatabaseReady() {
		err = fmt.Errorf("no tables found in any database; schema not initialized")
		return
	}

	s, err := newGState(c, gj, r)
	if err != nil {
		return
	}
	err = s.compileAndExecuteWrapper(c)

	resp.qc = s.qcode()
	resp.res.rootLimits = rootLimitInfoFromQCode(resp.qc)
	resp.res.sql = s.sql()
	resp.res.cacheControl = s.cacheHeader()
	resp.res.Vars = r.vars
	// Strip internal __gj_id fields unconditionally when cache tracking is enabled.
	// This handles all code paths: cache hits, multi-DB queries, and regular queries.
	if false {
		s.data = stripGjIdFields(s.data)
	}
	resp.res.Data = json.RawMessage(s.data)
	resp.res.Hash = s.dhash
	resp.res.role = s.role
	resp.res.cacheHit = s.cacheHit || (s.fragmentHits.Load() > 0 && s.fragmentMisses.Load() == 0)

	if err != nil {
		resp.res.Errors = newError(string(r.query), err)
	}

	if len(s.verrs) != 0 {
		resp.res.Validation = s.verrs
	}
	return
}

// Reload redoes database discover and reinitializes GraphJin.
func (g *GraphJin) Reload() error {
	g.reloadMu.Lock()
	defer g.reloadMu.Unlock()
	gj, err := g.getEngine()
	if err != nil {
		return err
	}
	var db *sql.DB
	if pdb := gj.primaryDB(); pdb != nil {
		db = pdb.db
	}
	if err := g.newGraphJin(gj.conf, db, nil, gj.fs, gj.opts...); err != nil {
		return err
	}
	g.fireAllSchemaCallbacks()
	return nil
}

// ReloadDatabase rediscovers and rebuilds one configured database context.
// It is intended for source-local schema changes. Call Reload for global
// config, auth, relationship, resolver, API, filesystem, or workflow changes.
func (g *GraphJin) ReloadDatabase(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("database name is required")
	}
	g.reloadMu.Lock()
	defer g.reloadMu.Unlock()

	gj, err := g.getEngine()
	if err != nil {
		return err
	}
	if _, ok := gj.databases[name]; !ok {
		return fmt.Errorf("database %q not configured", name)
	}
	if err := g.newGraphJinReloadingDatabase(gj, name); err != nil {
		return err
	}
	next, err := g.getEngine()
	if err != nil {
		return err
	}
	if ctx, ok := next.databases[name]; ok && ctx.dbinfo != nil {
		g.fireSchemaCallbacks(name, fmt.Sprintf("%x", ctx.dbinfo.Hash()))
	}
	return nil
}

// ReloadConfigDatabases swaps to conf while rediscoving and rebuilding only the
// named database contexts. It is intended for service-owned config commits that
// have already proved the change is database-source scoped. Use Reload for
// global config, auth, relationship, resolver, API, filesystem, or workflow
// changes.
func (g *GraphJin) ReloadConfigDatabases(conf *Config, databases []string, opts ...Option) error {
	if conf == nil {
		return fmt.Errorf("config is required")
	}
	reloadSet := make(map[string]struct{}, len(databases))
	for _, name := range databases {
		name = strings.TrimSpace(name)
		if name != "" {
			reloadSet[name] = struct{}{}
		}
	}
	if len(reloadSet) == 0 {
		return fmt.Errorf("at least one database name is required")
	}

	g.reloadMu.Lock()
	defer g.reloadMu.Unlock()

	base, err := g.getEngine()
	if err != nil {
		return err
	}
	if len(opts) == 0 {
		opts = base.opts
	}
	if err := g.newGraphJinReloadingConfigDatabases(base, conf, reloadSet, opts...); err != nil {
		return err
	}
	next, err := g.getEngine()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(reloadSet))
	for name := range reloadSet {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if ctx, ok := next.databases[name]; ok && ctx.dbinfo != nil {
			g.fireSchemaCallbacks(name, fmt.Sprintf("%x", ctx.dbinfo.Hash()))
		}
	}
	return nil
}

// ReloadWithDB redoes database discover with a new primary DB connection.
func (g *GraphJin) ReloadWithDB(db *sql.DB) error {
	g.reloadMu.Lock()
	defer g.reloadMu.Unlock()
	gj, err := g.getEngine()
	if err != nil {
		return err
	}
	return g.newGraphJin(gj.conf, db, nil, gj.fs, gj.opts...)
}

// ReloadFromRuntimeSchemaCache atomically rebuilds the engine from a complete,
// activated runtime schema generation. It never falls through to live database
// discovery, which keeps horizontally scaled replicas on the same generation.
func (g *GraphJin) ReloadFromRuntimeSchemaCache(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("runtime schema cache directory is required")
	}
	g.reloadMu.Lock()
	defer g.reloadMu.Unlock()
	base, err := g.getEngine()
	if err != nil {
		return err
	}
	var db *sql.DB
	if primary := base.primaryDB(); primary != nil {
		db = primary.db
	}
	reloadConf := base.conf
	opts := append([]Option(nil), base.opts...)
	opts = append(opts,
		OptionSetRuntimeSchemaDDLDir(dir),
		OptionSetRuntimeSchemaCacheFirst(true),
		OptionSetRuntimeSchemaCacheRequired(true),
	)
	if err := g.newGraphJin(reloadConf, db, nil, base.fs, opts...); err != nil {
		return err
	}
	g.fireAllSchemaCallbacks()
	return nil
}

func (g *GraphJin) newGraphJinReloadingConfigDatabases(base *graphjinEngine, nextConf *Config, reloadSet map[string]struct{}, opts ...Option) error {
	if base == nil {
		return errors.New("graphjin engine is not initialized")
	}
	conf := nextConf.clone()
	if conf.IsSourcesUsed() {
		for i := range conf.Tables {
			if conf.Tables[i].Source == "" && conf.Tables[i].Database != "" {
				conf.Tables[i].Source = conf.Tables[i].Database
			}
		}
	}
	if err := conf.NormalizeMode(); err != nil {
		return err
	}

	log := base.log
	if log == nil {
		log = _log.New(os.Stderr, "", 0)
	}
	trace := base.trace
	if trace == nil {
		trace = NewTracer()
	}
	printFormat := append([]byte(nil), base.printFormat...)
	if len(printFormat) == 0 {
		printFormat = []byte(fmt.Sprintf("gj-%x:", time.Now().UnixNano()))
	}

	gj := &graphjinEngine{
		conf:        conf,
		log:         log,
		prod:        conf.Production,
		prodSec:     conf.Production,
		learn:       !conf.Production,
		printFormat: printFormat,
		opts:        opts,
		fs:          base.fs,
		trace:       trace,
		namespace:   base.namespace,
		done:        g.lifecycle.Done(),
	}
	if gj.conf.DisableProdSecurity {
		gj.prodSec = false
	}
	if err := gj.initCache(); err != nil {
		return err
	}
	if err := gj.initConfig(); err != nil {
		return err
	}
	gj.defaultDB = gj.conf.defaultDatabaseName()

	for _, op := range opts {
		if err := op(gj); err != nil {
			return err
		}
	}
	if gj.databases == nil {
		gj.databases = make(map[string]*dbContext)
	}

	for name, ctx := range gj.databases {
		if _, reload := reloadSet[name]; reload {
			continue
		}
		baseCtx := base.databases[name]
		if baseCtx == nil {
			reloadSet[name] = struct{}{}
			continue
		}
		if !reflect.DeepEqual(base.conf.Databases[name], conf.Databases[name]) || baseCtx.db != ctx.db {
			reloadSet[name] = struct{}{}
			continue
		}
		gj.databases[name] = cloneDBContextForReload(baseCtx)
	}

	reloadDefault := base.defaultDB != gj.defaultDB
	for name := range reloadSet {
		if name == gj.defaultDB {
			reloadDefault = true
			break
		}
	}
	if reloadDefault {
		if err := gj.initResolvers(); err != nil {
			return err
		}
	} else {
		gj.rtmap = base.rtmap
		gj.rmap = base.rmap
	}

	reloadNames := make([]string, 0, len(reloadSet))
	for name := range reloadSet {
		reloadNames = append(reloadNames, name)
	}
	sort.Strings(reloadNames)
	for _, name := range reloadNames {
		target := gj.databases[name]
		if target == nil {
			continue
		}
		target.dbinfo = nil
		target.schema = nil
		target.qcodeCompiler = nil
		target.psqlCompiler = nil
		if err := gj.discoverDatabase(target); err != nil {
			return err
		}
		if err := gj.finalizeDatabaseSchema(target); err != nil {
			return err
		}
		if !schemaHasApplicationTables(target.schema) {
			return fmt.Errorf("database connected but schema discovery found no tables")
		}
	}
	if gj.anyDatabaseReady() {
		if err := gj.initAllowList(); err != nil {
			return err
		}
		if err := gj.prepareRoleStmt(); err != nil {
			return err
		}
		if err := gj.initIntro(); err != nil {
			return err
		}
	}
	if conf.SecretKey != "" {
		sk := sha256.Sum256([]byte(conf.SecretKey))
		gj.encryptionKey = sk
		gj.encryptionKeySet = true
	}
	g.Store(gj)
	return nil
}

func (g *GraphJin) newGraphJinReloadingDatabase(base *graphjinEngine, database string) error {
	if base == nil {
		return errors.New("graphjin engine is not initialized")
	}
	conf := base.conf.clone()
	if conf.IsSourcesUsed() {
		for i := range conf.Tables {
			if conf.Tables[i].Source == "" && conf.Tables[i].Database != "" {
				conf.Tables[i].Source = conf.Tables[i].Database
			}
		}
	}
	if err := conf.NormalizeMode(); err != nil {
		return err
	}

	log := base.log
	if log == nil {
		log = _log.New(os.Stderr, "", 0)
	}
	trace := base.trace
	if trace == nil {
		trace = NewTracer()
	}
	printFormat := append([]byte(nil), base.printFormat...)
	if len(printFormat) == 0 {
		printFormat = []byte(fmt.Sprintf("gj-%x:", time.Now().UnixNano()))
	}

	gj := &graphjinEngine{
		conf:        conf,
		log:         log,
		prod:        conf.Production,
		prodSec:     conf.Production,
		learn:       !conf.Production,
		printFormat: printFormat,
		opts:        base.opts,
		fs:          base.fs,
		trace:       trace,
		namespace:   base.namespace,
		done:        g.lifecycle.Done(),
	}
	if gj.conf.DisableProdSecurity {
		gj.prodSec = false
	}
	if err := gj.initCache(); err != nil {
		return err
	}
	if err := gj.initConfig(); err != nil {
		return err
	}
	gj.defaultDB = base.defaultDB
	if gj.defaultDB == "" {
		gj.defaultDB = gj.conf.defaultDatabaseName()
	}

	for _, op := range base.opts {
		if err := op(gj); err != nil {
			return err
		}
	}

	gj.databases = make(map[string]*dbContext, len(base.databases))
	for name, ctx := range base.databases {
		gj.databases[name] = cloneDBContextForReload(ctx)
	}
	target := gj.databases[database]
	if target == nil {
		return fmt.Errorf("database %q not configured", database)
	}
	target.dbinfo = nil
	target.schema = nil
	target.qcodeCompiler = nil
	target.psqlCompiler = nil

	if err := gj.discoverDatabase(target); err != nil {
		return err
	}

	if database == gj.defaultDB {
		if err := gj.initResolvers(); err != nil {
			return err
		}
	} else {
		gj.rtmap = base.rtmap
		gj.rmap = base.rmap
	}
	if err := gj.finalizeDatabaseSchema(target); err != nil {
		return err
	}
	if gj.anyDatabaseReady() {
		if err := gj.initAllowList(); err != nil {
			return err
		}
		if err := gj.prepareRoleStmt(); err != nil {
			return err
		}
		if err := gj.initIntro(); err != nil {
			return err
		}
	}
	if conf.SecretKey != "" {
		sk := sha256.Sum256([]byte(conf.SecretKey))
		gj.encryptionKey = sk
		gj.encryptionKeySet = true
	}
	g.Store(gj)
	return nil
}

func cloneDBContextForReload(ctx *dbContext) *dbContext {
	if ctx == nil {
		return nil
	}
	return &dbContext{
		name:          ctx.name,
		db:            ctx.db,
		dbtype:        ctx.dbtype,
		dbinfo:        ctx.dbinfo,
		schema:        ctx.schema,
		qcodeCompiler: ctx.qcodeCompiler,
		psqlCompiler:  ctx.psqlCompiler,
	}
}

// SetOptions replaces the options slice so the next Reload picks them up.
func (g *GraphJin) SetOptions(opts ...Option) {
	g.reloadMu.Lock()
	defer g.reloadMu.Unlock()
	gj, err := g.getEngine()
	if err != nil {
		return
	}
	gj.opts = opts
}

// IsProd return true for production mode or false for development mode
func (g *GraphJin) IsProd() bool {
	gj, err := g.getEngine()
	if err != nil {
		return false
	}
	return gj.prod
}

type Header struct {
	Type OpType
	Name string
}

// Operation function return the operation type and name from the query.
// It uses a very fast algorithm to extract the operation without having to parse the query.
func Operation(query string) (h Header, err error) {
	if v, err := graph.FastParse(query); err == nil {
		h.Type = OpType(qcode.GetQTypeByName(v.Operation))
		h.Name = v.Name
	}
	return
}

// getFS returns the file system to be used by GraphJin
func getFS(conf *Config) (fs FS, err error) {
	if v, ok := conf.FS.(FS); ok {
		fs = v
		return
	}

	v, err := os.Getwd()
	if err != nil {
		return
	}

	fs = NewOsFS(filepath.Join(v, "config"))
	return
}

// newError creates a new error list
func newError(query string, err error) (errList []Error) {
	e := Error{Message: err.Error()}
	if repair := BuildGraphJinErrorRepair(query, err.Error()); repair.Known() {
		e.Extensions = map[string]any{"graphjin_repair": repair}
	}
	errList = []Error{e}
	return
}

// stripGjIdFields removes all "__gj_id" fields from JSON response.
// Uses JSON parse/delete/marshal for correctness - doesn't depend on QCode.
// This is used to unconditionally strip internal tracking fields from all responses,
// including cache hits where s.cs is nil.
func stripGjIdFields(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	result, err := stripCacheTrackingFields(data)
	if err != nil {
		return data // Return original on strip error
	}
	return result
}

// TableInfo represents basic table information for MCP/API consumers
func (g *GraphJin) SchemaReady() bool {
	gj, ok := g.Load().(*graphjinEngine)
	if !ok || gj == nil {
		return false
	}
	for _, ctx := range gj.databases {
		if schemaHasApplicationTables(ctx.schema) {
			return true
		}
	}
	return false
}

func schemaHasApplicationTables(schema *sdata.DBSchema) bool {
	if schema == nil {
		return false
	}
	for _, table := range schema.GetTables() {
		name := strings.ToLower(table.Name)
		if table.Blocked || table.Type == "managed" {
			continue
		}
		switch name {
		case "gj_catalog", "gj_security", "gj_workflow", "gj_config", "gj_code":
			return true
		}
		if strings.HasPrefix(name, "gj_") {
			continue
		}
		return true
	}
	return false
}

func (g *GraphJin) EffectiveAnalyticsMode(database string) bool {
	gj, err := g.getEngine()
	if err != nil {
		return false
	}
	return gj.conf.EffectiveAnalyticsMode(database)
}

// DBForDatabase returns the read-only connection pool and database type for
// `database`. Empty name resolves to the default database. Do not close the pool.
func (g *GraphJin) DBForDatabase(database string) (*sql.DB, string, error) {
	gj, err := g.getEngine()
	if err != nil {
		return nil, "", err
	}
	ctx, ok := gj.GetDatabase(database)
	if !ok || ctx == nil {
		if database == "" {
			return nil, "", fmt.Errorf("no default database configured")
		}
		return nil, "", fmt.Errorf("database %q not configured", database)
	}
	if ctx.db == nil {
		return nil, ctx.dbtype, fmt.Errorf("database %q has no active connection", database)
	}
	return ctx.db, ctx.dbtype, nil
}

// getTables returns tables, optionally filtered by database name.
// With empty database, returns tables from all databases.

type SavedQueryInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Operation string `json:"operation"` // query or mutation
}

type SavedQueryDetails struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace,omitempty"`
	Operation string                 `json:"operation"`
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

func savedQueryVariablesToRaw(vars map[string]interface{}) map[string]json.RawMessage {
	if len(vars) == 0 {
		return nil
	}
	out := make(map[string]json.RawMessage, len(vars))
	for k, v := range vars {
		b, err := json.Marshal(v)
		if err != nil {
			continue
		}
		out[k] = json.RawMessage(b)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
