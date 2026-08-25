package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	sqlcipher "github.com/aquaticcalf/sqlcipher"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/dbadapter"
)

// engineName is this adapter's key in the dbadapter registry; it matches
// serv/database's DriverSQLite so config selects the right engine.
const engineName = "sqlite"

func init() {
	dbadapter.Register(adapter{})
	// Keep the legacy database/sql registration name alive for direct
	// consumers (seeding tools, tests) that open "sqlite" themselves.
	sql.Register(engineName, &sqlcipher.Driver{})
}

type adapter struct{}

func (adapter) Name() string { return engineName }

// Open resolves settings from the source config and connects.
//
// Recognized Settings keys (engine-keyed yaml section `sqlite:`):
//
//	path           string   database file path
//	conn_string    string   overrides path when set
//	encryption_key string   enables SQLCipher when set
//	pragmas        []any    ordered pragma statements ("name(value)")
//
// Legacy flat fields are used as fallback for keys absent from Settings.
func (adapter) Open(ctx context.Context, sc dbadapter.SourceConfig) (*sql.DB, error) {
	return sqlcipher.Open(ctx, optionsFrom(sc))
}

func optionsFrom(sc dbadapter.SourceConfig) sqlcipher.Options {
	get := func(key string) (string, bool) {
		if sc.Settings == nil {
			return "", false
		}
		v, ok := sc.Settings[key]
		if !ok || v == nil {
			return "", false
		}
		s, ok := v.(string)
		if !ok {
			s = fmt.Sprintf("%v", v)
		}
		return s, true
	}

	o := sqlcipher.Options{
		Path:       sc.Flat.Path,
		ConnString: sc.Flat.ConnString,
		Key:        sc.Flat.EncryptionKey,
	}
	if v, ok := get("path"); ok {
		o.Path = v
	}
	if v, ok := get("conn_string"); ok {
		o.ConnString = v
	}
	if v, ok := get("encryption_key"); ok {
		o.Key = v
	}
	if raw, ok := sc.Settings["pragmas"].([]any); ok {
		for _, p := range raw {
			if s, ok := p.(string); ok && s != "" {
				o.Pragmas = append(o.Pragmas, s)
			} else if s := fmt.Sprintf("%v", p); s != "" {
				o.Pragmas = append(o.Pragmas, s)
			}
		}
	}
	return o
}
