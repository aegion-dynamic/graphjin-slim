package core

import (
	"database/sql"
	"io"

	schemapkg "github.com/aegion-dynamic/graphjin-slim/core/v3/schema"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
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
func defaultSchemaForDBType(dbType string) string    { return schemapkg.DefaultSchemaForDBType(dbType) }

// GenerateSchema generates GraphJin DDL from database introspection.
func GenerateSchema(db *sql.DB, dbType string, blocklist []string) ([]byte, error) {
	return schemapkg.GenerateSchema(db, dbType, blocklist)
}

func writeSchema(s *sdata.DBInfo, out io.Writer) error { return schemapkg.WriteSchema(s, out) }

type DiffOptions = schemapkg.DiffOptions
type SchemaOperation = schemapkg.SchemaOperation
type TemporalColumn = schemapkg.TemporalColumn
type DDLDialect = schemapkg.DDLDialect

func SupportsSchemaDDL(dbType string) bool { return schemapkg.SupportsSchemaDDL(dbType) }
func SchemaDiff(db *sql.DB, dbType string, schemaBytes []byte, blocklist []string, opts DiffOptions) ([]SchemaOperation, error) {
	return schemapkg.SchemaDiff(db, dbType, schemaBytes, blocklist, opts)
}
func SchemaDDLTemporalColumns(schemaBytes []byte) (map[string][]TemporalColumn, error) {
	return schemapkg.SchemaDDLTemporalColumns(schemaBytes)
}
func GenerateSchemaSQL(dbType string, schemaBytes []byte, blocklist []string) ([]string, error) {
	return schemapkg.GenerateSchemaSQL(dbType, schemaBytes, blocklist)
}
func GenerateDiffSQL(ops []SchemaOperation) []string { return schemapkg.GenerateDiffSQL(ops) }
func SchemaDiffMultiDB(connections map[string]*sql.DB, dbTypes map[string]string, schemaBytes []byte, blocklist []string, opts DiffOptions) (map[string][]SchemaOperation, error) {
	return schemapkg.SchemaDiffMultiDB(connections, dbTypes, schemaBytes, blocklist, opts)
}
