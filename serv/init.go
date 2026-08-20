package serv

import (
	// "crypto/sha256"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/cache"
	"github.com/aegion-dynamic/graphjin-slim/serv/v3/database"
)

// initLogLevel initializes the log level
func initLogLevel(s *graphjinService) {
	switch s.conf.LogLevel {
	case "debug":
		s.logLevel = logLevelDebug
	case "error":
		s.logLevel = logLevelError
	case "warn":
		s.logLevel = logLevelWarn
	case "info":
		s.logLevel = logLevelInfo
	default:
		s.logLevel = logLevelNone
	}
}

// validateConf validates the configuration
func validateConf(s *graphjinService) error {
	var anonFound bool

	for _, r := range s.conf.Roles {
		if r.Name == "anon" {
			anonFound = true
		}
	}

	if !anonFound && s.conf.DefaultBlock {
		s.log.Warn("unauthenticated requests will be blocked. no role 'anon' defined")
		s.conf.AuthFailBlock = false
	}

	return nil
}

// initFS initializes the file system
func (s *graphjinService) initFS() error {
	basePath, err := s.basePath()
	if err != nil {
		return err
	}

	err = OptionSetFS(core.NewOsFS(basePath))(s)
	if err != nil {
		return err
	}
	return nil
}

// initConfig initializes the configuration
func (s *graphjinService) initConfig() error {
	c := s.conf
	c.dirty = true
	if err := normalizeConfigMode(c); err != nil {
		return err
	}

	// copy over db_type from database.type
	if c.DBType == "" {
		c.DBType = c.DB.Type
	}

	hp := strings.SplitN(s.conf.HostPort, ":", 2)

	if len(hp) == 2 {
		if s.conf.Host != "" {
			hp[0] = s.conf.Host
		}

		if s.conf.Port != "" {
			hp[1] = s.conf.Port
		}

		s.conf.hostPort = fmt.Sprintf("%s:%s", hp[0], hp[1])
	}

	if s.conf.hostPort == "" {
		s.conf.hostPort = defaultHP
	}

	return nil
}

// ErrGraphJinNotInitialized is returned when GraphJin core is not initialized
var ErrGraphJinNotInitialized = errors.New("GraphJin not initialized - no database configured")

// checkGraphJinInitialized returns an error if GraphJin core is not initialized
func (s *graphjinService) checkGraphJinInitialized() error {
	if s.gj == nil {
		return ErrGraphJinNotInitialized
	}
	return nil
}

// isDatabaseConfigured checks if a database connection is configured
func (s *graphjinService) isDatabaseConfigured() bool {
	// Check if connection string is provided
	if s.conf.DB.ConnString != "" {
		return true
	}
	if s.conf.DB.Path != "" {
		return true
	}
	// Check if host and dbname are provided (minimal required fields for auto-connect)
	if s.conf.DB.Host != "" && s.conf.DB.DBName != "" {
		return true
	}
	// Check if multi-database configs exist with actual connection info
	for _, dbConf := range s.conf.Core.Databases {
		if dbConf.ConnString != "" || dbConf.Host != "" || dbConf.Path != "" {
			return true
		}
	}
	return false
}

// initDB initializes database connections for all entries in conf.Core.Databases.
func (s *graphjinService) initDB() error {
	runtimeCore := cloneCoreConfig(s.conf.Core)
	if err := s.hydrateCoreConfigSecrets(&runtimeCore); err != nil {
		return err
	}
	if err := s.hydrateLegacyDatabaseSecrets(&s.conf.DB); err != nil {
		return err
	}
	s.runtimeCore = &runtimeCore

	if len(s.dbs) > 0 && !s.hasDatabaseConfigs() {
		return nil
	}

	// In dev mode, allow starting without a database configured
	if !s.conf.Serv.Production && !s.isDatabaseConfigured() {
		s.log.Warn("No databases configured. Use MCP to add a database configuration.")
		return nil
	}

	// In sources used, absence of SQL/CodeSQL connection details means there is
	// no legacy database to fall back to. Virtual/system sources get a small
	// host database in normalStart when needed.
	if s.conf.Core.IsSourcesUsed() && !s.hasDatabaseConfigs() {
		return nil
	}

	// If there are entries in conf.Core.Databases with connection info, use them.
	// Otherwise fall back to the legacy single-DB path via conf.DB.
	if s.hasDatabaseConfigs() {
		return s.initAllDBs()
	}

	// Legacy single-DB path: create one connection from conf.DB
	return s.initLegacyDB()
}

// hasDatabaseConfigs returns true if any entry in conf.Core.Databases
// has enough info to create a connection.
func (s *graphjinService) hasDatabaseConfigs() bool {
	for _, dbConf := range s.conf.Core.Databases {
		if dbConf.ConnString != "" || dbConf.Host != "" || dbConf.Path != "" {
			return true
		}
	}
	return false
}

