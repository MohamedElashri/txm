package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfile(t *testing.T) {
	// Create a temporary TOML file
	content := []byte(`
name = "test-workspace"
backend = "tmux"

[[window]]
name = "editor"
[[window.pane]]
command = "vim"

[[window]]
name = "server"
[[window.pane]]
command = "npm start"
[[window.pane]]
split = "h"
command = "npm run tail"
`)
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.toml")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	// Load the profile
	prof, err := LoadProfile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load profile: %v", err)
	}

	// Assertions
	if prof.Name != "test-workspace" {
		t.Errorf("Expected name 'test-workspace', got '%s'", prof.Name)
	}
	if prof.Backend != "tmux" {
		t.Errorf("Expected backend 'tmux', got '%s'", prof.Backend)
	}
	if len(prof.Windows) != 2 {
		t.Fatalf("Expected 2 windows, got %d", len(prof.Windows))
	}

	// Window 1
	if prof.Windows[0].Name != "editor" {
		t.Errorf("Expected window 1 name 'editor', got '%s'", prof.Windows[0].Name)
	}
	if len(prof.Windows[0].Panes) != 1 {
		t.Fatalf("Expected 1 pane in window 1, got %d", len(prof.Windows[0].Panes))
	}
	if prof.Windows[0].Panes[0].Command != "vim" {
		t.Errorf("Expected pane 1 command 'vim', got '%s'", prof.Windows[0].Panes[0].Command)
	}

	// Window 2
	if prof.Windows[1].Name != "server" {
		t.Errorf("Expected window 2 name 'server', got '%s'", prof.Windows[1].Name)
	}
	if len(prof.Windows[1].Panes) != 2 {
		t.Fatalf("Expected 2 panes in window 2, got %d", len(prof.Windows[1].Panes))
	}
	if prof.Windows[1].Panes[0].Command != "npm start" {
		t.Errorf("Expected pane 1 command 'npm start', got '%s'", prof.Windows[1].Panes[0].Command)
	}
	if prof.Windows[1].Panes[1].Command != "npm run tail" {
		t.Errorf("Expected pane 2 command 'npm run tail', got '%s'", prof.Windows[1].Panes[1].Command)
	}
	if prof.Windows[1].Panes[1].Split != "h" {
		t.Errorf("Expected pane 2 split 'h', got '%s'", prof.Windows[1].Panes[1].Split)
	}
}
