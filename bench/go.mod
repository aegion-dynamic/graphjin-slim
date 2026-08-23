module github.com/aegion-dynamic/graphjin-slim/bench/v3

go 1.26.6

require (
	github.com/aegion-dynamic/graphjin-slim/openapi/v3 v3.0.0-00010101000000-000000000000
	github.com/aegion-dynamic/graphjin-slim/serv/v3 v3.35.1
	github.com/aegion-dynamic/graphjin-slim/sqlite/v3 v3.34.1
	github.com/uptrace/bun v1.2.11
	github.com/uptrace/bun/dialect/sqlitedialect v1.2.11
)

require (
	github.com/Code-Hex/dd v1.1.0 // indirect
	github.com/aegion-dynamic/graphjin-slim/core/v3 v3.35.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-http-utils/headers v0.0.0-20181008091004-fed159eddc2a // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/klauspost/compress v1.18.3 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/thessem/zap-prettyconsole v0.6.0 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
)

replace github.com/aegion-dynamic/graphjin-slim/core/v3 => ../core

replace github.com/aegion-dynamic/graphjin-slim/serv/v3 => ../serv

replace github.com/aegion-dynamic/graphjin-slim/openapi/v3 => ../openapi

replace github.com/aegion-dynamic/graphjin-slim/webui/v3 => ../webui

replace github.com/aegion-dynamic/graphjin-slim/sqlite/v3 => ../sqlite
