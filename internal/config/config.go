package config

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"

	"genealogymapper/internal/graph"
)

const (
	ConfigFileEnvironment = "GENEALOGY_CONFIG_FILE"
	DefaultConfigFile     = ".env"
	DefaultNeo4jURI       = "bolt://localhost:7687"
	DefaultNeo4jUsername  = "neo4j"
	DefaultNeo4jPassword  = "genealogy-dev-password"
	DefaultNeo4jDatabase  = "neo4j"
)

type Config struct {
	Neo4j graph.Neo4jConfig
}

func Load() Config {
	path := strings.TrimSpace(os.Getenv(ConfigFileEnvironment))
	if path == "" {
		path = DefaultConfigFile
	}
	return LoadFromFile(path, os.Getenv)
}

func LoadFrom(get func(string) string) Config {
	return load(nil, get)
}

func LoadFromFile(path string, get func(string) string) Config {
	return load(readConfigFile(path), get)
}

func load(fileValues map[string]string, get func(string) string) Config {
	return Config{
		Neo4j: graph.Neo4jConfig{
			URI:      valueOrDefault(firstValueFrom(fileValues, get, "GENEALOGY_NEO4J_URI", "NEO4J_URI"), DefaultNeo4jURI),
			Username: valueOrDefault(firstValueFrom(fileValues, get, "GENEALOGY_NEO4J_USERNAME", "NEO4J_USERNAME"), DefaultNeo4jUsername),
			Password: valueOrDefault(firstValueFrom(fileValues, get, "GENEALOGY_NEO4J_PASSWORD", "NEO4J_PASSWORD"), DefaultNeo4jPassword),
			Database: valueOrDefault(firstValueFrom(fileValues, get, "GENEALOGY_NEO4J_DATABASE", "NEO4J_DATABASE"), DefaultNeo4jDatabase),
		},
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Neo4j.URI) == "" {
		return errors.New("GENEALOGY_NEO4J_URI is required")
	}
	if strings.TrimSpace(c.Neo4j.Username) == "" {
		return errors.New("GENEALOGY_NEO4J_USERNAME is required")
	}
	if c.Neo4j.Password == "" {
		return errors.New("GENEALOGY_NEO4J_PASSWORD is required")
	}
	if strings.TrimSpace(c.Neo4j.Database) == "" {
		return errors.New("GENEALOGY_NEO4J_DATABASE is required")
	}
	return nil
}

func firstValue(get func(string) string, keys ...string) string {
	return firstValueFrom(nil, get, keys...)
}

func firstValueFrom(fileValues map[string]string, get func(string) string, keys ...string) string {
	for _, key := range keys {
		if get != nil {
			if value := strings.TrimSpace(get(key)); value != "" {
				return value
			}
		}
	}
	for _, key := range keys {
		if value := strings.TrimSpace(fileValues[key]); value != "" {
			return value
		}
	}
	return ""
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func readConfigFile(path string) map[string]string {
	values := make(map[string]string)
	file, err := os.Open(path)
	if err != nil {
		return values
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values[key] = parseConfigValue(strings.TrimSpace(value))
	}
	return values
}

func parseConfigValue(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] != '"' || value[len(value)-1] != '"') && (value[0] != '\'' || value[len(value)-1] != '\'') {
		return value
	}
	unquoted, err := strconv.Unquote(value)
	if err != nil {
		return value[1 : len(value)-1]
	}
	return unquoted
}
