package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func defaultDashboardSessionPath() string {
	if path := strings.TrimSpace(os.Getenv("PALADIN_TUI_SESSION_PATH")); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".paladin", "session.json")
}

func loadSessionState(path string) (SessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SessionState{}, err
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return SessionState{}, fmt.Errorf("parse session: %w", err)
	}
	if !isDashboardMode(state.Mode) {
		state.Mode = "monitor"
	}
	return state, nil
}

func saveSessionState(path string, state SessionState) error {
	if state.Mode == "" {
		state.Mode = "monitor"
	}
	if len(state.Recent) > 8 {
		state.Recent = append([]string(nil), state.Recent[:8]...)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}
