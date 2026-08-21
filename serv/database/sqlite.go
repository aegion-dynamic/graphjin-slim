package database

// SQLite adapter configuration glue.
//
// The actual database/sql driver lives in the native/sqlite subpackage:
// it registers itself under the name "sqlite" on import and implements all
// connection initialization ordering (encryption key first) on top of a
// vendored SQLCipher amalgamation compiled via cgo.

import (
	"database/sql"
	"errors"
	"strings"

	nativesqlite "github.com/aegion-dynamic/graphjin-slim/serv/v3/database/sqlite"
)

// DriverSQLite is the database/sql driver name used by this adapter.
const DriverSQLite = nativesqlite.DriverName

// validateSQLCipher fails loudly when encryption was requested but the
// linked engine lacks the codec (defensive: the vendored amalgamation always
// includes it).
func validateSQLCipher(db *sql.DB) error {
	if nativesqlite.CipherAvailable(db) {
		return nil
	}
	return errors.New("sqlite: encryption_key is set but the linked engine has no SQLCipher codec; rebuild GraphJin from a complete source tree")
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
		base = appendDSNParam(base, "_gj_encryption_key="+urlQueryEscape(encryptionKey))
	}
	return base, nil
}

func urlQueryEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			const hex = "0123456789ABCDEF"
			b.WriteString("%")
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xF])
		}
	}
	return b.String()
}
