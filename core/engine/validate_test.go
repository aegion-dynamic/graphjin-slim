package engine

import (
	"strings"
	"testing"
)

func TestValidateDBType(t *testing.T) {
	for _, tt := range []struct {
		dbType  string
		wantErr bool
	}{
		{"", false},
		{"postgres", false},
		{"sqlite", false},
		{"mysql", true},
		{"codesql", true},
	} {
		err := ValidateDBType(tt.dbType)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateDBType(%q) err=%v wantErr=%v", tt.dbType, err, tt.wantErr)
		}
	}
}

func TestNormalizeMode(t *testing.T) {
	for _, tt := range []struct {
		mode    string
		prod    bool
		want    string
		wantErr bool
	}{
		{"", false, "dev", false},
		{"", true, "prod", false},
		{"dev", false, "dev", false},
		{"prod", false, "prod", false},
		{"agentic", false, "", true},
	} {
		c := Config{Mode: tt.mode, Production: tt.prod}
		err := c.NormalizeMode()
		if (err != nil) != tt.wantErr {
			t.Fatalf("mode %q: %v", tt.mode, err)
		}
		if !tt.wantErr && c.Mode != tt.want {
			t.Fatalf("mode=%q want %q", c.Mode, tt.want)
		}
	}
}

func TestConfigValidateRejectsResolvers(t *testing.T) {
	c := Config{Resolvers: []ResolverConfig{{Name: "x"}}}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "resolvers") {
		t.Fatalf("got %v", err)
	}
}

func TestNormalizeDatabasesBasic(t *testing.T) {
	c := Config{DBType: "postgres"}
	c.NormalizeDatabases()
	if _, ok := c.Databases[DefaultDBName]; !ok {
		t.Fatal("expected default database entry")
	}
}
