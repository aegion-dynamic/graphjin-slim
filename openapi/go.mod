module github.com/aegion-dynamic/graphjin-slim/openapi/v3

go 1.25.0

require github.com/aegion-dynamic/graphjin-slim/core/v3 v3.0.2

require (
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	golang.org/x/sync v0.20.0 // indirect
)

replace github.com/aegion-dynamic/graphjin-slim/core/v3 => ../core
