package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
)

// CipherAvailable reports whether the underlying engine of db is SQLCipher
// (codec present) rather than plain SQLite.
func CipherAvailable(db *sql.DB) bool {
	var ver string
	if err := db.QueryRow("PRAGMA cipher_version").Scan(&ver); err != nil {
		return false
	}
	return strings.TrimSpace(ver) != ""
}

// applyKey returns the PRAGMA statement applying an encryption passphrase.
// The passphrase is single-quote escaped; raw hex keys may be supplied by
// the caller using SQLCipher blob syntax x'<64 hex chars>' verbatim.
func applyKey(key string) string {
	return fmt.Sprintf("PRAGMA key = '%s'", strings.ReplaceAll(key, "'", "''"))
}
