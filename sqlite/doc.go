// Package sqlite provides GraphJin's SQLite database/sql driver, built
// directly on a vendored SQLCipher amalgamation (compiled into the final
// application via cgo).
//
// One engine, one driver name ("sqlite"). Encryption is policy, not a
// different stack:
//
//	encryption_key == "" -> standard SQLite behavior through SQLCipher
//	encryption_key != "" -> PRAGMA key applied as the first statement on
//	                       every physical connection
//
// Provenance of the vendored C sources (see scripts/gen-sqlite3.sh):
//
//	SQLCipher       4.18.0 community   (tag v4.18.0)
//	SQLite baseline 3.53.4
//	Pinned commit   63697beb0fafcb61faa7a3e6fd267036548ab11b
//
// Runtime dependency model: SQLCipher code is statically linked into the
// application; OpenSSL's libcrypto remains a dynamic system dependency.
package sqlite
