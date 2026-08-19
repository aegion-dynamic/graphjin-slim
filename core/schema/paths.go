// Package schema owns schema artifacts, discovery, and snapshots.
package schema

import (
	"path"
	"strings"
)

const (
	SchemaDDLFile           = "db.ddl"
	LegacySchemaGraphQLFile = "db.graphql"
	SourceSchemaDDLDir      = "schema-ddl"
	LocalStateDir           = ".graphjin"
)

// SourceDDLPath returns the canonical source-local DDL path.
func SourceDDLPath(source string) string {
	return path.Join(SourceSchemaDDLDir, sanitizeName(source)+".ddl")
}

// RuntimeDDLPath returns the generated runtime-cache DDL path.
func RuntimeDDLPath(source string) string {
	return path.Join(LocalStateDir, SourceSchemaDDLDir, sanitizeName(source)+".ddl")
}

// RuntimeSnapshotPath returns the generated runtime schema snapshot path.
func RuntimeSnapshotPath(source string) string {
	return path.Join(LocalStateDir, SourceSchemaDDLDir, sanitizeName(source)+".schema.json")
}

// SanitizeName returns the filesystem-safe source name used by schema paths.
func SanitizeName(name string) string { return sanitizeName(name) }

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}
