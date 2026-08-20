package schema

import (
	"fmt"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

// DDLDialect defines how to generate DDL for a specific database
type DDLDialect interface {
	Name() string
	QuoteIdentifier(s string) string
	MapType(graphqlType string, notNull bool, primaryKey bool) string
	MapDefault(defaultVal string) string
	CreateTable(table sdata.DBTable) string
	AddColumn(tableName string, col sdata.DBColumn) string
	DropColumn(tableName, colName string) string
	DropTable(tableName string) string
	AddForeignKey(tableName string, col sdata.DBColumn) string
	CreateSearchIndex(tableName string, col sdata.DBColumn) string
	CreateUniqueIndex(tableName string, col sdata.DBColumn) string
	CreateIndex(tableName string, col sdata.DBColumn) string
	AlterClusteringKey(tableName string, keys []string) string
}

// SupportsSchemaDDL reports whether GraphJin supports live schema DDL sync for dbType.
func SupportsSchemaDDL(dbType string) bool {
	switch strings.ToLower(strings.TrimSpace(dbType)) {
	case "postgresql", "postgres", "sqlite":
		return true
	default:
		return false
	}
}

func getDDLDialect(dbType string) DDLDialect {
	switch strings.ToLower(strings.TrimSpace(dbType)) {
	case "postgresql", "postgres":
		return &postgresDialect{}
	case "sqlite":
		return &sqliteDialect{}
	default:
		return nil
	}
}

// PostgreSQL dialect
type postgresDialect struct{}

func (d *postgresDialect) Name() string { return "postgresql" }

func (d *postgresDialect) QuoteIdentifier(s string) string {
	return `"` + s + `"`
}

func (d *postgresDialect) MapType(graphqlType string, notNull bool, primaryKey bool) string {
	t := strings.ToLower(graphqlType)

	if primaryKey {
		switch t {
		case "int", "integer", "bigint", "big int":
			return "BIGSERIAL PRIMARY KEY"
		case "smallint", "small int":
			return "SMALLSERIAL PRIMARY KEY"
		default:
			return d.mapBaseType(t) + " PRIMARY KEY"
		}
	}

	baseType := d.mapBaseType(t)
	if notNull {
		return baseType + " NOT NULL"
	}
	return baseType
}

func (d *postgresDialect) mapBaseType(t string) string {
	// Handle type aliases with embedded sizes
	if baseType, size := parseTypeWithSize(t); size != "" {
		switch baseType {
		case "varchar":
			return fmt.Sprintf("VARCHAR(%s)", size)
		case "char":
			return fmt.Sprintf("CHAR(%s)", size)
		case "decimal", "numeric":
			return fmt.Sprintf("NUMERIC(%s)", size)
		}
	}

	switch t {
	case "int", "integer":
		return "INTEGER"
	case "bigint", "big int":
		return "BIGINT"
	case "smallint", "small int":
		return "SMALLINT"
	case "float", "real":
		return "REAL"
	case "double", "double precision":
		return "DOUBLE PRECISION"
	case "decimal", "numeric":
		return "NUMERIC"
	case "boolean", "bool":
		return "BOOLEAN"
	case "text", "string":
		return "TEXT"
	case "varchar", "character varying":
		return "VARCHAR(255)"
	case "char", "character":
		return "CHAR(1)"
	case "timestamp", "timestamp with time zone", "timestamptz":
		return "TIMESTAMPTZ"
	case "timestamp without time zone":
		return "TIMESTAMP"
	case "date":
		return "DATE"
	case "time", "time with time zone", "timetz":
		return "TIMETZ"
	case "time without time zone":
		return "TIME"
	case "interval":
		return "INTERVAL"
	case "json":
		return "JSON"
	case "jsonb":
		return "JSONB"
	case "uuid":
		return "UUID"
	case "bytea", "bytes":
		return "BYTEA"
	case "inet":
		return "INET"
	case "cidr":
		return "CIDR"
	case "macaddr":
		return "MACADDR"
	case "point":
		return "POINT"
	case "line":
		return "LINE"
	case "polygon":
		return "POLYGON"
	case "geometry":
		return "GEOMETRY"
	case "geography":
		return "GEOGRAPHY"
	case "money":
		return "MONEY"
	case "xml":
		return "XML"
	case "serial":
		return "SERIAL"
	case "bigserial", "big serial":
		return "BIGSERIAL"
	default:
		return "TEXT"
	}
}

func (d *postgresDialect) MapDefault(defaultVal string) string {
	return defaultVal
}

func (d *postgresDialect) CreateTable(table sdata.DBTable) string {
	var cols []string
	var constraints []string

	for _, col := range table.Columns {
		colDef := fmt.Sprintf("  %s %s",
			d.QuoteIdentifier(col.Name),
			d.MapType(col.Type, col.NotNull, col.PrimaryKey))
		if col.Default != "" {
			colDef += fmt.Sprintf(" DEFAULT %s", d.MapDefault(col.Default))
		}
		cols = append(cols, colDef)

		if col.FKeyTable != "" && col.FKeyCol != "" {
			fkName := fmt.Sprintf("fk_%s_%s", table.Name, col.Name)
			fkDef := fmt.Sprintf("  CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)",
				d.QuoteIdentifier(fkName),
				d.QuoteIdentifier(col.Name),
				d.QuoteIdentifier(col.FKeyTable),
				d.QuoteIdentifier(col.FKeyCol))
			if col.FKOnDelete != "" {
				fkDef += fmt.Sprintf(" ON DELETE %s", col.FKOnDelete)
			}
			if col.FKOnUpdate != "" {
				fkDef += fmt.Sprintf(" ON UPDATE %s", col.FKOnUpdate)
			}
			constraints = append(constraints, fkDef)
		}
	}

	tableParts := append(cols, constraints...)
	return fmt.Sprintf("CREATE TABLE %s (\n%s\n);",
		d.QuoteIdentifier(table.Name),
		strings.Join(tableParts, ",\n"))
}

func (d *postgresDialect) AddColumn(tableName string, col sdata.DBColumn) string {
	colDef := d.MapType(col.Type, col.NotNull, false)
	if col.Default != "" {
		colDef += fmt.Sprintf(" DEFAULT %s", d.MapDefault(col.Default))
	}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;",
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(col.Name),
		colDef)
}

