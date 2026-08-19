package database

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	_ "modernc.org/sqlite"
)

// Options controls how a database connection is opened.
type Options struct {
	Config     Config
	AppName    string
	OpenDBName bool
	Filesystem core.FS
	Logger     *zap.SugaredLogger
	Retry      bool
}

// OpenCore opens a named database from the core source configuration. It is
// used for multi-source service startup where each source has its own name.
func OpenCore(ctx context.Context, name string, c core.DatabaseConfig) (*sql.DB, error) {
	dbType := strings.ToLower(strings.TrimSpace(c.Type))
	if dbType == "" {
		dbType = "postgres"
	}
	timeout := c.PingTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	var driverName, dsn string
	switch dbType {
	case "sqlite":
		dsn = c.Path
		if dsn == "" {
			dsn = c.ConnString
		}
		if dsn == "" {
			return nil, fmt.Errorf("sqlite database %q requires a path or connection_string", name)
		}
		driverName = "sqlite"
	case "postgres":
		dsn = c.ConnString
		if dsn == "" {
			dbName := c.DBName
			if dbName == "" {
				dbName = name
			}
			dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", c.Host, c.Port, c.User, c.Password, dbName)
		}
		driverName = "pgx"
	default:
		return nil, fmt.Errorf("unsupported database type %q: supported types are postgres, sqlite", c.Type)
	}
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", driverName, err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("ping %s: %w", driverName, err)
	}
	return db, nil
}

// Open opens and pings a supported database.
func Open(opts Options) (*sql.DB, error) {
	driverName, dsn, err := connection(opts)
	if err != nil {
		return nil, err
	}
	open := func() (*sql.DB, error) {
		db, err := sql.Open(driverName, dsn)
		if err != nil {
			return nil, err
		}
		configure(db, opts.Config)
		if err := db.Ping(); err != nil {
			db.Close() //nolint:errcheck
			return nil, err
		}
		return db, nil
	}

	if !opts.Retry {
		return openOnce(open)
	}
	var lastErr error
	for attempt := 0; attempt <= 50; attempt++ {
		db, err := openOnce(open)
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

func openOnce(open func() (*sql.DB, error)) (*sql.DB, error) {
	db, err := open()
	if err != nil {
		return nil, fmt.Errorf("database connection: %w", err)
	}
	return db, nil
}

func connection(opts Options) (string, string, error) {
	c := opts.Config
	dbType := strings.ToLower(strings.TrimSpace(c.Type))
	if dbType == "" {
		dbType = "postgres"
	}
	switch dbType {
	case "postgres":
		cfg, err := pgx.ParseConfig(c.ConnString)
		if err != nil {
			return "", "", err
		}
		if c.Host != "" {
			cfg.Host = c.Host
		}
		if c.Port != 0 {
			cfg.Port = c.Port
		}
		if c.User != "" {
			cfg.User = c.User
		}
		if c.Password != "" {
			cfg.Password = c.Password
		}
		if opts.OpenDBName {
			cfg.Database = c.DBName
		}
		if c.Schema != "" {
			if cfg.RuntimeParams == nil {
				cfg.RuntimeParams = map[string]string{}
			}
			cfg.RuntimeParams["search_path"] = c.Schema
		}
		if opts.AppName != "" {
			if cfg.RuntimeParams == nil {
				cfg.RuntimeParams = map[string]string{}
			}
			cfg.RuntimeParams["application_name"] = opts.AppName
		}
		if err := applyTLS(cfg, c, opts.Filesystem); err != nil {
			return "", "", err
		}
		return "pgx", stdlib.RegisterConnConfig(cfg), nil
	case "sqlite":
		dsn := c.ConnString
		if dsn == "" {
			dsn = c.Path
		}
		if dsn == "" {
			return "", "", errors.New("sqlite requires a connection string or path")
		}
		return "sqlite", dsn, nil
	default:
		return "", "", fmt.Errorf("unsupported database type %q: supported types are postgres, sqlite", c.Type)
	}
}

func configure(db *sql.DB, c Config) {
	db.SetMaxIdleConns(c.PoolSize)
	db.SetMaxOpenConns(c.MaxConnections)
	db.SetConnMaxIdleTime(c.MaxConnIdleTime)
	db.SetConnMaxLifetime(c.MaxConnLifeTime)
}

func applyTLS(cfg *pgx.ConnConfig, c Config, fs core.FS) error {
	if !c.EnableTLS {
		return nil
	}
	if c.ServerName == "" || c.ServerCert == "" {
		return errors.New("tls: server_name and server_cert are required")
	}
	certData := []byte(c.ServerCert)
	if !strings.Contains(c.ServerCert, "--BEGIN ") {
		if fs == nil {
			return errors.New("tls: filesystem is required for server_cert paths")
		}
		var err error
		certData, err = fs.Get(c.ServerCert)
		if err != nil {
			return fmt.Errorf("tls: %w", err)
		}
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(string(certData), `\n`, "\n"))) {
		return errors.New("tls: failed to append server certificate")
	}
	cfg.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, ServerName: c.ServerName}
	return nil
}
