# Postgres

`postgres` is GraphJin's PostgreSQL connection adapter, built on pgx/v5.

Module path:

```text
github.com/aegion-dynamic/graphjin-slim/postgres/v3
```

It resolves connection settings into a registered pgx connector DSN for
`sql.Open("pgx", dsn)`, including search_path, application_name, and TLS
material loading. It is a separate module so consumers who only need SQLite
build without pgx, and vice versa.
