package postgres

// Postgres adapter: pgx/v5 connection construction and TLS material loading.
// Selected by db type "postgres", mirroring the dialect/postgres.go vs
// dialect/sqlite.go sibling layout in core.

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// DriverPostgres is the database/sql driver name (pgx stdlib).
const DriverPostgres = "pgx"

// Options configures a PostgreSQL connection.
type Options struct {
	ConnString string
	Host       string
	Port       uint16
	User       string
	Password   string
	DBName     string
	Schema     string
	AppName    string
	OpenDBName bool
	EnableTLS  bool
	ServerName string
	ServerCert string

	// GetFile loads server_cert content when ServerCert is not inline PEM.
	// Optional: nil means "cert must be inline PEM".
	GetFile func(path string) ([]byte, error)
}

// BuildConn resolves the pgx connection configuration into a
// registered-connector DSN string understood by sql.Open("pgx", ...).
func BuildConn(o Options) (string, error) {
	cfg, err := pgx.ParseConfig(o.ConnString)
	if err != nil {
		return "", err
	}
	if o.Host != "" {
		cfg.Host = o.Host
	}
	if o.Port != 0 {
		cfg.Port = o.Port
	}
	if o.User != "" {
		cfg.User = o.User
	}
	if o.Password != "" {
		cfg.Password = o.Password
	}
	if o.OpenDBName {
		cfg.Database = o.DBName
	}
	if o.Schema != "" {
		if cfg.RuntimeParams == nil {
			cfg.RuntimeParams = map[string]string{}
		}
		cfg.RuntimeParams["search_path"] = o.Schema
	}
	if o.AppName != "" {
		if cfg.RuntimeParams == nil {
			cfg.RuntimeParams = map[string]string{}
		}
		cfg.RuntimeParams["application_name"] = o.AppName
	}
	if err := applyTLS(cfg, o); err != nil {
		return "", err
	}
	return stdlib.RegisterConnConfig(cfg), nil
}

func applyTLS(cfg *pgx.ConnConfig, o Options) error {
	if !o.EnableTLS {
		return nil
	}
	if o.ServerName == "" || o.ServerCert == "" {
		return errors.New("tls: server_name and server_cert are required")
	}
	certData := []byte(o.ServerCert)
	if !strings.Contains(o.ServerCert, "--BEGIN ") {
		if o.GetFile == nil {
			return errors.New("tls: filesystem is required for server_cert paths")
		}
		var err error
		certData, err = o.GetFile(o.ServerCert)
		if err != nil {
			return fmt.Errorf("tls: %w", err)
		}
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(string(certData), `\n`, "\n"))) {
		return errors.New("tls: failed to append server certificate")
	}
	cfg.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, ServerName: o.ServerName}
	return nil
}
