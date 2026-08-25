# sqlite (GraphJin engine adapter)

Registers GraphJin's SQLite database engine (`conf.DB.Type = "sqlite"`).
The underlying driver is
[aquaticcalf/sqlcipher](https://github.com/aquaticcalf/sqlcipher) — a
SQLCipher-backed `database/sql` implementation with the C amalgamation
vendored. Encryption is configured through the standard
`encryption_key` setting; with no key, databases behave as plain SQLite.
