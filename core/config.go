package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/qcode"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/internal/sdata"
)

const (
	ModeDev       = "dev"
	ModeProd      = "prod"
	DefaultDBName = "default"
)

var (
	SupportedDBTypes      = []string{"postgres", "sqlite"}
	SupportedMultiDBTypes = []string{"postgres", "sqlite"}
)

func CanonicalMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "":
		return "", nil
	case "dev", "development":
		return ModeDev, nil
	case "prod", "production":
		return ModeProd, nil
	default:
		return "", fmt.Errorf("unsupported mode %q: supported modes are dev, prod", mode)
	}
}

func (c *Config) NormalizeMode() error {
	if c == nil {
		return nil
	}
	mode, err := CanonicalMode(c.Mode)
	if err != nil {
		return err
	}
	if mode == "" {
		switch {
		case c.Production:
			mode = ModeProd
		case c.IsSourcesUsed():
			// Fail closed (security, audit F1): in source mode an unspecified
			// deployment mode must NOT silently fall back to dev. Dev makes every
			// gj_* system root public and mounts the dev surface; defaulting
			// to it on a missing `mode` is fail-open. Require an explicit
			// dev/dev selection (GO_ENV, dev.yml/dev.yml, or `mode:`);
			// anything ambiguous resolves to the locked-down prod posture.
			mode = ModeProd
		default:
			// Legacy (non-source) configs keep the long-standing dev default so
			// existing local development without `sources:` is unaffected.
			mode = ModeDev
		}
	}
	c.Mode = mode
	return nil
}

func ValidateDBType(dbType string) error {
	if dbType == "" {
		return nil // Empty defaults to postgres, which is valid
	}
	for _, t := range SupportedDBTypes {
		if strings.EqualFold(dbType, t) {
			return nil
		}
	}
	return fmt.Errorf("unsupported database type %q: supported types are %s",
		dbType, strings.Join(SupportedDBTypes, ", "))
}

func ValidateMultiDBType(dbType string) error {
	if dbType == "" {
		return nil // Empty defaults to postgres, which is valid
	}
	for _, t := range SupportedMultiDBTypes {
		if strings.EqualFold(dbType, t) {
			return nil
		}
	}
	return fmt.Errorf("unsupported database type %q: supported types are %s",
		dbType, strings.Join(SupportedMultiDBTypes, ", "))
}

func (c *Config) Validate() error {
	if c == nil {
		return nil
	}
	if err := c.NormalizeMode(); err != nil {
		return err
	}
	if err := ValidateDBType(c.DBType); err != nil {
		return err
	}
	for name, db := range c.Databases {
		if err := ValidateMultiDBType(db.Type); err != nil {
			return fmt.Errorf("databases[%q]: %w", name, err)
		}
	}
	if len(c.Resolvers) != 0 {
		return fmt.Errorf("resolvers are not supported in slim build")
	}
	return nil
}

func (c *Config) ValidateIsSourcesUsed() error { return nil }
func (c *Config) NormalizeSources() error {
	if c == nil {
		return nil
	}
	// No source-mode product surface; relationships still apply.
	return c.applyRelationshipOverlays()
}
func (c *Config) IsSourcesUsed() bool { return false }

func (c *Config) EffectiveIdentityConfig() IdentityConfig { return IdentityConfig{} }

func CanonicalSourceKind(kind string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(kind))
	switch k {
	case "database", "sql", "":
		return "database", nil
	default:
		return "", fmt.Errorf("unsupported kind %q (supported: database)", kind)
	}
}

func (c *Config) defaultDatabaseName() string {
	if c == nil {
		return ""
	}
	if _, ok := c.Databases[DefaultDBName]; ok {
		return DefaultDBName
	}
	names := make([]string, 0, len(c.Databases))
	for name := range c.Databases {
		names = append(names, name)
	}
	if len(names) == 0 {
		return DefaultDBName
	}
	sort.Strings(names)
	return names[0]
}

func (c *Config) NormalizeDatabases() {
	if c == nil {
		return
	}
	if c.Databases == nil {
		c.Databases = make(map[string]DatabaseConfig)
	}
	defaultName := c.defaultDatabaseName()
	if defaultName == "" {
		defaultName = DefaultDBName
	}
	if _, ok := c.Databases[defaultName]; !ok {
		c.Databases[defaultName] = DatabaseConfig{Type: c.DBType}
	}
	defConf := c.Databases[defaultName]
	if defConf.Type == "" {
		defConf.Type = c.DBType
	}
	if defConf.Type == "" {
		defConf.Type = "postgres"
	}
	c.Databases[defaultName] = defConf
}

