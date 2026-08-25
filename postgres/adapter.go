package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/dbadapter"
)

func init() {
	dbadapter.Register(adapter{})
}

type adapter struct{}

// Name is the configuration-facing engine key. It must match
// serv/database's DriverPostgres ("postgres") so conf.DB.Type selects
// this adapter; the raw database/sql driver stays "pgx" internally.
func (adapter) Name() string { return "postgres" }

// Open resolves settings from the source config and connects.
//
// Recognized Settings keys (engine-keyed yaml section `postgres:`):
//
//	host, port, user, password, db_name, schema,
//	app_name, open_db_name, enable_tls, server_name, server_cert
//
// server_cert may be inline PEM or a path resolved through SourceConfig.GetFile.
// Legacy flat fields are used as fallback for absent keys.
func (adapter) Open(ctx context.Context, sc dbadapter.SourceConfig) (*sql.DB, error) {
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
	getInt := func(key string) (uint16, bool) {
		sv, ok := get(key)
		if !ok {
			return 0, false
		}
		n, err := strconv.ParseUint(sv, 10, 16)
		if err != nil {
			if f, ferr := strconv.ParseFloat(sv, 64); ferr == nil && f >= 0 && f <= 65535 {
				return uint16(f), true
			}
			return 0, false
		}
		return uint16(n), true
	}

	o := Options{
		ConnString: sc.Flat.ConnString,
		Host:       sc.Flat.Host,
		Port:       sc.Flat.Port,
		User:       sc.Flat.User,
		Password:   sc.Flat.Password,
		DBName:     sc.Flat.DBName,
		Schema:     sc.Flat.Schema,
		AppName:    sc.Flat.AppName,
		OpenDBName: sc.Flat.OpenDBName,
		EnableTLS:  sc.Flat.EnableTLS,
		ServerName: sc.Flat.ServerName,
		ServerCert: sc.Flat.ServerCert,
		GetFile:    sc.GetFile,
	}
	setStr := func(k string, dst *string) {
		if v, ok := get(k); ok {
			*dst = v
		}
	}
	setStr("host", &o.Host)
	setStr("user", &o.User)
	setStr("password", &o.Password)
	setStr("db_name", &o.DBName)
	setStr("schema", &o.Schema)
	setStr("app_name", &o.AppName)
	setStr("server_name", &o.ServerName)
	setStr("server_cert", &o.ServerCert)
	if v, ok := getInt("port"); ok {
		o.Port = v
	}
	for _, k := range []string{"open_db_name", "enable_tls"} {
		v, ok := get(k)
		if !ok {
			continue
		}
		b := strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes") || strings.EqualFold(v, "on")
		switch k {
		case "open_db_name":
			o.OpenDBName = b
		case "enable_tls":
			o.EnableTLS = b
		}
	}

	dsn, err := BuildConn(o)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(DriverPostgres, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
