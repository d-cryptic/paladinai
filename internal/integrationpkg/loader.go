// Package integrationpkg loads integration definitions from YAML files
// stored under the repository's integrations/ directory. Each integration
// is described by an integration.yaml file in its own subdirectory.
package integrationpkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned when an integration with the requested name
// does not exist in the integrations directory.
var ErrNotFound = errors.New("integration not found")

// Integration represents the parsed integration.yaml definition.
type Integration struct {
	Name         string           `yaml:"name" json:"name"`
	Version      string           `yaml:"version" json:"version"`
	Description  string           `yaml:"description" json:"description"`
	Receiver     ReceiverConfig   `yaml:"receiver" json:"receiver"`
	ConfigSchema map[string]Field `yaml:"config_schema" json:"config_schema,omitempty"`
	Auth         AuthConfig       `yaml:"auth" json:"auth"`
	Tools        []string         `yaml:"tools" json:"tools"`
	DocsURL      string           `yaml:"docs_url" json:"docs_url,omitempty"`
}

// ReceiverConfig describes how alerts arrive from a third-party source.
type ReceiverConfig struct {
	Type       string `yaml:"type" json:"type"` // "webhook" | "none"
	Path       string `yaml:"path" json:"path,omitempty"`
	HMACHeader string `yaml:"hmac_header" json:"hmac_header,omitempty"`
}

// Field describes a single key in the integration's config_schema.
type Field struct {
	Type        string `yaml:"type" json:"type"`
	Default     any    `yaml:"default" json:"default,omitempty"`
	Required    bool   `yaml:"required" json:"required,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
	Secret      bool   `yaml:"secret" json:"secret,omitempty"`
}

// AuthConfig describes how to authenticate against the third-party API.
type AuthConfig struct {
	Type   string      `yaml:"type" json:"type"`
	Fields []AuthField `yaml:"fields" json:"fields,omitempty"`
}

// AuthField is a single credential field expected from operators.
type AuthField struct {
	Name        string `yaml:"name" json:"name"`
	Secret      bool   `yaml:"secret" json:"secret,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
}

// LoadAll reads every integration.yaml file under dir and returns them
// sorted by Name. dir should point at the integrations/ root.
func LoadAll(dir string) ([]Integration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read integrations dir: %w", err)
	}

	out := make([]Integration, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name(), "integration.yaml")
		if _, statErr := os.Stat(path); statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", path, statErr)
		}
		integ, err := loadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		out = append(out, *integ)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// LoadByName loads a single integration by directory/name.
func LoadByName(dir, name string) (*Integration, error) {
	if name == "" {
		return nil, fmt.Errorf("integration name is required")
	}
	path := filepath.Join(dir, name, "integration.yaml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return loadFile(path)
}

func loadFile(path string) (*Integration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	var integ Integration
	if err := yaml.Unmarshal(data, &integ); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if integ.Name == "" {
		return nil, fmt.Errorf("integration at %s has empty name", path)
	}
	return &integ, nil
}