func (c *Config) EffectiveAnalyticsMode(database string) bool {
	if c == nil {
		return false
	}
	if db, ok := c.Databases[database]; ok && db.AnalyticsMode {
		return true
	}
	return c.AnalyticsMode
}

func splitRelationshipEndpoint(endpoint string) (string, string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", "", fmt.Errorf("must not be empty")
	}
	idx := strings.LastIndex(endpoint, ".")
	if idx <= 0 || idx == len(endpoint)-1 {
		return "", "", fmt.Errorf("must be table.column, schema.table.column, or source:schema.table.column")
	}
	return endpoint[:idx], endpoint[idx+1:], nil
}

func splitTablePath(path string) (string, string, string) {
	db := ""
	if idx := strings.IndexByte(path, ':'); idx != -1 {
		db = path[:idx]
		path = path[idx+1:]
	}
	parts := strings.Split(path, ".")
	if len(parts) >= 2 {
		return db, strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
	}
	return db, "", path
}

func (c *Config) applyRelationshipOverlays() error {
	for i, rel := range c.Relationships {
		fromTable, fromColumn, err := splitRelationshipEndpoint(rel.From)
		if err != nil {
			return fmt.Errorf("relationships[%d].from: %w", i, err)
		}
		if _, _, err := splitRelationshipEndpoint(rel.To); err != nil {
			return fmt.Errorf("relationships[%d].to: %w", i, err)
		}
		idx := c.findTableIndex(fromTable)
		if idx == -1 {
			return fmt.Errorf("relationships[%d]: from table %q is not configured in tables", i, fromTable)
		}
		table := &c.Tables[idx]
		found := false
		for ci := range table.Columns {
			if table.Columns[ci].Name == fromColumn {
				table.Columns[ci].ForeignKey = rel.To
				found = true
				break
			}
		}
		if !found {
			table.Columns = append(table.Columns, Column{Name: fromColumn, ForeignKey: rel.To})
		}
	}
	return nil
}

func (c *Config) findTableIndex(path string) int {
	db, schema, table := splitTablePath(path)
	for i := range c.Tables {
		t := c.Tables[i]
		if db != "" && t.Database != db && t.Source != db {
			continue
		}
		if schema != "" && t.Schema != "" && t.Schema != schema {
			continue
		}
		if t.Name == table || t.Table == table {
			return i
		}
	}
	return -1
}

func (c *Config) AddRoleTable(role, table string, conf interface{}) error { return errRolesDisabled }

func (c *Config) RemoveRoleTable(role, table string) error { return errRolesDisabled }

// Config is the slim GraphJin compiler configuration (Postgres/SQLite + multi-DB).
type Config struct {
	SecretKey            string               `mapstructure:"secret_key" json:"secret_key" yaml:"secret_key"`
	DisableAllowList     bool                 `mapstructure:"disable_allow_list" json:"disable_allow_list" yaml:"disable_allow_list"`
	EnableSchema         bool                 `mapstructure:"enable_schema" json:"enable_schema" yaml:"enable_schema"`
	EnableIntrospection  bool                 `mapstructure:"enable_introspection" json:"enable_introspection" yaml:"enable_introspection"`
	DefaultBlock         bool                 `mapstructure:"default_block" json:"default_block" yaml:"default_block"`
	Vars                 map[string]string    `mapstructure:"variables" json:"variables" yaml:"variables"`
	HeaderVars           map[string]string    `mapstructure:"header_variables" json:"header_variables" yaml:"header_variables"`
	Blocklist            []string             `jsonschema:"title=Block List"`
	Resolvers            []ResolverConfig     `jsonschema:"-"`
	Tables               []Table              `jsonschema:"title=Tables"`
	Relationships        []RelationshipConfig `mapstructure:"relationships" json:"relationships" yaml:"relationships"`
	Functions            []Function           `jsonschema:"title=Functions"`
	DBType               string               `mapstructure:"db_type" json:"db_type" yaml:"db_type" jsonschema:"title=Database Type,enum=postgres,enum=sqlite"`
	Debug                bool
	LogVars              bool `mapstructure:"log_vars" json:"log_vars" yaml:"log_vars"`
	DefaultLimit         int  `mapstructure:"default_limit" json:"default_limit" yaml:"default_limit"`
	AnalyticsMode        bool `mapstructure:"analytics_mode" json:"analytics_mode" yaml:"analytics_mode"`
	DisableAgg           bool `mapstructure:"disable_agg_functions" json:"disable_agg_functions" yaml:"disable_agg_functions"`
	DisableFuncs         bool `mapstructure:"disable_functions" json:"disable_functions" yaml:"disable_functions"`
	EnableCamelcase      bool `mapstructure:"enable_camelcase" json:"enable_camelcase" yaml:"enable_camelcase"`
	Production           bool
	Mode                 string                    `mapstructure:"mode" json:"mode" yaml:"mode" jsonschema:"title=Mode,enum=dev,enum=prod"`
	DBSchemaPollDuration time.Duration             `mapstructure:"db_schema_poll_duration" json:"db_schema_poll_duration" yaml:"db_schema_poll_duration"`
	DisableProdSecurity  bool                      `mapstructure:"disable_production_security" json:"disable_production_security" yaml:"disable_production_security"`
	Databases            map[string]DatabaseConfig `mapstructure:"databases" json:"databases" yaml:"databases"`

	// Internal
	CacheTrackingEnabled bool        `mapstructure:"-" json:"-" yaml:"-" jsonschema:"-"`
	FS                   interface{} `mapstructure:"-" json:"-" yaml:"-" jsonschema:"-"`
}

