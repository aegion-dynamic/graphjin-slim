package core

import (
	"strings"
	"testing"
)

func TestValidateDBType(t *testing.T) {
	tests := []struct {
		name    string
		dbType  string
		wantErr bool
	}{
		{"empty string defaults to postgres", "", false},
		{"postgres is valid", "postgres", false},
		{"sqlite is valid", "sqlite", false},
		{"case insensitive", "PostgreS", false},
		{"codesql rejected", "codesql", true},
		{"mysql rejected", "mysql", true},
		{"invalid type", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDBType(tt.dbType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDBType(%q) error = %v, wantErr %v", tt.dbType, err, tt.wantErr)
			}
		})
	}
}

func TestValidateMultiDBType(t *testing.T) {
	tests := []struct {
		name    string
		dbType  string
		wantErr bool
	}{
		{"empty string defaults to postgres", "", false},
		{"postgres is valid", "postgres", false},
		{"sqlite is valid", "sqlite", false},
		{"case insensitive", "PostgreS", false},
		{"invalid type", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMultiDBType(tt.dbType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMultiDBType(%q) error = %v, wantErr %v", tt.dbType, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		name           string
		mode           string
		production     bool
		wantMode       string
		wantProduction bool
		wantErr        bool
	}{
		{name: "empty dev", wantMode: "dev"},
		{name: "empty prod from production", production: true, wantMode: "prod", wantProduction: true},
		{name: "dev", mode: "dev", production: true, wantMode: "dev", wantProduction: true},
		{name: "prod", mode: "prod", wantMode: "prod"},
		{name: "agentic rejected", mode: "agentic", wantErr: true},
		{name: "invalid", mode: "secure-ish", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := Config{Mode: tt.mode, Production: tt.production}
			err := conf.NormalizeMode()
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeMode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if conf.Mode != tt.wantMode || conf.Production != tt.wantProduction {
				t.Fatalf("NormalizeMode() = mode %q production %v, want mode %q production %v",
					conf.Mode, conf.Production, tt.wantMode, tt.wantProduction)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty config is valid",
			config:  Config{},
			wantErr: false,
		},
		{
			name: "postgres source is valid",
			config: Config{Sources: []SourceConfig{
				{Name: "app", Kind: "database", Type: "postgres", Default: true},
			}},
			wantErr: false,
		},
		{
			name:    "sources rejects code kind",
			config:  Config{Sources: []SourceConfig{{Name: "repo", Kind: "code", Path: "."}}},
			wantErr: true,
			errMsg:  "CodeSQL is not supported",
		},
		{
			name:    "sources rejects api kind",
			config:  Config{Sources: []SourceConfig{{Name: "api", Kind: "api"}}},
			wantErr: true,
			errMsg:  "OpenAPI remote sources are not supported",
		},
		{
			name:    "sources rejects file kind",
			config:  Config{Sources: []SourceConfig{{Name: "avatars", Kind: "file", Path: "."}}},
			wantErr: true,
			errMsg:  "filesystem virtual tables are not supported",
		},
		{
			name: "source names are unique case insensitively",
			config: Config{Sources: []SourceConfig{
				{Name: "App", Kind: "database", Type: "postgres"},
				{Name: "app", Kind: "database", Type: "postgres"},
			}},
			wantErr: true,
			errMsg:  "duplicate source name",
		},
		{
			name: "former internal names are valid external source names",
			config: Config{Sources: []SourceConfig{
				{Name: "graphjin", Kind: "database", Type: "postgres"},
				{Name: "workflows", Kind: "database", Type: "sqlite", Path: "."},
			}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Config.Validate() error = %v, should contain %q", err, tt.errMsg)
			}
		})
	}
}

func TestNormalizeModeFailsClosedInSourceMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		conf *Config
		want string
	}{
		{name: "source mode, no mode, not production -> prod (fail closed)",
			conf: &Config{Sources: []SourceConfig{{Name: "app", Kind: "database", Type: "postgres", Default: true}}}, want: "prod"},
		{name: "legacy mode, no mode, not production -> dev (unchanged)",
			conf: &Config{}, want: "dev"},
		{name: "explicit dev is preserved in source mode",
			conf: &Config{Mode: "dev", Sources: []SourceConfig{{Name: "app", Kind: "database", Type: "postgres", Default: true}}}, want: "dev"},
		{name: "production flag still implies prod",
			conf: &Config{Production: true}, want: "prod"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.conf.NormalizeMode(); err != nil {
				t.Fatalf("NormalizeMode: %v", err)
			}
			if tc.conf.Mode != tc.want {
				t.Fatalf("mode = %q, want %q", tc.conf.Mode, tc.want)
			}
		})
	}

	t.Run("explicit agentic is rejected", func(t *testing.T) {
		conf := &Config{Mode: "agentic", Sources: []SourceConfig{{Name: "app", Kind: "database", Type: "postgres", Default: true}}}
		if err := conf.NormalizeMode(); err == nil {
			t.Fatal("NormalizeMode(agentic) expected error")
		}
	})
}

func TestNormalizeSourcesMapsDatabaseSources(t *testing.T) {
	conf := &Config{
		Sources: []SourceConfig{
			{Name: "app", Kind: "database", Type: "postgres", Default: true},
			{Name: "analytics", Kind: "database", Type: "sqlite", Path: "a.db", ReadOnly: true},
		},
		Tables: []Table{
			{Name: "users", Source: "app"},
			{Name: "events", Source: "analytics"},
		},
	}
	if err := conf.NormalizeSources(); err != nil {
		t.Fatalf("NormalizeSources: %v", err)
	}
	if conf.Databases["app"].Type != "postgres" || conf.Databases["analytics"].Type != "sqlite" {
		t.Fatalf("unexpected database normalization: %+v", conf.Databases)
	}
	if conf.Tables[0].Database != "app" || conf.Tables[1].Database != "analytics" {
		t.Fatalf("unexpected table database mapping: %+v", conf.Tables)
	}
	if !conf.Tables[1].ReadOnly {
		t.Fatalf("source read_only was not applied to tables: %+v", conf.Tables)
	}
}

func TestRenormalizeSourcesRebuildsGeneratedRuntimeFields(t *testing.T) {
	conf := &Config{
		Sources: []SourceConfig{
			{Name: "app", Kind: "database", Type: "sqlite", Path: "old.sqlite3", Default: true},
		},
		Tables: []Table{{Name: "users", Source: "app"}},
	}
	if err := conf.NormalizeSources(); err != nil {
		t.Fatalf("NormalizeSources: %v", err)
	}
	conf.Sources = []SourceConfig{
		{Name: "app", Kind: "database", Type: "sqlite", Path: "new.sqlite3", Default: true},
	}
	if err := conf.RenormalizeSources(); err != nil {
		t.Fatalf("RenormalizeSources: %v", err)
	}
	if conf.Databases["app"].Path != "new.sqlite3" {
		t.Fatalf("expected rebuilt path, got %+v", conf.Databases["app"])
	}
}
