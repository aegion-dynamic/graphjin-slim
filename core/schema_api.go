package core

import (
	"database/sql"
	"io"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/sdata"
	schemapkg "github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
)

const (
	SchemaDDLFile           = schemapkg.SchemaDDLFile
	LegacySchemaGraphQLFile = schemapkg.LegacySchemaGraphQLFile
	SourceSchemaDDLDir      = schemapkg.SourceSchemaDDLDir
	LocalStateDir           = schemapkg.LocalStateDir
)

func SourceSchemaDDLPath(source string) string       { return schemapkg.SourceDDLPath(source) }
func RuntimeSchemaDDLPath(source string) string      { return schemapkg.RuntimeDDLPath(source) }
func RuntimeSchemaSnapshotPath(source string) string { return schemapkg.RuntimeSnapshotPath(source) }
func sanitizeSchemaDDLName(name string) string       { return schemapkg.SanitizeName(name) }

// GenerateSchema generates GraphJin DDL from database introspection.
func GenerateSchema(db *sql.DB, dbType string, blocklist []string) ([]byte, error) {
	return schemapkg.GenerateSchema(db, dbType, blocklist)
}

func writeSchema(s *sdata.DBInfo, out io.Writer) error { return schemapkg.WriteSchema(s, out) }
