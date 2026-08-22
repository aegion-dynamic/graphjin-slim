// Package sqlite provides GraphJin's SQLite database/sql driver, built
// directly on a vendored SQLCipher amalgamation (compiled into the final
// application via cgo).
//
// One engine, one driver name ("sqlite"). Encryption is policy, not a
// different stack:
//
//	encryption_key == "" -> standard SQLite behavior through SQLCipher
//	encryption_key != "" -> PRAGMA key applied as the first statement on
//	                        every physical connection
//
// Provenance of the vendored C sources (regenerate with gen.sh): the
// amalgamation is generated from SQLCipher v4.18.0 community (SQLite
// baseline 3.53.4, upstream commit 63697beb0fafcb61faa7a3e6fd267036548ab11b)
// and committed here, so consumers need no submodules. Runtime dependency
// model: SQLCipher code is statically linked into the application;
// OpenSSL's libcrypto remains a dynamic system dependency.
package sqlite
