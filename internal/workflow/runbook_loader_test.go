package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleRunbook = `
id: rca-db-pool-v2
name: Database Connection Pool Exhaustion
version: "2"
description: Recover from pg_max_connections exhaustion.
tags:
  - database
  - postgres
steps:
  - id: check_connections
    type: tool_call
    description: Query current connection count
    tool: prometheus_query
    params:
      query: pg_stat_database_numbackends
    requires_approval: false
  - id: increase_pool
    type: api_call
    description: Update connection pool max size
    tool: kubectl_patch
    requires_approval: true
    risk_level: MEDIUM
`

func writeRunbook(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("write runbook: %v", err)
	}
}

func TestLoadRunbookFile_Success(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "rca-db-pool-v2.yaml", sampleRunbook)

	def, err := LoadRunbookFile(filepath.Join(dir, "rca-db-pool-v2.yaml"))
	if err != nil {
		t.Fatalf("LoadRunbookFile error: %v", err)
	}
	if def.ID != "rca-db-pool-v2" {
		t.Errorf("ID = %q, want rca-db-pool-v2", def.ID)
	}
	if len(def.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(def.Steps))
	}
	if def.Steps[1].RequiresApproval != true {
		t.Error("step 2 should require approval")
	}
	if def.Steps[1].RiskLevel != "MEDIUM" {
		t.Errorf("step 2 risk_level = %q, want MEDIUM", def.Steps[1].RiskLevel)
	}
}

func TestLoadRunbookFile_MissingID_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "bad.yaml", "name: No ID runbook\n")
	_, err := LoadRunbookFile(filepath.Join(dir, "bad.yaml"))
	if err == nil {
		t.Error("expected error for runbook without id field")
	}
}

func TestLoadRunbookFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "broken.yaml", ": not: valid: yaml: {{{{")
	_, err := LoadRunbookFile(filepath.Join(dir, "broken.yaml"))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadRunbookFile_NotExist(t *testing.T) {
	_, err := LoadRunbookFile("/nonexistent/runbook.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestRunbookLoader_LoadByID_YAMLExtension(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "rca-db-pool-v2.yaml", sampleRunbook)
	loader := NewRunbookLoader(dir)

	def, err := loader.LoadByID("rca-db-pool-v2")
	if err != nil {
		t.Fatalf("LoadByID error: %v", err)
	}
	if def.ID != "rca-db-pool-v2" {
		t.Errorf("ID = %q, want rca-db-pool-v2", def.ID)
	}
}

func TestRunbookLoader_LoadByID_YMLExtension(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "oom-kill.yml", "id: oom-kill\nname: OOM Kill Recovery\n")
	loader := NewRunbookLoader(dir)

	def, err := loader.LoadByID("oom-kill")
	if err != nil {
		t.Fatalf("LoadByID .yml error: %v", err)
	}
	if def.ID != "oom-kill" {
		t.Errorf("ID = %q, want oom-kill", def.ID)
	}
}

func TestRunbookLoader_LoadByID_NotFound(t *testing.T) {
	loader := NewRunbookLoader(t.TempDir())
	_, err := loader.LoadByID("nonexistent-runbook")
	if err == nil {
		t.Error("expected error for nonexistent runbook")
	}
}

func TestRunbookLoader_LoadAll_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "rca-db-pool-v2.yaml", sampleRunbook)
	writeRunbook(t, dir, "oom-kill.yaml", "id: oom-kill\nname: OOM Kill\n")
	writeRunbook(t, dir, "not-a-yaml.txt", "should be ignored")

	loader := NewRunbookLoader(dir)
	all, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("LoadAll = %d runbooks, want 2", len(all))
	}
	if _, ok := all["rca-db-pool-v2"]; !ok {
		t.Error("expected rca-db-pool-v2 in LoadAll result")
	}
}

func TestRunbookLoader_LoadAll_BadDir(t *testing.T) {
	loader := NewRunbookLoader("/nonexistent/dir")
	_, err := loader.LoadAll()
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestRunbookDefinition_ToRunbookPlan(t *testing.T) {
	dir := t.TempDir()
	writeRunbook(t, dir, "rca-db-pool-v2.yaml", sampleRunbook)
	def, _ := LoadRunbookFile(filepath.Join(dir, "rca-db-pool-v2.yaml"))

	plan := def.ToRunbookPlan()
	if plan.RunbookID != "rca-db-pool-v2" {
		t.Errorf("plan.RunbookID = %q, want rca-db-pool-v2", plan.RunbookID)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("plan.Steps = %d, want 2", len(plan.Steps))
	}
	if plan.Steps[0].Tool != "prometheus_query" {
		t.Errorf("step 0 tool = %q, want prometheus_query", plan.Steps[0].Tool)
	}
	if !plan.Steps[1].RequiresApproval {
		t.Error("step 1 should require approval in plan")
	}
}
