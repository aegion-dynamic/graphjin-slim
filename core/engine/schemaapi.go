package engine

import (
	"database/sql"
	"fmt"
	"io"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/langadapter"
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
func SchemaDiff(db *sql.DB, dbType string, expected *sdata.DBInfo, blocklist []string, opts DiffOptions) ([]SchemaOperation, error) {
	return schemapkg.SchemaDiff(db, dbType, expected, blocklist, opts)
}
func TemporalColumns(di *sdata.DBInfo) map[string][]TemporalColumn {
	return schemapkg.TemporalColumns(di)
}
func GenerateSchemaSQLFromSchema(dbType string, di *sdata.DBInfo) ([]string, error) {
	return schemapkg.GenerateSchemaSQLFromSchema(dbType, di)
}
func GenerateDiffSQL(ops []SchemaOperation) []string { return schemapkg.GenerateDiffSQL(ops) }
func SchemaDiffMultiDB(connections map[string]*sql.DB, dbTypes map[string]string, expected *sdata.DBInfo, blocklist []string, opts DiffOptions) (map[string][]SchemaOperation, error) {
	return schemapkg.SchemaDiffMultiDB(connections, dbTypes, expected, blocklist, opts)
}

// ParseSchemaSDL parses frontend-authored schema definition text into
// neutral database metadata via the input seam. Free function so tooling
// without an engine can use it with an explicit language name.
func ParseSchemaSDL(b []byte, dbType string, blocklist []string) (*sdata.DBInfo, error) {
	return parseSchemaSDLWith(langadapter.DefaultLanguageName, b, dbType, blocklist)
}

func parseSchemaSDLWith(langName string, b []byte, dbType string, blocklist []string) (*sdata.DBInfo, error) {
	d, err := langadapter.Lookup(langName)
	if err != nil {
		return nil, err
	}
	sp, ok := d.(langadapter.SchemaParser)
	if !ok {
		return nil, fmt.Errorf("language %q does not parse schema definitions", langName)
	}
	return sp.ParseSchemaSDL(b, dbType, blocklist)
}
