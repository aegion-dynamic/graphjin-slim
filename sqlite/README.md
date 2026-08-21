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
| `cipher/` | git submodule pinning upstream sqlcipher (provenance only) |
| `gen.sh` | regenerates the amalgamation from the pinned commit |

Consumers need no submodules: `go get` + `go build` compiles the committed
amalgamation directly. A C toolchain is required.

## Upgrading SQLCipher

1. Move `cipher/` to the new upstream release tag.
2. Update `PIN_SQLCIPHER` in `gen.sh`.
3. Run `./gen.sh` and commit the regenerated amalgamation.
