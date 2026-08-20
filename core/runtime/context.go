package runtime

// ContextKey is used for values stored on request contexts.
type ContextKey int

// Context keys shared across core packages.
const (
	// UserIDProviderKey is the authentication provider name (google, github, ...).
	UserIDProviderKey ContextKey = iota

	// UserIDRawKey is the raw user id (jwt sub) value.
	UserIDRawKey

	// UserIDKey is the user id value for authenticated users.
	UserIDKey

	// UserRoleKey is a pre-defined user role.
	UserRoleKey

	// IdentityVarsKey carries trusted request-wide identity variables such as
	// account_id that may be referenced by generated source-mode filters.
	IdentityVarsKey

	// IdentityRolesKey carries candidate roles extracted from the verified
	// request identity before roles_query / match fallback.
	IdentityRolesKey
)
