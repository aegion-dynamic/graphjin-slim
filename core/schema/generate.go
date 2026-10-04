package schema

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/tabwriter"
	"text/template"
	"unicode"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/introspection"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/sdata"
)

const schemaTemplate = `# dbinfo:{{if .Type}}{{ .Type }}{{else}}postgres{{end}},{{- .Version }},{{- .Schema }}

{{ define "schema_directive"}}
{{- if and (ne .Schema "public") (ne .Schema "")}} @schema(name: {{ .Schema }}){{end}}
{{- end}}

{{- define "database_directive"}}
{{- if (ne .Database "")}} @database(name: {{ .Database }}){{end}}
{{- end}}

{{- define "cluster_directive"}}
{{- if .ClusteringKeys}} @cluster(columns: [{{range $i, $c := .ClusteringKeys}}{{if $i}}, {{end}}"{{$c}}"{{end}}]){{end}}
{{- end}}

{{- define "relation_directive"}}
{{- if (ne .FKeyTable "")}} @relation(type: {{ .FKeyTable }}
{{- if (ne .FKeyCol "")}}, field: {{ .FKeyCol }}{{end -}}
{{- if and (ne .FKeySchema "public") (ne .FKeySchema "")}}, schema: {{ .FKeySchema }}{{end -}})
{{- end}}
{{- end}}

{{- define "function_directive"}}
{{- " @function" }}
{{- if (ne .Type "")}}(return_type: {{ .Type | printf "%q" }}){{end}}
{{- end}}

{{- define "column_type"}}
{{- $var := .Type|dbtype }}
{{- $type := ((index $var 0)|basename|pascal) }}
{{- if .Array}}[{{ $type }}]{{else}}{{ $type }}{{end}}
{{- if .NotNull}}!{{end}}
{{- "\t" }}
{{- if ne (index $var 1) ""}} @type(args: {{ (index $var 1) | printf "%q" }}){{end}}
{{- if ne (.Type|dbtypeverbatim) ""}} @dbtype(value: {{ .Type|dbtypeverbatim | printf "%q" }}){{end}}
{{- template "relation_directive" .}}
{{- end}}

{{- define "column"}}
{{ "\t" }}
{{- .Name }}:
{{- "\t"}}
{{- template "column_type" .}}
{{- if .PrimaryKey}} @id{{end}}
{{- if .UniqueKey}} @unique{{end}}
{{- if .FullText}} @search{{end}}
{{- if .Blocked}} @blocked{{end}}
{{- end}}

{{- define "func_args"}}
{{ "\t" }}
{{- if ne .Name "" }}{{ .Name }}{{ else }}_arg{{ .ID }}{{ end }}:
{{- "\t"}}
{{- $var := .Type|dbtype }}
{{- ((index $var 0)|basename|pascal) }}
{{- if .Array}}[]{{end}}
{{- "\t"}}
{{- if ne (index $var 1) ""}} @type_args(args: {{ (index $var 1) | printf "%q" }}){{end}}
{{- if ne (.Type|dbtypeverbatim) ""}} @dbtype(value: {{ .Type|dbtypeverbatim | printf "%q" }}){{end}}
{{- end -}}

{{range .Tables -}}
type {{.Name}}
{{- template "database_directive" .}}
{{- template "schema_directive" .}}
{{- template "cluster_directive" .}} {
{{- range .Columns}}{{template "column" .}}{{end}}
}

{{end -}}

{{range .Functions -}}
type {{.Name}}
{{- template "schema_directive" .}}
{{- template "function_directive" .}} {
{{- range .Inputs}}{{template "func_args" .}}{{"\t"}}@input{{end}}
{{- range .Outputs}}{{template "func_args" .}}{{"\t"}}@output{{end}}
}

{{end -}}
`

// writeSchema writes the schema to the given writer
func WriteSchema(s *sdata.DBInfo, out io.Writer) (err error) {
	fn := template.FuncMap{
		"pascal":         toPascalCase,
		"basename":       baseName,
		"dbtype":         parseDBType,
		"dbtypeverbatim": dbtypeVerbatim,
	}

	tmpl, err := template.
		New("schema").
		Funcs(fn).
		Parse(schemaTemplate)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(out, 2, 2, 2, ' ', 0)
	err = tmpl.Execute(w, s)
	if err != nil {
		return err
	}
	return
}

// toPascalCase converts a string to pascal case
func toPascalCase(text string) string {
	var sb strings.Builder
	for _, v := range strings.Fields(text) {
		sb.WriteRune(unicode.ToUpper(rune(v[0])))
		sb.WriteString(v[1:])
	}
	return sb.String()
}

var dbTypeRe = regexp.MustCompile(`^([\w][\w. ]*?)\s*(\((.+)\))?(\[\])?$`)

// splitDbType splits a live column type like "application.event_status",
// "character varying(255)" or "text[]" into its base name and parenthesized
// suffix. The trailing array marker is dropped because the template renders
// arrays from the column's Array flag. It never fails; shapes it cannot
// split round-trip through the @dbtype directive instead.
func splitDbType(name string) (base, args string) {
	s := strings.TrimSpace(name)
	s = strings.TrimSuffix(s, "[]")
	if i := strings.Index(s, "("); i >= 0 && strings.HasSuffix(s, ")") {
		return strings.TrimSpace(s[:i]), s[i+1 : len(s)-1]
	}
	return s, ""
}

// baseName strips a schema qualifier ("application.event_status" ->
// "event_status"). Dots cannot appear in an SDL type token, so the token
// only ever carries the bare name; the qualifier is preserved via @dbtype.
func baseName(t string) string {
	if i := strings.LastIndex(t, "."); i >= 0 {
		return t[i+1:]
	}
	return t
}

// sdlToDbType inverts toPascalCase for SDL type tokens. It must stay in sync
// with graphql.pascalToSnakeSpace, which performs the same inversion when
// loading schema files (pinned by TestWriteSchemaRoundTrip).
func sdlToDbType(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte(' ')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// dbtypeVerbatim returns the exact live type when the SDL token cannot
// reproduce it (schema-qualified enums and other unlexable shapes), else "".
// The loader prefers it over the token, so save/load is always exact.
func dbtypeVerbatim(typ string) string {
	base, args := splitDbType(typ)
	norm := base
	if args != "" {
		norm += "(" + args + ")"
	}
	token := toPascalCase(baseName(base))
	if !isSDLName(token) {
		return typ
	}
	back := sdlToDbType(token)
	if args != "" {
		back += "(" + args + ")"
	}
	if back != typ {
		return typ
	}
	return ""
}

// isSDLName reports whether s is lexable as a type token in a schema file.
func isSDLName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// parseDBType parses the db type string
func parseDBType(name string) (res [2]string, err error) {
	if v := dbTypeRe.FindStringSubmatch(name); len(v) == 5 {
		res = [2]string{v[1], v[3]}
		return
	}
	base, args := splitDbType(name)
	if base == "" {
		err = fmt.Errorf("invalid db type: %s", name)
		return
	}
	res = [2]string{base, args}
	return
}

// GenerateSchema generates GraphJin DDL from database introspection.
func GenerateSchema(db *sql.DB, dbType string, blocklist []string) ([]byte, error) {
	dbinfo, err := introspection.GetDBInfo(context.Background(), db, dbType, blocklist)
	if err != nil {
		return nil, fmt.Errorf("failed to introspect database: %w", err)
	}

	var buf bytes.Buffer
	if err := WriteSchema(dbinfo, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