// initAllDBs creates connections for every entry in conf.Core.Databases.
func (s *graphjinService) initAllDBs() error {
	dbNames := make([]string, 0, len(s.conf.Core.Databases))
	for name := range s.conf.Core.Databases {
		dbNames = append(dbNames, name)
	}
	sort.Strings(dbNames)
	for _, name := range dbNames {
		dbConf := s.conf.Core.Databases[name]
		runtimeDBConf := dbConf
		if s.runtimeCore != nil && s.runtimeCore.Databases != nil {
			if hydrated, ok := s.runtimeCore.Databases[name]; ok {
				runtimeDBConf = hydrated
			}
		}
		if _, ok := s.dbs[name]; ok {
			// runtime event removed
			continue
		}
		db, err := s.newDBFromDatabaseConfigInto(name, runtimeDBConf, s.runtimeCore)
		if err != nil {
			// runtime event removed
			if s.conf.Serv.Production {
				return fmt.Errorf("database %s: %s", name, redactRuntimeStringValue(err.Error()))
			}
			s.log.Warnf("Database '%s' connection failed: %s. Skipping.", name, redactRuntimeStringValue(err.Error()))
			continue
		}
		s.dbs[name] = db
		// runtime event removed
	}
	// Sync legacy conf.DB from first database for code that still reads it
	if len(s.dbs) > 0 {
		syncRuntimeDBFromDatabases(s.conf, s.runtimeCore)
	}
	return nil
}

// initLegacyDB creates a single connection from the legacy conf.DB fields.
func (s *graphjinService) initLegacyDB() error {
	if isCodeSQLType(s.conf.DB.Type) || isCodeSQLType(s.conf.DBType) {
		return fmt.Errorf("codesql databases are not supported in slim build")
	}

	var db *sql.DB
	var err error

	if s.conf.Serv.Production {
		db, err = newDB(s.conf, true, true, s.log, s.fs)
		if err != nil {
			// runtime event removed
			return fmt.Errorf("%s", redactRuntimeStringValue(err.Error()))
		}
	} else {
		db, err = newDBOnce(s.conf, true, true, s.log, s.fs)
		if err != nil {
			// runtime event removed
			s.log.Warnf("Database connection failed: %s. Server starting without database — use MCP to configure.", redactRuntimeStringValue(err.Error()))
			return nil
		}
	}

	// Store under the first Databases key (sorted for determinism)
	name := core.DefaultDBName
	if len(s.conf.Core.Databases) > 0 {
		names := make([]string, 0, len(s.conf.Core.Databases))
		for n := range s.conf.Core.Databases {
			names = append(names, n)
		}
		sort.Strings(names)
		name = names[0]
	}
	s.dbs[name] = db
	// runtime event removed
	return nil
}

// newDBFromDatabaseConfig creates a *sql.DB from a core.DatabaseConfig.
func (s *graphjinService) newDBFromDatabaseConfig(name string, dbConf core.DatabaseConfig) (*sql.DB, error) {
	return s.newDBFromDatabaseConfigInto(name, dbConf, &s.conf.Core)
}

func (s *graphjinService) newDBFromDatabaseConfigInto(name string, dbConf core.DatabaseConfig, runtimeCore *core.Config) (*sql.DB, error) {
	return database.OpenCore(context.Background(), name, dbConf)
}

// basePath returns the base path
func (s *graphjinService) basePath() (string, error) {
	if s.conf.ConfigPath == "" {
		if cp, err := os.Getwd(); err == nil {
			return filepath.Join(cp, "config"), nil
		} else {
			return "", err
		}
	}
	return s.conf.ConfigPath, nil
}

// initResponseCache initializes the response cache (Redis or in-memory)
func (s *graphjinService) initResponseCache() error {
	// Caching is enabled by default unless explicitly disabled
	if s.conf.Caching.Disable {
		s.log.Info("Response cache disabled")
		return nil
	}

	var err error
	s.cache, err = cache.New(s.conf.Caching, s.conf.Redis.URL, defaultMemoryCacheSize, s.log)
	if err != nil {
		s.log.Warnf("Failed to initialize response cache: %s", err)
		return nil
	}

	// Enable cache tracking in qcode compiler (injects __gj_id fields)
	s.conf.CacheTrackingEnabled = true

	return nil
}

// cloneCoreConfig creates a copy of a core.Config.
func cloneCoreConfig(c core.Config) core.Config {
	return c
}

// syncRuntimeDBFromDatabases syncs the legacy conf.DB from the first database.
func syncRuntimeDBFromDatabases(conf *Config, runtimeCore *core.Config) {}

// isCodeSQLType checks if the database type is codesql.
func isCodeSQLType(t string) bool {
	return false
}

const dbTypeCodeSQL = "codesql"

// normalizeServiceSources is a no-op in slim build.
func normalizeServiceSources(c *Config) error { return nil }