func (d *postgresDialect) DropColumn(tableName, colName string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;",
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(colName))
}

func (d *postgresDialect) DropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE %s;", d.QuoteIdentifier(tableName))
}

func (d *postgresDialect) AddForeignKey(tableName string, col sdata.DBColumn) string {
	if col.FKeyTable == "" || col.FKeyCol == "" {
		return ""
	}
	fkName := fmt.Sprintf("fk_%s_%s", tableName, col.Name)
	sql := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)",
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(fkName),
		d.QuoteIdentifier(col.Name),
		d.QuoteIdentifier(col.FKeyTable),
		d.QuoteIdentifier(col.FKeyCol))
	if col.FKOnDelete != "" {
		sql += fmt.Sprintf(" ON DELETE %s", col.FKOnDelete)
	}
	if col.FKOnUpdate != "" {
		sql += fmt.Sprintf(" ON UPDATE %s", col.FKOnUpdate)
	}
	return sql + ";"
}

func (d *postgresDialect) CreateSearchIndex(tableName string, col sdata.DBColumn) string {
	idxName := fmt.Sprintf("idx_%s_%s_search", tableName, col.Name)
	return fmt.Sprintf("CREATE INDEX %s ON %s USING gin(to_tsvector('english', %s));",
		d.QuoteIdentifier(idxName),
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(col.Name))
}

func (d *postgresDialect) CreateUniqueIndex(tableName string, col sdata.DBColumn) string {
	idxName := fmt.Sprintf("idx_%s_%s_unique", tableName, col.Name)
	return fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s);",
		d.QuoteIdentifier(idxName),
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(col.Name))
}

func (d *postgresDialect) CreateIndex(tableName string, col sdata.DBColumn) string {
	idxName := col.IndexName
	if idxName == "" {
		idxName = fmt.Sprintf("idx_%s_%s", tableName, col.Name)
	}
	return fmt.Sprintf("CREATE INDEX %s ON %s (%s);",
		d.QuoteIdentifier(idxName),
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(col.Name))
}

func (d *postgresDialect) AlterClusteringKey(_ string, _ []string) string {
	return ""
}

type sqliteDialect struct{}

func (d *sqliteDialect) Name() string { return "sqlite" }

func (d *sqliteDialect) QuoteIdentifier(s string) string {
	return `"` + s + `"`
}

func (d *sqliteDialect) MapType(graphqlType string, notNull bool, primaryKey bool) string {
	t := strings.ToLower(graphqlType)

	if primaryKey {
		switch t {
		case "int", "integer", "bigint", "big int", "smallint", "small int":
			return "INTEGER PRIMARY KEY AUTOINCREMENT"
		default:
			return d.mapBaseType(t) + " PRIMARY KEY"
		}
	}

	baseType := d.mapBaseType(t)
	if notNull {
		return baseType + " NOT NULL"
	}
	return baseType
}

