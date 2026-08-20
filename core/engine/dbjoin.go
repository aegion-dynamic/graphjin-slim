package engine

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/dbjoin"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/jsn"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/qcode"
)

type dbResult = dbjoin.DBResult

var (
	buildChildGraphQLQuery = dbjoin.BuildChildGraphQLQuery
	writeGraphQLLiteral    = dbjoin.WriteGraphQLLiteral
	writeSelectFields      = dbjoin.WriteSelectFields
)

func (gj *graphjinEngine) isMultiDB() bool {
	return len(gj.databases) > 1
}

func (s *gstate) databaseJoinFieldIds() ([][]byte, map[string]*qcode.Select, error) {
	if s.cs == nil || s.cs.st.qc == nil {
		return nil, nil, nil
	}
	return dbjoin.DatabaseJoinFieldIDs(s.cs.st.qc.Selects)
}

func countDatabaseJoins(qc *qcode.QCode) int32 {
	return dbjoin.CountDatabaseJoins(qc)
}

func (s *gstate) mergeRootResults(results []dbResult) error {
	data, err := dbjoin.MergeRootResults(results)
	if err != nil {
		return err
	}
	s.data = data
	return nil
}

type dbGroup struct {
	database string
	selects  []int32
}

func (s *gstate) groupSelectsByDatabase() []dbGroup {
	if s.cs == nil || s.cs.st.qc == nil {
		return nil
	}
	qc := s.cs.st.qc
	byDB := make(map[string][]int32)
	for _, rootID := range qc.Roots {
		sel := qc.Selects[rootID]
		db := sel.Database
		if db == "" {
			db = sel.Ti.Database
		}
		if db == "" {
			db = s.gj.defaultDB
		}
		byDB[db] = append(byDB[db], rootID)
	}
	groups := make([]dbGroup, 0, len(byDB))
	for db, sels := range byDB {
		groups = append(groups, dbGroup{database: db, selects: sels})
	}
	return groups
}

func (s *gstate) execDatabaseJoins(c context.Context) (err error) {
	fids, sfmap, err := s.databaseJoinFieldIds()
	if err != nil || len(fids) == 0 {
		return err
	}

	from := jsn.Get(s.data, fids)
	if len(from) == 0 {
		return nil
	}

	to, err := s.resolveDatabaseJoins(c, from, sfmap)
	if err != nil {
		return err
	}

	var ob bytes.Buffer
	if err = jsn.Replace(&ob, s.data, from, to); err != nil {
		return err
	}
	s.data = ob.Bytes()
	return nil
}

func (s *gstate) resolveDatabaseJoins(
	ctx context.Context,
	from []jsn.Field,
	sfmap map[string]*qcode.Select,
) ([]jsn.Field, error) {
	selects := s.cs.st.qc.Selects
	to := make([]jsn.Field, len(from))

	var wg sync.WaitGroup
	wg.Add(len(from))

	var cerr error
	var cerrMutex sync.Mutex

	for i, id := range from {
		sel, ok := sfmap[string(id.Key)]
		if !ok {
			return nil, fmt.Errorf("invalid database join field key")
		}
		p := selects[sel.ParentID]

		targetDB := sel.Database
		if targetDB == "" {
			targetDB = sel.Ti.Database
		}

		dbCtx, ok := s.gj.databases[targetDB]
		if !ok {
			return nil, fmt.Errorf("database not found: %s", targetDB)
		}

		rawIDVal := bytes.TrimSpace(id.Value)
		idVal := jsn.Value(rawIDVal)

		go func(n int, idVal, rawIDVal []byte, sel *qcode.Select, dbCtx *dbContext, parentTable string) {
			defer wg.Done()

			if len(idVal) == 0 || bytes.Equal(rawIDVal, []byte("null")) {
				to[n] = jsn.Field{Key: []byte(sel.FieldName), Value: []byte("null")}
				return
			}

			ctx1, span := s.gj.spanStart(ctx, "Execute Database Join")
			if span.IsRecording() {
				span.SetAttributesString(
					StringAttr{"join.database", dbCtx.name},
					StringAttr{"join.table", sel.Table},
					StringAttr{"join.parent_table", parentTable},
				)
			}

			b, err := s.executeDatabaseJoinQuery(ctx1, dbCtx, sel, idVal)
			if err != nil {
				cerrMutex.Lock()
				cerr = fmt.Errorf("database join %s.%s: %w", dbCtx.name, sel.Table, err)
				spanErr := cerr
				cerrMutex.Unlock()
				span.Error(spanErr)
			}
			span.End()

			if err != nil {
				return
			}

			b = jsn.Strip(b, [][]byte{[]byte(sel.Table)})
			to[n] = jsn.Field{Key: []byte(sel.FieldName), Value: b}
		}(i, idVal, rawIDVal, sel, dbCtx, p.Table)
	}

	wg.Wait()
	return to, cerr
}

