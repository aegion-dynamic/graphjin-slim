module github.com/aegion-dynamic/graphjin-slim/webui/v3

go 1.25.0

require (
	github.com/aegion-dynamic/graphjin-slim/core/v3 v3.35.0
	github.com/aegion-dynamic/graphjin-slim/serv/v3 v3.35.0
)

replace github.com/aegion-dynamic/graphjin-slim/core/v3 => ../core

replace github.com/aegion-dynamic/graphjin-slim/serv/v3 => ../serv
