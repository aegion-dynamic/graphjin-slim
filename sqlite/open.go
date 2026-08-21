package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Options configures a SQLite connection.
type Options struct {
	// Path is the database file path (or :memory:, file: URI, etc).
	Path string
	// ConnString overrides Path when set; passed through verbatim apart
	// from the internal encryption-key and pragma parameters.
	ConnString string
	// EncryptionKey enables SQLCipher at-rest encryption when non-empty.
	// Applied as the first statement on every physical connection.
	EncryptionKey string
	// Pragmas are applied after the key, in order. Entries use the form
	// "name(value)" or "name", e.g. "busy_timeout(5000)".
	Pragmas []string
}

// Open connects to the database described by opts and pings it.
func Open(ctx context.Context, opts Options) (*sql.DB, error) {
	dsn, err := BuildDSN(opts)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(DriverName, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// BuildDSN resolves the connection target and attaches the internal
// encryption-key and pragma parameters.
func BuildDSN(opts Options) (string, error) {
	base := opts.ConnString
	if base == "" {
		base = opts.Path
	}
	if base == "" {
		return "", errors.New("sqlite requires a path or conn_string")
	}
	if opts.EncryptionKey != "" {
		base = appendParam(base, "_gj_encryption_key="+queryEscape(opts.EncryptionKey))
	}
	for _, p := range opts.Pragmas {
		base = appendParam(base, "_pragma="+queryEscape(p))
	}
	return base, nil
}

func appendParam(base, kv string) string {
	if strings.ContainsRune(base, '?') {
		return base + "&" + kv
	}
	return base + "?" + kv
}

func queryEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'),
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		default:
			const hex = "0123456789ABCDEF"
			b.WriteString("%")
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xF])
		}
	}
	return b.String()
}
