package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ZellijBackend contains zellij-specific path checks
var commonZellijPaths = []string{
	"/usr/bin/zellij",
	"/usr/local/bin/zellij",
	"/opt/homebrew/bin/zellij",
	"/home/linuxbrew/.linuxbrew/bin/zellij",
}

// checkZellijAvailable checks if zellij is available on the system
func checkZellijAvailable() bool {
	if _, err := exec.LookPath("zellij"); err == nil {
		return true
	}

	for _, path := range commonZellijPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// runZellijCommand executes a zellij command with proper environment
func (sm *SessionManager) runZellijCommand(args ...string) error {
	cmd := exec.Command("zellij", args...)
	preserveEnvironment(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// runZellijCommandOutput executes a zellij command and returns output
func (sm *SessionManager) runZellijCommandOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("zellij", args...)
	preserveEnvironment(cmd)
	return cmd.Output()
}

// Zellij session management implementation

func (sm *SessionManager) createZellijSession(name string) error {
	return sm.runZellijCommand("--session", name)
}

func (sm *SessionManager) listZellijSessions() error {
	return sm.runZellijCommand("list-sessions")
}

func (sm *SessionManager) attachZellijSession(name string) error {
	return sm.runZellijCommand("attach", name)
}

func (sm *SessionManager) killZellijSession(name string) error {
	return sm.runZellijCommand("delete-session", name)
}

func (sm *SessionManager) zellijSessionExists(name string) bool {
	output, err := sm.runZellijCommandOutput("list-sessions")
	if err != nil {
		return false
	}
	return strings.Contains(string(output), name)
}

// Zellij window/tab management implementation

func (sm *SessionManager) newZellijTab(session, name string) error {
	// For zellij, we need to use actions within a session
	// This is a simplified approach - in practice, zellij tab management
	// works differently as it's more focused on panes and layouts
	return sm.runZellijCommand("action", "new-tab")
}

func (sm *SessionManager) listZellijTabs(session string) error {
	// Zellij doesn't have a direct equivalent to list tabs/windows
	// This would typically show the current layout/tab info
	return fmt.Errorf("listing tabs not directly supported in zellij - use zellij session view")
}

func (sm *SessionManager) killZellijTab(session, tab string) error {
	return sm.runZellijCommand("action", "close-tab")
}

func (sm *SessionManager) nextZellijTab(session string) error {
	return sm.runZellijCommand("action", "go-to-next-tab")
}

func (sm *SessionManager) previousZellijTab(session string) error {
	return sm.runZellijCommand("action", "go-to-previous-tab")
}

func (sm *SessionManager) renameZellijTab(session, oldName, newName string) error {
	// Zellij doesn't have direct tab renaming in the same way
	// This would typically be done through layouts or tab-specific actions
	return fmt.Errorf("renaming tabs not directly supported in zellij")
}

// Zellij pane management implementation

func (sm *SessionManager) splitZellijPane(session, direction string) error {
	if direction == "v" {
		return sm.runZellijCommand("action", "new-pane", "--direction", "down")
	} else if direction == "h" {
		return sm.runZellijCommand("action", "new-pane", "--direction", "right")
	}
	return fmt.Errorf("invalid split direction for zellij: %s", direction)
}

func (sm *SessionManager) listZellijPanes(session string) error {
	// Zellij shows pane info in its UI, not via command line listing
	return fmt.Errorf("listing panes not directly supported in zellij - use zellij session view")
}

func (sm *SessionManager) killZellijPane(session, pane string) error {
	return sm.runZellijCommand("action", "close-pane")
}

func (sm *SessionManager) resizeZellijPane(session, direction string, size int) error {
	var dir string
	switch direction {
	case "U":
		dir = "up"
	case "D":
		dir = "down"
	case "L":
		dir = "left"
	case "R":
		dir = "right"
	default:
		return fmt.Errorf("invalid resize direction for zellij: %s", direction)
	}
	
	// Zellij resize commands
	for i := 0; i < size; i++ {
		if err := sm.runZellijCommand("action", "resize", dir); err != nil {
			return err
		}
	}
	return nil
}

func (sm *SessionManager) sendKeysZellij(session, keys string) error {
	// Zellij can write text to the current pane
	return sm.runZellijCommand("action", "write-chars", keys)
}

func (sm *SessionManager) detachZellijSession() error {
	return sm.runZellijCommand("action", "detach")
}

func (sm *SessionManager) nukeAllZellijSessions() error {
	// Get all sessions and delete them one by one
	output, err := sm.runZellijCommandOutput("list-sessions")
	if err != nil {
		return fmt.Errorf("failed to list zellij sessions: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	killCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Parse session name from zellij list-sessions output
		// Format is typically: session_name [CREATED: ...]
		parts := strings.Fields(line)
		if len(parts) > 0 {
			sessionName := parts[0]
			if err := sm.runZellijCommand("delete-session", sessionName); err == nil {
				killCount++
			}
		}
	}

	if killCount == 0 {
		return fmt.Errorf("no zellij sessions found to delete")
	}

	return nil
}