type DatabaseConfig struct {
	Type            string        `mapstructure:"type" json:"type" yaml:"type" jsonschema:"title=Database Type,enum=postgres,enum=sqlite"`
	ConnString      string        `mapstructure:"connection_string" json:"connection_string" yaml:"connection_string"`
	Host            string        `mapstructure:"host" json:"host" yaml:"host"`
	Port            uint16        `mapstructure:"port" json:"port" yaml:"port"`
	User            string        `mapstructure:"user" json:"user" yaml:"user"`
	Password        string        `mapstructure:"password" json:"password" yaml:"password"`
	DBName          string        `mapstructure:"db_name" json:"db_name" yaml:"db_name"`
	Schema          string        `mapstructure:"schema" json:"schema" yaml:"schema"`
	Path            string        `mapstructure:"path" json:"path" yaml:"path"`
	ReadOnly        bool          `mapstructure:"read_only" json:"read_only" yaml:"read_only"`
	AnalyticsMode   bool          `mapstructure:"analytics_mode" json:"analytics_mode" yaml:"analytics_mode"`
	PoolSize        int           `mapstructure:"pool_size" json:"pool_size" yaml:"pool_size"`
	MaxConnections  int           `mapstructure:"max_connections" json:"max_connections" yaml:"max_connections"`
	MaxConnIdleTime time.Duration `mapstructure:"max_conn_idle_time" json:"max_conn_idle_time" yaml:"max_conn_idle_time"`
	MaxConnLifeTime time.Duration `mapstructure:"max_conn_life_time" json:"max_conn_life_time" yaml:"max_conn_life_time"`
	PingTimeout     time.Duration `mapstructure:"ping_timeout" json:"ping_timeout" yaml:"ping_timeout"`
	// EnableCamelcase / DisableAgg can also be global on Config
}

type Table struct {
	Name      string
	Schema    string
	Table     string // Inherits Table
	Type      string
	Generated bool   `mapstructure:"-" json:"-" yaml:"-" jsonschema:"-"`
	Source    string `mapstructure:"source" json:"source" yaml:"source"`
	Database  string `mapstructure:"database" json:"database" yaml:"database"`
	ReadOnly  bool   `mapstructure:"read_only" json:"read_only" yaml:"read_only"`
	Blocklist []string
	Columns   []Column
	OrderBy   map[string][]string `mapstructure:"order_by" json:"order_by" yaml:"order_by"`
}

type Column struct {
	Name       string
	Type       string `jsonschema:"example=integer,example=text"`
	Primary    bool
	Array      bool
	FullText   bool   `mapstructure:"full_text" json:"full_text" yaml:"full_text" jsonschema:"title=Full Text Search"`
	ForeignKey string `mapstructure:"related_to" json:"related_to" yaml:"related_to" jsonschema:"title=Related To,example=other_table.id_column,example=users.id"`
}

type Function struct {
	Name       string
	Schema     string
	ReturnType string `mapstructure:"return_type" json:"return_type" yaml:"return_type" jsonschema:"title=Return Type,example=boolean,example=record"`
}