func (d *sqliteDialect) mapBaseType(t string) string {
	// Handle type aliases with embedded sizes (SQLite doesn't use sizes but we parse anyway)
	if baseType, _ := parseTypeWithSize(t); baseType != "" {
		switch baseType {
		case "varchar", "char", "decimal", "numeric":
			return "TEXT"
		}
	}

	switch t {
	case "int", "integer", "bigint", "big int", "smallint", "small int":
		return "INTEGER"
	case "float", "real", "double", "double precision", "decimal", "numeric":
		return "REAL"
	case "boolean", "bool":
		return "INTEGER"
	case "text", "string", "varchar", "character varying", "char", "character":
		return "TEXT"
	case "timestamp", "timestamp with time zone", "timestamptz", "timestamp without time zone":
		return "TEXT"
	case "date", "time", "time with time zone", "timetz", "time without time zone":
		return "TEXT"
	case "interval":
		return "TEXT"
	case "json", "jsonb":
		return "TEXT"
	case "uuid":
		return "TEXT"
	case "bytea", "bytes":
		return "BLOB"
	case "money":
		return "REAL"
	case "xml":
		return "TEXT"
	case "serial", "bigserial", "big serial":
		return "INTEGER"
	default:
		return "TEXT"
	}
}

func (d *sqliteDialect) MapDefault(defaultVal string) string {
	return defaultVal
}

func (d *sqliteDialect) CreateTable(table sdata.DBTable) string {
	var cols []string
	var constraints []string

	for _, col := range table.Columns {
		colDef := fmt.Sprintf("  %s %s",
			d.QuoteIdentifier(col.Name),
			d.MapType(col.Type, col.NotNull, col.PrimaryKey))
		if col.Default != "" {
			colDef += fmt.Sprintf(" DEFAULT %s", d.MapDefault(col.Default))
		}
		cols = append(cols, colDef)

		if col.FKeyTable != "" && col.FKeyCol != "" {
			fkDef := fmt.Sprintf("  FOREIGN KEY (%s) REFERENCES %s(%s)",
				d.QuoteIdentifier(col.Name),
				d.QuoteIdentifier(col.FKeyTable),
				d.QuoteIdentifier(col.FKeyCol))
			if col.FKOnDelete != "" {
				fkDef += fmt.Sprintf(" ON DELETE %s", col.FKOnDelete)
			}
			if col.FKOnUpdate != "" {
				fkDef += fmt.Sprintf(" ON UPDATE %s", col.FKOnUpdate)
			}
			constraints = append(constraints, fkDef)
		}
	}

	tableParts := append(cols, constraints...)
	return fmt.Sprintf("CREATE TABLE %s (\n%s\n);",
		d.QuoteIdentifier(table.Name),
		strings.Join(tableParts, ",\n"))
}

func (d *sqliteDialect) AddColumn(tableName string, col sdata.DBColumn) string {
	colDef := d.MapType(col.Type, col.NotNull, false)
	if col.Default != "" {
		colDef += fmt.Sprintf(" DEFAULT %s", d.MapDefault(col.Default))
	}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;",
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(col.Name),
		colDef)
}

func (d *sqliteDialect) DropColumn(tableName, colName string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;",
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(colName))
}

func (d *sqliteDialect) DropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE %s;", d.QuoteIdentifier(tableName))
}

func (d *sqliteDialect) AddForeignKey(tableName string, col sdata.DBColumn) string {
	return "" // SQLite doesn't support adding FK constraints after table creation
}

func (d *sqliteDialect) CreateSearchIndex(tableName string, col sdata.DBColumn) string {
	return "" // SQLite FTS5 requires virtual table setup
}

func (d *sqliteDialect) CreateUniqueIndex(tableName string, col sdata.DBColumn) string {
	idxName := fmt.Sprintf("idx_%s_%s_unique", tableName, col.Name)
	return fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s);",
		d.QuoteIdentifier(idxName),
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(col.Name))
}

func (d *sqliteDialect) CreateIndex(tableName string, col sdata.DBColumn) string {
	idxName := col.IndexName
	if idxName == "" {
		idxName = fmt.Sprintf("idx_%s_%s", tableName, col.Name)
	}
	return fmt.Sprintf("CREATE INDEX %s ON %s (%s);",
		d.QuoteIdentifier(idxName),
		d.QuoteIdentifier(tableName),
		d.QuoteIdentifier(col.Name))
}

func (d *sqliteDialect) AlterClusteringKey(_ string, _ []string) string {
	return ""
}

func parseTypeWithSize(typeName string) (baseType string, size string) {
	typeName = strings.ToLower(typeName)

	// Check for common patterns
	patterns := []struct {
		prefix string
		base   string
	}{
		{"varchar", "varchar"},
		{"char", "char"},
		{"decimal", "decimal"},
		{"numeric", "numeric"},
	}

	for _, p := range patterns {
		if strings.HasPrefix(typeName, p.prefix) {
			suffix := typeName[len(p.prefix):]
			if suffix == "" {
				return "", ""
			}
			// Strip parentheses if present (from @type(args: "7,2") -> "numeric(7,2)")
			suffix = strings.TrimPrefix(suffix, "(")
			suffix = strings.TrimSuffix(suffix, ")")
			// Convert underscore to comma for decimal types (e.g., "10_2" -> "10,2")
			size = strings.ReplaceAll(suffix, "_", ",")
			return p.base, size
		}
	}

	return "", ""
}
