package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/dbadapter"
)

func init() {
	dbadapter.Register(adapter{})
}

type adapter struct{}

func (adapter) Name() string { return DriverName }

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
	opts := optionsFrom(sc)
	return Open(ctx, opts)
}

func optionsFrom(sc dbadapter.SourceConfig) Options {
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

	o := Options{
		Path:          sc.Flat.Path,
		ConnString:    sc.Flat.ConnString,
		EncryptionKey: sc.Flat.EncryptionKey,
	}
	if v, ok := get("path"); ok {
		o.Path = v
	}
	if v, ok := get("conn_string"); ok {
		o.ConnString = v
	}
	if v, ok := get("encryption_key"); ok {
		o.EncryptionKey = v
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

