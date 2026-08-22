// Package database contains service database configuration and adapters.
package database

import "time"

// Config contains connection and pool settings for the supported drivers.
type Config struct {
	ConnString    string `mapstructure:"connection_string" jsonschema:"title=Connection String"`
	Type          string `jsonschema:"title=Type,enum=postgres,enum=sqlite"`
	Host          string `jsonschema:"title=Host"`
	Port          uint16 `jsonschema:"title=Port"`
	DBName        string `jsonschema:"title=Database Name"`
	User          string `jsonschema:"title=User"`
	Password      string `jsonschema:"title=Password"`
	Schema        string `jsonschema:"title=Postgres Schema"`
	Path          string `jsonschema:"title=File Path (SQLite)"`
	EncryptionKey string `mapstructure:"encryption_key" jsonschema:"title=SQLCipher Encryption Key (optional)"`

	PoolSize        int           `mapstructure:"pool_size" jsonschema:"title=Connection Pool Size"`
	MaxConnections  int           `mapstructure:"max_connections" jsonschema:"title=Maximum Connections"`
	MaxConnIdleTime time.Duration `mapstructure:"max_connection_idle_time" jsonschema:"title=Connection Idle Time"`
	MaxConnLifeTime time.Duration `mapstructure:"max_connection_life_time" jsonschema:"title=Connection Life Time"`
	PingTimeout     time.Duration `mapstructure:"ping_timeout" jsonschema:"title=Healthcheck Ping Timeout"`

	EnableTLS  bool   `mapstructure:"enable_tls" jsonschema:"title=Enable TLS"`
	ServerName string `mapstructure:"server_name" jsonschema:"title=TLS Server Name"`
	ServerCert string `mapstructure:"server_cert" jsonschema:"title=Server Certificate"`

	// Settings captures engine-specific configuration keys that are not part
	// of the shared struct (engine-keyed yaml sections like `sqlite:`).
	Settings               map[string]any `mapstructure:",remain" json:"-"`
	ClientCert             string         `mapstructure:"client_cert" jsonschema:"title=Client Certificate"`
	ClientKey              string         `mapstructure:"client_key" jsonschema:"title=Client Key"`
	Encrypt                *bool          `mapstructure:"encrypt" jsonschema:"title=MSSQL Encrypt"`
	TrustServerCertificate *bool          `mapstructure:"trust_server_certificate" jsonschema:"title=MSSQL Trust Server Certificate"`
	PrivateKeyPath         string         `mapstructure:"private_key_path" jsonschema:"title=Private Key File Path (Snowflake)"`
	PrivateKeyPEM          string         `mapstructure:"private_key_pem" jsonschema:"title=Private Key PEM (Snowflake)"`
	KeyPassphrase          string         `mapstructure:"key_passphrase" jsonschema:"title=Key Passphrase (Snowflake)"`
	Consistency            string         `mapstructure:"consistency" jsonschema:"title=Cassandra Consistency Level"`
}
