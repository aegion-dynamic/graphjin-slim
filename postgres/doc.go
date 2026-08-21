// Package postgres provides GraphJin's PostgreSQL connection adapter,
// built on jackc/pgx/v5.
//
// It is a separate module so that consumers who only need SQLite can build
// without pgx, and vice versa: the adapter registers nothing globally; the
// service layer selects it by database type.
package postgres
