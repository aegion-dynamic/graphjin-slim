// Package sourcecap provides minimal source-kind constants for slim config.
package sourcecap

import (
	"fmt"
	"strings"
)

const (
	ModeDev  = "dev"
	ModeProd = "prod"

	KindDatabase = "database"
)

// CanonicalKind normalizes sources[].kind. Slim only supports database.
func CanonicalKind(kind string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(kind))
	switch k {
	case KindDatabase, "sql", "":
		return KindDatabase, nil
	case "code", "codesql":
		return "", fmt.Errorf("unsupported kind %q; CodeSQL is not supported", kind)
	case "api", "openapi":
		return "", fmt.Errorf("unsupported kind %q; OpenAPI remote sources are not supported", kind)
	case "file", "filesystem", "files":
		return "", fmt.Errorf("unsupported kind %q; filesystem virtual tables are not supported", kind)
	default:
		return "", fmt.Errorf("unsupported kind %q (supported: database)", kind)
	}
}

// Lookup always fails — capability tables removed in slim.
func Lookup(kind, key string) (struct{}, bool) { return struct{}{}, false }

// ValidKeyList is empty in slim.
func ValidKeyList(kind string) string { return "" }
