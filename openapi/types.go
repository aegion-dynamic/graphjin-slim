// Package openapi generates OpenAPI 3.0 specifications for a GraphJin
// service's REST endpoints from its saved named queries and discovered
// database schemas.
//
// The generator is pure: it consumes a plain snapshot (core.OpenAPIInputs)
// produced by the engine and renders a specification document. It never
// touches the database or the network.
package openapi

// Document is an OpenAPI 3.0 specification document.
type Document struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Servers    []Server              `json:"servers,omitempty"`
	Paths      map[string]PathItem   `json:"paths"`
	Components *Components           `json:"components,omitempty"`
	Security   []map[string][]string `json:"security,omitempty"`
}

// Info describes the API metadata.
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// Server describes a host serving the API.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// PathItem holds the operations available on a single path.
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
}

// Operation describes a single API operation on a path.
type Operation struct {
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
	Tags        []string            `json:"tags,omitempty"`
}

// Parameter describes a single operation parameter.
type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"` // "query", "header", "path", "cookie"
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Schema      Schema `json:"schema"`
}

// RequestBody describes a request body.
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Content     map[string]MediaType `json:"content"`
	Required    bool                 `json:"required,omitempty"`
}

// Response describes a single response from an API operation.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// MediaType provides schema and examples for a media type.
type MediaType struct {
	Schema Schema `json:"schema"`
}

// Schema is a JSON-Schema-flavoured OpenAPI schema object.
type Schema struct {
	Type                 string            `json:"type,omitempty"`
	Format               string            `json:"format,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	Required             []string          `json:"required,omitempty"`
	AdditionalProperties interface{}       `json:"additionalProperties,omitempty"`
	OneOf                []Schema          `json:"oneOf,omitempty"`
	Description          string            `json:"description,omitempty"`
	Example              interface{}       `json:"example,omitempty"`
	Ref                  string            `json:"$ref,omitempty"`
}

// Components holds reusable objects for the specification.
type Components struct {
	Schemas map[string]Schema `json:"schemas,omitempty"`
}
