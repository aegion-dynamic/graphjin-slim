package database

// SQLite adapter built on github.com/mattn/go-sqlite3 (patched: ConnectHook
// runs before mattn's built-in pragmas — required for SQLCipher reopens).
//
// One driver serves both modes:
//   - Config.EncryptionKey == ""  → plain SQLite (files stay plaintext)
//   - Config.EncryptionKey != ""  → SQLCipher: PRAGMA key applied first on
//     every physical connection; validated via PRAGMA cipher_version.
//
// Build recipe for encryption support:
//
//	./scripts/apply-sqlite3-patch.sh
//	CGO_CFLAGS="-I$DIST/include" \
//	CGO_LDFLAGS="-L$DIST/lib -Wl,-rpath,$DIST/lib" \
//	go build -tags libsqlite3 ./...
//
// Without that tag the binary links vanilla SQLite: plaintext mode works,
// encrypted mode fails fast with a clear configuration error.

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"strings"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// DriverSQLite is the database/sql name under which the adapter registers.
const DriverSQLite = "sqlite"

// dsnKeyParam carries the encryption key inside the DSN. Internal only:
// never documented, stripped before the statement reaches mattn.
const dsnKeyParam = "_gj_encryption_key"

func init() {
	sql.Register(DriverSQLite, sqliteDriver{})
}

type sqliteDriver struct{}

// Open wraps mattn's driver. The encryption key (if any) is applied via
// ConnectHook immediately after the connection opens — before any other
// statement, as SQLCipher requires — followed by any _pragma parameters.
func (sqliteDriver) Open(dsn string) (driver.Conn, error) {
	base, key, pragmas := splitDSN(dsn)
	drv := &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			if key != "" {
				if _, err := conn.Exec(
					fmt.Sprintf("PRAGMA key = '%s'", strings.ReplaceAll(key, "'", "''")),
					nil); err != nil {
					return fmt.Errorf("sqlite: apply encryption key: %w", err)
				}
			}
			for _, p := range pragmas {
				if _, err := conn.Exec("PRAGMA "+p, nil); err != nil {
					return fmt.Errorf("sqlite: pragma %q: %w", p, err)
				}
			}
			return nil
		},
	}
	return drv.Open(base)
}

// splitDSN separates the base DSN, the internal encryption-key parameter,
// and ordered _pragma parameters.
func splitDSN(dsn string) (base, key string, pragmas []string) {
	i := strings.IndexByte(dsn, '?')
	if i < 0 {
		return dsn, "", nil
	}
	base, query := dsn[:i], dsn[i+1:]
	for _, kv := range strings.Split(query, "&") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || v == "" {
			continue
		}
		if dec, err := url.QueryUnescape(v); err == nil {
			v = dec
		}
		switch k {
		case dsnKeyParam:
			key = v
		case "_pragma":
			pragmas = append(pragmas, v)
		}
	}
	return base, key, pragmas
}

// appendDSNParam adds a parameter to a base DSN, handling the ?/& separator.
func appendDSNParam(base, kv string) string {
	if strings.ContainsRune(base, '?') {
		return base + "&" + kv
	}
	return base + "?" + kv
}

// buildSQLiteDSN resolves the configured connection target and attaches the
// internal encryption-key parameter when configured.
func buildSQLiteDSN(connString, path, encryptionKey string) (string, error) {
	base := connString
	if base == "" {
		base = path
	}
	if base == "" {
		return "", errors.New("sqlite requires a connection_string or path")
	}
	if encryptionKey != "" {
		base = appendDSNParam(base, dsnKeyParam+"="+url.QueryEscape(encryptionKey))
	}
	return base, nil
}

// validateSQLCipher fails loudly when encryption was requested but the linked
// library is vanilla SQLite (no codec). PRAGMA cipher_version returns a row
// only on SQLCipher builds.
func validateSQLCipher(db *sql.DB) error {
	var ver string
	err := db.QueryRow("PRAGMA cipher_version").Scan(&ver)
	if err != nil || strings.TrimSpace(ver) == "" {
		return errors.New("sqlite: encryption_key is set but this binary links plain SQLite; rebuild with \"-tags libsqlite3\" and CGO flags pointing at your libsqlcipher.so (see scripts/apply-sqlite3-patch.sh)")
	}
	return nil
}
