package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"go.uber.org/zap"
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
		d, derr := buildSQLiteDSN(c.ConnString, c.Path, c.EncryptionKey)
		if derr != nil {
			return nil, fmt.Errorf("sqlite database %q: %w", name, derr)
		}
		dsn = d
		driverName = DriverSQLite
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
	if c.EncryptionKey != "" {
		if err := validateSQLCipher(db); err != nil {
			db.Close() //nolint:errcheck
			return nil, fmt.Errorf("sqlite database %q: %w", name, err)
		}
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
		if opts.Config.EncryptionKey != "" {
			if err := validateSQLCipher(db); err != nil {
				db.Close() //nolint:errcheck
				return nil, err
			}
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

var connection = Connection

func Connection(opts Options) (string, string, error) {
	c := opts.Config
	dbType := strings.ToLower(strings.TrimSpace(c.Type))
	if dbType == "" {
		dbType = "postgres"
	}
	switch dbType {
	case "postgres":
		dsn, err := buildPostgresConn(c, opts)
		if err != nil {
			return "", "", err
		}
		return DriverPostgres, dsn, nil
	case "sqlite":
		dsn, err := buildSQLiteDSN(c.ConnString, c.Path, c.EncryptionKey)
		if err != nil {
			return "", "", err
		}
		return DriverSQLite, dsn, nil
	default:
		return "", "", fmt.Errorf("unsupported database type %q: supported types are postgres, sqlite", dbType)
	}
}

func configure(db *sql.DB, c Config) {
	// Pool retention guard: a zero-value programmatic SQLite config must not
	// silently disable idle-connection retention — with SQLCipher every
	// dropped connection is re-paid (KDF) on the next request. Explicitly
	// configured positive sizes are preserved; Postgres keeps database/sql
	// defaults.
	ps := c.PoolSize
	if strings.EqualFold(strings.TrimSpace(c.Type), "sqlite") && ps <= 0 {
		ps = 4
	}
	db.SetMaxIdleConns(ps)
	db.SetMaxOpenConns(c.MaxConnections)
	db.SetConnMaxIdleTime(c.MaxConnIdleTime)
	db.SetConnMaxLifetime(c.MaxConnLifeTime)
}
