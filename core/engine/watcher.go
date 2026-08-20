package engine

import (
	"context"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/introspection"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/watcher"
)

// initDBWatcher initializes the database schema watcher
func (g *GraphJin) initDBWatcher() error {
	gj := g.Load().(*graphjinEngine)
	if gj.disableDBSchemaWatcher || gj.prod {
		return nil
	}

	ps := gj.conf.DBSchemaPollDuration
	if ps < 1*time.Second {
		return nil
	}

	watcher.Start(ps, g.lifecycle, g.checkSchemaChanges, g.reloadOnSchemaChange)
	return nil
}

func (g *GraphJin) checkSchemaChanges(ctx context.Context) (bool, error) {
	gj := g.Load().(*graphjinEngine)

	for _, dbCtx := range gj.databases {
		if dbCtx.db == nil {
			continue
		}

		latestDi, err := introspection.GetDBInfo(
			ctx,
			dbCtx.db,
			dbCtx.dbtype,
			gj.conf.Blocklist)
		if err != nil {
			gj.log.Printf("database %s: schema poll error: %v", dbCtx.name, err)
			continue
		}

		if dbCtx.schema == nil {
			if len(latestDi.Tables) > 0 {
				gj.log.Printf("database %s: tables discovered, reinitializing...", dbCtx.name)
				return true, nil
			}
			continue
		}

		if latestDi.Hash() != dbCtx.dbinfo.Hash() {
			gj.log.Printf("database %s: schema change detected, reinitializing...", dbCtx.name)
			return true, nil
		}
	}
	return false, nil
}

func (g *GraphJin) reloadOnSchemaChange() error {
	g.reloadMu.Lock()
	defer g.reloadMu.Unlock()

	gj := g.Load().(*graphjinEngine)
	pdb := gj.primaryDB()
	if pdb != nil {
		if err := g.newGraphJin(gj.conf, pdb.db, nil, gj.fs, gj.opts...); err != nil {
			gj.log.Println(err)
			return err
		}
		g.fireAllSchemaCallbacks()
	}
	return nil
}
