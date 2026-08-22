module github.com/aegion-dynamic/graphjin-slim/openapi/v3

go 1.26.6

require (
	github.com/aegion-dynamic/graphjin-slim/core/v3 v3.35.0
	github.com/aegion-dynamic/graphjin-slim/serv/v3 v3.35.0
)

require (
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	golang.org/x/sync v0.20.0 // indirect
)

replace github.com/aegion-dynamic/graphjin-slim/core/v3 => ../core

replace github.com/aegion-dynamic/graphjin-slim/serv/v3 => ../serv
