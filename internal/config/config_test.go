package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	settings := LoadFrom(func(string) string { return "" })

	if settings.Neo4j.URI != "bolt://localhost:7687" {
		t.Fatalf("URI = %q", settings.Neo4j.URI)
	}
	if settings.Neo4j.Username != "neo4j" {
		t.Fatalf("Username = %q", settings.Neo4j.Username)
	}
	if settings.Neo4j.Database != "neo4j" {
		t.Fatalf("Database = %q", settings.Neo4j.Database)
	}
	if settings.Neo4j.Password != DefaultNeo4jPassword {
		t.Fatalf("Password = %q", settings.Neo4j.Password)
	}
	if err := settings.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadPrefersApplicationOverrides(t *testing.T) {
	values := map[string]string{
		"GENEALOGY_NEO4J_URI":      "neo4j://example:7687",
		"NEO4J_URI":                "bolt://ignored:7687",
		"GENEALOGY_NEO4J_USERNAME": "family",
		"NEO4J_USERNAME":           "ignored",
		"GENEALOGY_NEO4J_PASSWORD": "secret",
		"GENEALOGY_NEO4J_DATABASE": "family-tree",
	}
	settings := LoadFrom(func(key string) string { return values[key] })

	if settings.Neo4j.URI != "neo4j://example:7687" || settings.Neo4j.Username != "family" || settings.Neo4j.Password != "secret" || settings.Neo4j.Database != "family-tree" {
		t.Fatalf("unexpected configuration: %#v", settings.Neo4j)
	}
	if err := settings.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadFromFileUsesFileValues(t *testing.T) {
	path := writeConfigFile(t, `
# Application configuration
GENEALOGY_NEO4J_URI="neo4j://family.example:7687"
GENEALOGY_NEO4J_USERNAME=family
GENEALOGY_NEO4J_PASSWORD='file-secret'
GENEALOGY_NEO4J_DATABASE=family-tree
`)

	settings := LoadFromFile(path, func(string) string { return "" })

	if settings.Neo4j.URI != "neo4j://family.example:7687" || settings.Neo4j.Username != "family" || settings.Neo4j.Password != "file-secret" || settings.Neo4j.Database != "family-tree" {
		t.Fatalf("unexpected file configuration: %#v", settings.Neo4j)
	}
}

func TestEnvironmentOverridesFileValues(t *testing.T) {
	path := writeConfigFile(t, "GENEALOGY_NEO4J_URI=file-uri\nNEO4J_PASSWORD=file-password\n")
	values := map[string]string{
		"GENEALOGY_NEO4J_URI":      "environment-uri",
		"GENEALOGY_NEO4J_PASSWORD": "environment-password",
	}

	settings := LoadFromFile(path, func(key string) string { return values[key] })

	if settings.Neo4j.URI != "environment-uri" || settings.Neo4j.Password != "environment-password" {
		t.Fatalf("environment values did not override file: %#v", settings.Neo4j)
	}
	if settings.Neo4j.Username != DefaultNeo4jUsername || settings.Neo4j.Database != DefaultNeo4jDatabase {
		t.Fatalf("missing values did not use defaults: %#v", settings.Neo4j)
	}
}

func TestLoadFromFileFallsBackWhenFileIsMissing(t *testing.T) {
	settings := LoadFromFile(filepath.Join(t.TempDir(), "missing.env"), func(string) string { return "" })

	if settings.Neo4j.URI != DefaultNeo4jURI || settings.Neo4j.Username != DefaultNeo4jUsername || settings.Neo4j.Password != DefaultNeo4jPassword || settings.Neo4j.Database != DefaultNeo4jDatabase {
		t.Fatalf("unexpected missing-file defaults: %#v", settings.Neo4j)
	}
}

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.env")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}
