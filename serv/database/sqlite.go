package database

// SQLite adapter configuration glue.
//
// The actual SQLite driver lives in the root-level sqlite/v3 module
// (github.com/aegion-dynamic/graphjin-slim/sqlite/v3): a SQLCipher-backed
// database/sql driver that applies encryption_key as the first statement of
// every physical connection and validates PRAGMA cipher_version.

import (
	"database/sql"
	"errors"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	nativesqlite "github.com/aegion-dynamic/graphjin-slim/sqlite/v3"
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

// buildSQLiteDSN resolves the configured connection target via the sqlite
// module's DSN builder.
func buildSQLiteDSN(c Config) (string, error) {
	return nativesqlite.BuildDSN(nativesqlite.Options{
		Path:          c.Path,
		ConnString:    c.ConnString,
		EncryptionKey: c.EncryptionKey,
	})
}

// buildCoreSQLiteDSN is buildSQLiteDSN for core.DatabaseConfig sources
// opened through OpenCore.
func buildCoreSQLiteDSN(c core.DatabaseConfig) (string, error) {
	return nativesqlite.BuildDSN(nativesqlite.Options{
		Path:          c.Path,
		ConnString:    c.ConnString,
		EncryptionKey: c.EncryptionKey,
	})
}
