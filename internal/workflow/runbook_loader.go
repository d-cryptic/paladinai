// runbook_loader.go implements loading of runbook definitions from YAML files
// on disk. Runbooks are stored in a directory (e.g., runbooks/) and loaded by
// name. The loader is used both by the agent (selecting the best runbook) and
// by the executor (following runbook steps).
//
// YAML schema for a runbook file:
//
//	id: rca-db-pool-v2
//	name: Database Connection Pool Exhaustion
//	version: "2"
//	description: Recover from pg_max_connections exhaustion.
//	tags: [database, postgres, pool]
//	steps:
//	  - id: check_current_connections
//	    type: tool_call
//	    description: Query current connection count
//	    tool: prometheus_query
//	    params: { query: "pg_stat_database_numbackends" }
//	    requires_approval: false
//	  - id: increase_pool_size
//	    type: api_call
//	    description: Update connection pool max size
//	    tool: kubectl_patch
//	    requires_approval: true
//	    risk_level: MEDIUM
package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RunbookDefinition is the full YAML schema for a runbook file.
type RunbookDefinition struct {
	ID          string           `yaml:"id"`
	Name        string           `yaml:"name"`
	Version     string           `yaml:"version"`
	Description string           `yaml:"description"`
	Tags        []string         `yaml:"tags"`
	Steps       []RunbookStepDef `yaml:"steps"`
}

// RunbookStepDef is the YAML schema for one runbook step.
type RunbookStepDef struct {
	ID               string         `yaml:"id"`
	Type             string         `yaml:"type"`
	Description      string         `yaml:"description"`
	Tool             string         `yaml:"tool"`
	Params           map[string]any `yaml:"params,omitempty"`
	RequiresApproval bool           `yaml:"requires_approval"`
	RiskLevel        string         `yaml:"risk_level,omitempty"` // HIGH | MEDIUM | LOW
}

// ToRunbookPlan converts a RunbookDefinition to the executor-friendly RunbookPlan.
func (rd RunbookDefinition) ToRunbookPlan() RunbookPlan {
	steps := make([]RunbookStep, len(rd.Steps))
	for i, s := range rd.Steps {
		// Convert map[string]any params to map[string]string args.
		args := make(map[string]string, len(s.Params))
		for k, v := range s.Params {
			args[k] = fmt.Sprintf("%v", v)
		}
		steps[i] = RunbookStep{
			StepID:           s.ID,
			Name:             s.Description,
			Type:             StepType(s.Type),
			Tool:             s.Tool,
			Args:             args,
			RequiresApproval: s.RequiresApproval,
		}
	}
	return RunbookPlan{
		RunbookID: rd.ID,
		Name:      rd.Name,
		Steps:     steps,
	}
}

// RunbookLoader loads RunbookDefinition files from a directory.
type RunbookLoader struct {
	dir string
}

// NewRunbookLoader creates a loader pointing at dir.
func NewRunbookLoader(dir string) *RunbookLoader {
	return &RunbookLoader{dir: dir}
}

// LoadByID loads the runbook definition with the given ID from dir.
// It searches for a file named {id}.yaml or {id}.yml.
// id must contain only alphanumeric characters, hyphens, and underscores
// to prevent path traversal attacks.
func (l *RunbookLoader) LoadByID(id string) (RunbookDefinition, error) {
	if err := validateRunbookID(id); err != nil {
		return RunbookDefinition{}, err
	}
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(l.dir, id+ext)
		def, err := LoadRunbookFile(path)
		if err == nil {
			return def, nil
		}
		if !os.IsNotExist(err) {
			return RunbookDefinition{}, err
		}
	}
	return RunbookDefinition{}, fmt.Errorf("runbook %q not found in %s", id, l.dir)
}

// validateRunbookID rejects IDs that could cause path traversal.
// Only alphanumeric characters, hyphens, underscores, and dots (but not leading dots) are allowed.
func validateRunbookID(id string) error {
	if id == "" {
		return fmt.Errorf("runbook id must not be empty")
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("runbook id %q contains invalid path separator", id)
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("runbook id %q contains invalid sequence", id)
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' && c != '.' {
			return fmt.Errorf("runbook id %q contains invalid character %q", id, c)
		}
	}
	return nil
}

// LoadAll loads all runbook YAML files from dir and returns them indexed by ID.
func (l *RunbookLoader) LoadAll() (map[string]RunbookDefinition, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, fmt.Errorf("read runbook dir %s: %w", l.dir, err)
	}
	result := make(map[string]RunbookDefinition)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		path := filepath.Join(l.dir, name)
		def, err := LoadRunbookFile(path)
		if err != nil {
			// Skip unparseable files rather than aborting the full load;
			// a bad file should not hide all other runbooks.
			continue
		}
		result[def.ID] = def
	}
	return result, nil
}

// LoadRunbookFile parses a single runbook YAML file.
func LoadRunbookFile(path string) (RunbookDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RunbookDefinition{}, err
	}
	var def RunbookDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return RunbookDefinition{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if def.ID == "" {
		return RunbookDefinition{}, fmt.Errorf("runbook file %s: missing id field", path)
	}
	return def, nil
}