func (s *gstate) executeDatabaseJoinQuery(
	ctx context.Context,
	dbCtx *dbContext,
	sel *qcode.Select,
	parentID []byte,
) ([]byte, error) {
	selects := s.cs.st.qc.Selects
	fkCol := sel.Rel.Left.Col
	subQuery := dbjoin.BuildChildGraphQLQuery(sel, selects, fkCol, parentID)

	qc, err := dbCtx.qcodeCompiler.Compile(subQuery, nil, s.r.namespace)
	if err != nil {
		return nil, fmt.Errorf("qcode compile failed: %w", err)
	}

	var sqlBuf bytes.Buffer
	md, err := dbCtx.psqlCompiler.Compile(&sqlBuf, qc)
	if err != nil {
		return nil, fmt.Errorf("sql compile failed: %w", err)
	}

	conn, err := dbCtx.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer conn.Close()

	args, err := s.gj.argList(ctx, md, nil, s.r.requestconfig, false, dbCtx.psqlCompiler)
	if err != nil {
		return nil, fmt.Errorf("failed to build args: %w", err)
	}

	querySQL, queryArgs, err := prepareQueryArgsForDB(dbCtx.dbtype, sqlBuf.String(), args.values)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query args: %w", err)
	}

	cacheKey := s.dbFragmentKey(ctx, fragmentKindDBJoin, dbCtx.name, querySQL, queryArgs, qc)
	produce := func(c context.Context, useConn *sql.Conn) ([]byte, error) {
		raw, err := scanJSONRow(c, dbCtx.dbtype, useConn, nil, querySQL, queryArgs)
		if err == sql.ErrNoRows {
			if sel.Singular {
				raw = []byte(`{"` + sel.Table + `": null}`)
			} else {
				raw = []byte(`{"` + sel.Table + `": []}`)
			}
		} else if err != nil {
			return nil, fmt.Errorf("query execution failed: %w", err)
		}
		return raw, nil
	}

	if cached, ok := s.fragmentCacheGet(ctx, cacheKey, func() ([]byte, []RowRef, CacheEntryOptions, error) {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), swrRefreshTimeout)
		defer cancel()
		refreshConn, err := dbCtx.db.Conn(c)
		if err != nil {
			return nil, nil, CacheEntryOptions{}, err
		}
		defer refreshConn.Close()
		data, err := produce(c, refreshConn)
		if err != nil {
			return nil, nil, CacheEntryOptions{}, err
		}
		cachedData, refs, err := s.processDBFragmentForCache(dbCtx.name, qc, data)
		return cachedData, refs, CacheEntryOptions{}, err
	}); ok {
		return cached, nil
	}

	start := time.Now()
	data, err := produce(ctx, conn)
	if err != nil {
		return nil, err
	}
	if cachedData, refs, perr := s.processDBFragmentForCache(dbCtx.name, qc, data); perr == nil {
		s.fragmentCacheSet(ctx, cacheKey, cachedData, refs, start, CacheEntryOptions{})
	}

	return data, nil
}

func (s *gstate) executeParallelRoots(c context.Context) error {
	if !s.multiDB || len(s.dbGroups) == 0 {
		return fmt.Errorf("executeParallelRoots called without multi-DB configuration")
	}

	var wg sync.WaitGroup
	results := make([]dbjoin.DBResult, len(s.dbGroups))

	i := 0
	for dbName, rootFields := range s.dbGroups {
		wg.Add(1)
		go func(idx int, db string, fields []string) {
			defer wg.Done()
			rootState := s.cloneForDatabaseRoot(db)

			ctx1, span := s.gj.spanStart(c, "Execute Parallel Root")
			span.SetAttributesString(StringAttr{"query.database", db})
			defer span.End()

			data, err := rootState.executeForDatabaseRoots(ctx1, db, fields)
			if err != nil {
				span.Error(err)
			}

			results[idx] = dbjoin.DBResult{
				Database:       db,
				Data:           data,
				FragmentHits:   rootState.fragmentHits.Load(),
				FragmentMisses: rootState.fragmentMisses.Load(),
				Err:            err,
			}
		}(i, dbName, rootFields)
		i++
	}

	wg.Wait()
	for _, result := range results {
		if result.FragmentHits != 0 {
			s.fragmentHits.Add(result.FragmentHits)
		}
		if result.FragmentMisses != 0 {
			s.fragmentMisses.Add(result.FragmentMisses)
		}
	}

	merged, err := dbjoin.MergeRootResults(results)
	if err != nil {
		return err
	}
	s.data = merged
	return nil
}

