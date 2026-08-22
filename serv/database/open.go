package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/dbadapter"
	"go.uber.org/zap"
)

// DriverName re-exports for consumers that need the raw driver names.
const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// DefaultDBType is used when a source does not specify a type.
const DefaultDBType = "postgres"

// Options controls how a database connection is opened.
type Options struct {
	Config     Config
	AppName    string
	OpenDBName bool
	Filesystem core.FS
	Logger     *zap.SugaredLogger
	Retry      bool
}

// normalizeType lowercases and applies the default engine type.
func normalizeType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		return DefaultDBType
	}
	return t
}

// sourceConfig converts configuration values into the engine-neutral input
// consumed by registered adapters.
func sourceConfig(c Config, opts Options) dbadapter.SourceConfig {
	return dbadapter.SourceConfig{
		Type:     normalizeType(c.Type),
		Settings: c.Settings,
		Flat: dbadapter.FlatFields{
			ConnString:    c.ConnString,
			Path:          c.Path,
			EncryptionKey: c.EncryptionKey,
			Host:          c.Host,
			Port:          c.Port,
			User:          c.User,
			Password:      c.Password,
			DBName:        c.DBName,
			Schema:        c.Schema,
			AppName:       opts.AppName,
			OpenDBName:    opts.OpenDBName,
			EnableTLS:     c.EnableTLS,
			ServerName:    c.ServerName,
			ServerCert:    c.ServerCert,
		},
		GetFile: func(path string) ([]byte, error) {
			if opts.Filesystem == nil {
				return nil, errors.New("tls: filesystem is required for server_cert paths")
			}
			return opts.Filesystem.Get(path)
		},
	}
}

func sourceConfigFromCore(name string, c core.DatabaseConfig, appOpenDBName bool) dbadapter.SourceConfig {
	sc := sourceConfig(Config{
		Type:          c.Type,
		ConnString:    c.ConnString,
		Path:          c.Path,
		EncryptionKey: c.EncryptionKey,
		Host:          c.Host,
		Port:          c.Port,
		User:          c.User,
		Password:      c.Password,
		DBName:        c.DBName,
		Schema:        c.Schema,
		EnableTLS:     c.EnableTLS,
		ServerName:    c.ServerName,
		ServerCert:    c.ServerCert,
	}, Options{AppName: "", OpenDBName: appOpenDBName})
	if sc.Flat.DBName == "" && sc.Type == DriverPostgres {
		sc.Flat.DBName = name
	}
	return sc
}

// SourceConfigFor exposes the engine-neutral resolution of options for
// tooling and tests without opening a connection.
func SourceConfigFor(opts Options) dbadapter.SourceConfig {
	return sourceConfig(opts.Config, opts)
}

// OpenCore opens a named database from the core source configuration. It is
// used for multi-source service startup where each source has its own name.
func OpenCore(ctx context.Context, name string, c core.DatabaseConfig) (*sql.DB, error) {
	timeout := c.PingTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	a, err := dbadapter.Lookup(normalizeType(c.Type))
	if err != nil {
		return nil, fmt.Errorf("database %q: %w", name, err)
	}
	db, err := a.Open(ctx, sourceConfigFromCore(name, c, false))
	if err != nil {
		return nil, fmt.Errorf("database %q: %w", name, err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("ping %s: %w", name, err)
	}
	return db, nil
}

// Open opens and pings a supported database.
func Open(opts Options) (*sql.DB, error) {
	sc := sourceConfig(opts.Config, opts)

	open := func() (*sql.DB, error) {
		a, err := dbadapter.Lookup(sc.Type)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return a.Open(ctx, sc)
	}

	build := func(db *sql.DB) error {
		configure(db, opts.Config)
		return db.Ping()
	}

	if !opts.Retry {
		db, err := openOnce(open, build)
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	var lastErr error
	for attempt := 0; attempt <= 50; attempt++ {
		db, err := openOnce(open, build)
		if err == nil {
			return db, nil
		}
		lastErr = err
		if opts.Logger != nil {
			opts.Logger.Warnf("database connection failed: %s", err)
		}
		time.Sleep(time.Duration(attempt*100) * time.Millisecond)
	}
	return nil, lastErr
}

func openOnce(open func() (*sql.DB, error), build func(*sql.DB) error) (*sql.DB, error) {
	db, err := open()
	if err != nil {
		return nil, fmt.Errorf("database connection: %w", err)
	}
	if err := build(db); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("database connection: %w", err)
	}
	return db, nil
}

// configure applies pooling limits. The retention guard keeps SQLite pools
// from silently dropping keyed idle connections when callers pass zero-value
// configs; other engines keep database/sql defaults.
func configure(db *sql.DB, c Config) {
	ps := c.PoolSize
	if strings.EqualFold(strings.TrimSpace(c.Type), DriverSQLite) && ps <= 0 {
		ps = 4
	}
	db.SetMaxIdleConns(ps)
	db.SetMaxOpenConns(c.MaxConnections)
	db.SetConnMaxIdleTime(c.MaxConnIdleTime)
	db.SetConnMaxLifetime(c.MaxConnLifeTime)
}
