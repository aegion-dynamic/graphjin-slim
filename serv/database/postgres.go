package database

// Postgres adapter: pgx/v5 construction and TLS material loading.
// Selected through Connection() for db type "postgres", mirroring the
// dialect/postgres.go vs dialect/sqlite.go sibling layout in core.

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
)

// DriverPostgres is the database/sql name under which the Postgres adapter
// registers (pgx stdlib).
const DriverPostgres = "pgx"

// buildPostgresConn resolves the pgx connection configuration into a
// registered-connector DSN string understood by sql.Open("pgx", ...).
func buildPostgresConn(c Config, opts Options) (string, error) {
	cfg, err := pgx.ParseConfig(c.ConnString)
	if err != nil {
		return "", err
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
		return "", err
	}
	return stdlib.RegisterConnConfig(cfg), nil
}

func applyTLS(cfg *pgx.ConnConfig, c Config, fs core.FS) error {
	if !c.EnableTLS {
		return nil
	}
	if c.ServerName == "" || c.ServerCert == "" {
		return fmt.Errorf("tls: server_name and server_cert are required")
	}
	certData := []byte(c.ServerCert)
	if !strings.Contains(c.ServerCert, "--BEGIN ") {
		if fs == nil {
			return fmt.Errorf("tls: filesystem is required for server_cert paths")
		}
		var err error
		certData, err = fs.Get(c.ServerCert)
		if err != nil {
			return fmt.Errorf("tls: %w", err)
		}
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(string(certData), `\n`, "\n"))) {
		return fmt.Errorf("tls: failed to append server certificate")
	}
	cfg.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, ServerName: c.ServerName}
	return nil
}