func (s *gstate) executeForDatabaseRoots(ctx context.Context, dbName string, rootFields []string) (json.RawMessage, error) {
	dbCtx, ok := s.gj.GetDatabase(dbName)
	if !ok {
		return nil, fmt.Errorf("database not found: %s", dbName)
	}

	if s.r.operation == qcode.QTMutation {
		if dbConf, ok := s.gj.conf.Databases[dbName]; ok && dbConf.ReadOnly {
			return nil, fmt.Errorf("mutations blocked: database %s is read-only", dbName)
		}
	}

	subQuery, err := dbjoin.BuildDatabaseQuery(s.r.query, rootFields)
	if err != nil {
		return nil, fmt.Errorf("failed to build sub-query for %s: %w", dbName, err)
	}

	var vars map[string]json.RawMessage
	if len(s.r.aschema) != 0 {
		vars = s.r.aschema
	} else {
		vars = s.vmap
	}

	qc, err := dbCtx.qcodeCompiler.Compile(subQuery, vars, s.r.namespace)
	if err != nil {
		return nil, fmt.Errorf("qcode compile failed for %s: %w", dbName, err)
	}

	var sqlBuf bytes.Buffer
	md, err := dbCtx.psqlCompiler.Compile(&sqlBuf, qc)
	if err != nil {
		return nil, fmt.Errorf("sql compile failed for %s: %w", dbName, err)
	}

	conn, err := dbCtx.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for %s: %w", dbName, err)
	}
	defer conn.Close()

	args, err := s.gj.argList(ctx, md, vars, s.r.requestconfig, false, dbCtx.psqlCompiler)
	if err != nil {
		return nil, fmt.Errorf("failed to build args for %s: %w", dbName, err)
	}

	querySQL, queryArgs, err := prepareQueryArgsForDB(dbCtx.dbtype, sqlBuf.String(), args.values)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query args for %s: %w", dbName, err)
	}

	cacheKey := s.dbFragmentKey(ctx, fragmentKindDBRoot, dbName, querySQL, queryArgs, qc)
	produce := func(c context.Context, useConn *sql.Conn) ([]byte, error) {
		raw, err := scanJSONRow(c, dbCtx.dbtype, useConn, nil, querySQL, queryArgs)
		if err == sql.ErrNoRows {
			return json.RawMessage(`{}`), nil
		}
		if err != nil {
			return nil, fmt.Errorf("query execution failed for %s: %w", dbName, err)
		}
		encrypted, _, err := encryptResultFragment(raw, s.gj.printFormat, s.gj.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("encryption failed for %s: %w", dbName, err)
		}
		return encrypted, nil
	}

	if cached, ok := s.fragmentCacheGet(ctx, cacheKey, func() ([]byte, []RowRef, CacheEntryOptions, error) {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), swrRefreshTimeout)
		defer cancel()
		refreshConn, err := dbCtx.db.Conn(c)
		if err != nil {
			return nil, nil, CacheEntryOptions{}, err
		}
		defer refreshConn.Close()
		data, err := produce(c, refreshConn)
		if err != nil {
			return nil, nil, CacheEntryOptions{}, err
		}
		cachedData, refs, err := s.processDBFragmentForCache(dbName, qc, data)
		return cachedData, refs, CacheEntryOptions{}, err
	}); ok {
		return s.resolveDatabaseRootRemotes(ctx, dbName, qc, cached)
	}

	start := time.Now()
	data, err := produce(ctx, conn)
	if err != nil {
		return nil, err
	}
	if cachedData, refs, perr := s.processDBFragmentForCache(dbName, qc, data); perr == nil {
		s.fragmentCacheSet(ctx, cacheKey, cachedData, refs, start, CacheEntryOptions{})
	}

	return s.resolveDatabaseRootRemotes(ctx, dbName, qc, data)
}

func (s *gstate) resolveDatabaseRootRemotes(
	ctx context.Context,
	dbName string,
	qc *qcode.QCode,
	data []byte,
) (json.RawMessage, error) {
	if qc == nil || qc.Remotes == 0 || len(data) == 0 {
		return json.RawMessage(data), nil
	}

	sub := gstate{
		gj:        s.gj,
		r:         cloneGraphqlReq(s.r),
		cs:        &cstate{st: stmt{qc: qc}},
		data:      injectRemoteMarkers(data, qc),
		database:  dbName,
		skipCache: s.skipCache,
	}
	err := sub.execRemoteJoin(ctx)
	if hits := sub.fragmentHits.Load(); hits != 0 {
		s.fragmentHits.Add(hits)
	}
	if misses := sub.fragmentMisses.Load(); misses != 0 {
		s.fragmentMisses.Add(misses)
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(sub.data), nil
}
