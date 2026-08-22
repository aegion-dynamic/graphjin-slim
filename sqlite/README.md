# SQLite

`sqlite` is GraphJin's SQLCipher-backed SQLite database/sql driver.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/sqlite/v3
```

One engine, one driver name (`"sqlite"`). Encryption is policy, not a
different stack:

- `EncryptionKey == ""` -> standard SQLite behavior through SQLCipher; files
  remain standard plaintext databases.
- `EncryptionKey != ""` -> SQLCipher: the key is applied as the first
  statement on every physical connection, and the open is validated against
  `PRAGMA cipher_version`.

## Usage

```go
db, err := sqlite.Open(ctx, sqlite.Options{
    Path:          "./app.db",
    EncryptionKey: os.Getenv("DB_KEY"),
})
```

## Layout

| File | Responsibility |
| --- | --- |
| `driver.go` | database/sql driver: Conn, Stmt, Rows, Tx, multi-statement exec |
| `key.go` | cipher availability check and key statement building |
| `open.go` | Options and Open entry point |
| `sqlite3.c/h` | committed SQLCipher amalgamation - the actual build input |
| `upstream/` | gitignored; where gen.sh clones pinned upstream sources |
| `gen.sh` | clones pinned upstream and regenerates the amalgamation |

Consumers need no submodules: `go get` + `go build` compiles the committed
amalgamation directly. A C toolchain is required.

## Upgrading SQLCipher

1. Update `PIN_SQLCIPHER` in `gen.sh` to the new release commit.
2. Run `./gen.sh` (clones the pinned commit into gitignored `upstream/`) and
   commit the regenerated amalgamation.
3. Bump the module version via a normal master push - auto-release handles
   tags and the GitHub Release.
