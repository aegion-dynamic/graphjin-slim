module github.com/aegion-dynamic/graphjin-slim/tests

go 1.26.6

require (
	github.com/aegion-dynamic/graphjin-slim/postgres/v3 v3.1.1
	github.com/aegion-dynamic/graphjin-slim/serv/v3 v3.0.0
	github.com/aegion-dynamic/graphjin-slim/sqlite/v3 v3.1.1
)

require (
	github.com/aegion-dynamic/graphjin-slim/core/v3 v3.33.33 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.36.0 // indirect
)

replace (
	github.com/aegion-dynamic/graphjin-slim/core/v3 => ../core
	github.com/aegion-dynamic/graphjin-slim/postgres/v3 => ../postgres
	github.com/aegion-dynamic/graphjin-slim/serv/v3 => ../serv
	github.com/aegion-dynamic/graphjin-slim/sqlite/v3 => ../sqlite
)