type Role struct {
	Name    string
	Comment string
	Match   string      `jsonschema:"title=Related To,example=other_table.id_column,example=users.id"`
	Tables  []RoleTable `jsonschema:"title=Table Configuration for Role"`
	tm      map[string]*RoleTable
}

type RoleTable struct {
	Name      string
	Schema    string
	Database  string `mapstructure:"database" json:"database" yaml:"database" jsonschema:"title=Database"`
	ReadOnly  bool   `mapstructure:"read_only" json:"read_only" yaml:"read_only" jsonschema:"title=Read Only"`
	Generated bool   `mapstructure:"-" json:"-" yaml:"-" jsonschema:"-"`

	Query  *Query
	Insert *Insert
	Update *Update
	Upsert *Upsert
	Delete *Delete
}

type Query struct {
	Limit int
	// Use filters to enforce table wide things like { disabled: false } where you never want disabled users to be shown.
	Filters          []string
	Columns          []string
	DisableFunctions bool `mapstructure:"disable_functions" json:"disable_functions" yaml:"disable_functions"`
	Block            bool
}

type Insert struct {
	Filters []string
	Columns []string
	Presets map[string]string
	Block   bool
}

type Update struct {
	Filters []string
	Columns []string
	Presets map[string]string
	Block   bool
}

type Upsert struct {
	Filters []string
	Columns []string
	Presets map[string]string
	Block   bool
}

type Delete struct {
	Filters []string
	Columns []string
	Block   bool
}

type RelationshipConfig struct {
	From string `mapstructure:"from" json:"from" yaml:"from" jsonschema:"title=From"`
	To   string `mapstructure:"to" json:"to" yaml:"to" jsonschema:"title=To"`
	As   string `mapstructure:"as" json:"as,omitempty" yaml:"as,omitempty" jsonschema:"title=Alias"`
}

type IdentityConfig struct {
	UserIDClaim    string   `mapstructure:"user_id_claim" json:"user_id_claim" yaml:"user_id_claim"`
	RoleClaims     []string `mapstructure:"role_claims" json:"role_claims" yaml:"role_claims"`
	NamespaceClaim string   `mapstructure:"namespace_claim" json:"namespace_claim" yaml:"namespace_claim"`
	AdminRoles     []string `mapstructure:"admin_roles" json:"admin_roles" yaml:"admin_roles"`
	Query          string   `mapstructure:"query" json:"query" yaml:"query"`
}

func (c IdentityConfig) clone() IdentityConfig {
	out := c
	if c.RoleClaims != nil {
		out.RoleClaims = append([]string(nil), c.RoleClaims...)
	}
	if c.AdminRoles != nil {
		out.AdminRoles = append([]string(nil), c.AdminRoles...)
	}
	return out
}

type Resolver interface {
	Resolve(context.Context, ResolverReq) ([]byte, error)
}

type ResolverProps map[string]interface{}

type ResolverConfig struct {
	Name          string
	Type          string
	Schema        string
	Table         string
	Column        string
	StripPath     string        `mapstructure:"strip_path" json:"strip_path" yaml:"strip_path"`
	Props         ResolverProps `mapstructure:",remain"`
	remoteColumns []sdata.DBColumn
}

type ResolverReq struct {
	ID   string
	Sel  *qcode.Select
	Log  *log.Logger
	Vars map[string]json.RawMessage
	*RequestConfig
}

func isAgenticMode(c *Config) bool { return false }

func (c *Config) clone() *Config {
	if c == nil {
		return nil
	}
	out := *c
	if c.Vars != nil {
		out.Vars = make(map[string]string, len(c.Vars))
		for k, v := range c.Vars {
			out.Vars[k] = v
		}
	}
	if c.HeaderVars != nil {
		out.HeaderVars = make(map[string]string, len(c.HeaderVars))
		for k, v := range c.HeaderVars {
			out.HeaderVars[k] = v
		}
	}
	if c.Blocklist != nil {
		out.Blocklist = append([]string(nil), c.Blocklist...)
	}
	if c.Tables != nil {
		out.Tables = append([]Table(nil), c.Tables...)
	}
	if c.Relationships != nil {
		out.Relationships = append([]RelationshipConfig(nil), c.Relationships...)
	}
	if c.Functions != nil {
		out.Functions = append([]Function(nil), c.Functions...)
	}
	if c.Databases != nil {
		out.Databases = make(map[string]DatabaseConfig, len(c.Databases))
		for k, v := range c.Databases {
			out.Databases[k] = v
		}
	}
	out.Resolvers = nil
	return &out
}
