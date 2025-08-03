package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
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

// runZellijCommandWithSession executes a zellij command with session context
func (sm *SessionManager) runZellijCommandWithSession(session string, args ...string) error {
	fullArgs := append([]string{"-s", session}, args...)
	cmd := exec.Command("zellij", fullArgs...)
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

// Zellij session management implementation

func (sm *SessionManager) createZellijSession(name string) error {
	// Check if session already exists
	if sm.zellijSessionExists(name) {
		return fmt.Errorf("session with name \"%s\" already exists. Use attach command to connect to it or specify a different name", name)
	}
	
	// Use zellij's proper background session creation
	cmd := exec.Command("zellij", "attach", "--create-background", name)
	preserveEnvironment(cmd)
	
	// Redirect outputs to prevent hanging
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create zellij session: %v", err)
	}
	
	// Verify session was created
	if !sm.zellijSessionExists(name) {
		return fmt.Errorf("session creation appeared to succeed but session not found")
	}
	
	return nil
}

func (sm *SessionManager) listZellijSessions() error {
	return sm.runZellijCommand("list-sessions")
}

func (sm *SessionManager) attachZellijSession(name string) error {
	return sm.runZellijCommand("attach", name)
}

func (sm *SessionManager) killZellijSession(name string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(name) {
		return fmt.Errorf("session '%s' does not exist", name)
	}
	
	// Try normal delete first
	err := sm.runZellijCommand("delete-session", name)
	if err != nil {
		// If it fails, try with --force flag
		return sm.runZellijCommand("delete-session", "--force", name)
	}
	return nil
}

func (sm *SessionManager) zellijSessionExists(name string) bool {
	output, err := sm.runZellijCommandOutput("list-sessions")
	if err != nil {
		// If zellij list-sessions fails, assume no sessions exist
		return false
	}
	
	// Simple string contains check for now - more precise later
	return strings.Contains(string(output), name)
}

// Zellij window/tab management implementation

func (sm *SessionManager) newZellijTab(session, name string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
	// For zellij, create a new tab with session context
	if name != "" {
		return sm.runZellijCommandWithSession(session, "action", "new-tab", "--name", name)
	}
	return sm.runZellijCommandWithSession(session, "action", "new-tab")
}

func (sm *SessionManager) listZellijTabs(session string) error {
	// Zellij doesn't have a direct equivalent to list tabs/windows
	// This would typically show the current layout/tab info
	return fmt.Errorf("listing tabs not directly supported in zellij - use zellij session view")
}

func (sm *SessionManager) killZellijTab(session, tab string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
	return sm.runZellijCommandWithSession(session, "action", "close-tab")
}

func (sm *SessionManager) nextZellijTab(session string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
	return sm.runZellijCommandWithSession(session, "action", "go-to-next-tab")
}

func (sm *SessionManager) previousZellijTab(session string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
	return sm.runZellijCommandWithSession(session, "action", "go-to-previous-tab")
}

func (sm *SessionManager) renameZellijTab(session, oldName, newName string) error {
	// Zellij doesn't have direct tab renaming in the same way
	// This would typically be done through layouts or tab-specific actions
	return fmt.Errorf("renaming tabs not directly supported in zellij")
}

// Zellij pane management implementation

func (sm *SessionManager) splitZellijPane(session, direction string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
	if direction == "v" {
		return sm.runZellijCommandWithSession(session, "action", "new-pane", "--direction", "down")
	} else if direction == "h" {
		return sm.runZellijCommandWithSession(session, "action", "new-pane", "--direction", "right")
	}
	return fmt.Errorf("invalid split direction for zellij: %s", direction)
}

func (sm *SessionManager) listZellijPanes(session string) error {
	// Zellij shows pane info in its UI, not via command line listing
	return fmt.Errorf("listing panes not directly supported in zellij - use zellij session view")
}

func (sm *SessionManager) killZellijPane(session, pane string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
	return sm.runZellijCommandWithSession(session, "action", "close-pane")
}

func (sm *SessionManager) resizeZellijPane(session, direction string, size int) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
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
		if err := sm.runZellijCommandWithSession(session, "action", "resize", dir); err != nil {
			return err
		}
	}
	return nil
}

func (sm *SessionManager) sendKeysZellij(session, keys string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
	// Zellij can write text to the current pane
	return sm.runZellijCommandWithSession(session, "action", "write-chars", keys)
}

func (sm *SessionManager) detachZellijSession() error {
	// Zellij doesn't have a traditional detach concept like tmux
	// Instead, we can switch to another session or just exit
	// For now, we'll return an error indicating this operation isn't supported
	return fmt.Errorf("detach operation not supported in zellij - zellij uses a different session paradigm")
}

func (sm *SessionManager) closeZellijPane(session string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
	return sm.runZellijCommandWithSession(session, "action", "close-pane")
}

func (sm *SessionManager) sendKeysToZellijPane(session, keys string) error {
	// Check if session exists first
	if !sm.zellijSessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	
	// Zellij can write text to the current pane
	return sm.runZellijCommandWithSession(session, "action", "write-chars", keys)
}

func (sm *SessionManager) nukeAllZellijSessions() error {
	// Use delete-all-sessions command with automatic yes and force flags
	if err := sm.runZellijCommand("delete-all-sessions", "-y", "-f"); err == nil {
		return nil
	}
	
	// Fallback: Get all sessions and delete them one by one
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
		
		// Remove ANSI color codes
		// Simple regex to remove ANSI escape sequences
		re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
		cleanLine := re.ReplaceAllString(line, "")
		
		// Parse session name from zellij list-sessions output
		// Format is typically: session_name [CREATED: ...]
		parts := strings.Fields(cleanLine)
		if len(parts) > 0 {
			sessionName := parts[0]
			// Try normal delete first, then force delete
			if err := sm.runZellijCommand("delete-session", sessionName); err != nil {
				sm.runZellijCommand("delete-session", "--force", sessionName)
			}
			killCount++
		}
	}

	if killCount == 0 {
		return fmt.Errorf("no zellij sessions found to delete")
	}

	return nil
}