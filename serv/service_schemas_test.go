package serv

import (
	"testing"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
)

func TestSyncSingleDBSchemas(t *testing.T) {
	newConf := func() *Config { return &Config{} }

	t.Run("copies into empty default entry", func(t *testing.T) {
		c := newConf()
		c.DB.Schemas = core.SchemasConfig{Allowed: []string{"public", "users"}, Default: "public"}
		syncSingleDBSchemas(c)
		got := c.Core.Databases[core.DefaultDBName].Schemas
		if got.Default != "public" || len(got.Allowed) != 2 {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("explicit core config wins", func(t *testing.T) {
		c := newConf()
		c.DB.Schemas = core.SchemasConfig{Allowed: []string{"public"}}
		c.Core.Databases = map[string]core.DatabaseConfig{
			core.DefaultDBName: {Schemas: core.SchemasConfig{Allowed: []string{"private"}}},
		}
		syncSingleDBSchemas(c)
		got := c.Core.Databases[core.DefaultDBName].Schemas.Allowed
		if len(got) != 1 || got[0] != "private" {
			t.Fatalf("clobbered: %v", got)
		}
	})

	t.Run("empty source is a no-op", func(t *testing.T) {
		c := newConf()
		syncSingleDBSchemas(c)
		if len(c.Core.Databases) != 0 {
			t.Fatalf("created entries: %v", c.Core.Databases)
		}
	})

	syncSingleDBSchemas(nil)
}
