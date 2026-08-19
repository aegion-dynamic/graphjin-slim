package bigqueryemu

import "github.com/aegion-dynamic/graphjin-slim/tests/v3/hostedemu/snowflake/catalog"

func ParseSeed(path string) (*catalog.Schema, error) {
	return catalog.ParseSeed(path)
}
