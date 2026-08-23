package engine

import (
	"fmt"
	"sort"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/langadapter"
)

// languages returns the query languages bound to this database. They are
// constructed eagerly at context creation via the input seam; this
// lazy path only serves contexts created before binding existed.
func (db *dbContext) languages(gj *graphjinEngine) map[string]langadapter.Language {
	db.langsMu.Lock()
	defer db.langsMu.Unlock()
	if db.langs == nil && db.schema != nil && gj != nil {
		langs, err := gj.newLanguages(db)
		if err == nil {
			db.langs = langs
		}
	}
	return db.langs
}

// GetLanguage resolves a query language for the named database. An empty
// dbName selects the primary database.
func (gj *graphjinEngine) GetLanguage(dbName, langName string) (langadapter.Language, error) {
	if langName == "" {
		langName = langadapter.DefaultLanguageName
	}
	if dbName == "" {
		dbName = gj.defaultDB
	}
	ctx, ok := gj.GetDatabase(dbName)
	if !ok {
		return nil, fmt.Errorf("database not found: %s", dbName)
	}
	l, ok := ctx.languages(gj)[langName]
	if !ok {
		return nil, fmt.Errorf("database %s: no language %q (available: %v)",
			dbName, langName, gj.ListLanguages(dbName))
	}
	return l, nil
}

// ListLanguages lists query languages available on the named database.
// An empty dbName selects the primary database.
func (gj *graphjinEngine) ListLanguages(dbName string) []string {
	if dbName == "" {
		dbName = gj.defaultDB
	}
	ctx, ok := gj.GetDatabase(dbName)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(ctx.languages(gj)))
	for n := range ctx.languages(gj) {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ListLanguages lists the query languages available on the named
// database (an empty name selects the primary database).
func (g *GraphJin) ListLanguages(dbName string) ([]string, error) {
	gj, err := g.getEngine()
	if err != nil {
		return nil, err
	}
	return gj.ListLanguages(dbName), nil
}
