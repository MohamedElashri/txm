package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// runScreenCommand executes a screen command with proper environment
func (sm *SessionManager) runScreenCommand(args ...string) error {
	cmd := exec.Command("screen", args...)
	preserveEnvironment(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// screen-specific session management

func (sm *SessionManager) screenSessionExists(name string) bool {
	cmd := exec.Command("screen", "-ls")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "."+name) && (strings.Contains(line, "(Attached)") || strings.Contains(line, "(Detached)")) {
			return true
		}
	}
	return false
}

func (sm *SessionManager) screenCreateSession(name string) error {
	return sm.runScreenCommand("-dmS", name)
}

func (sm *SessionManager) screenListSessions() error {
	return sm.runScreenCommand("-ls")
}

func (sm *SessionManager) screenAttachSession(name string) error {
	return sm.runScreenCommand("-r", name)
}

func (sm *SessionManager) screenDetachSession() error {
	// Screen doesn't have a direct detach command like tmux
	// This would typically be done with Ctrl-A d
	return fmt.Errorf("screen detach must be done manually with Ctrl-A d")
}

func (sm *SessionManager) screenKillSession(name string) error {
	// Get all matching sessions and kill them
	cmd := exec.Command("screen", "-ls")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// Find all sessions with the given name and kill them
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "."+name) && (strings.Contains(line, "(Attached)") || strings.Contains(line, "(Detached)")) {
			// Extract session ID (format: pid.name)
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) > 0 {
				sessionID := parts[0]
				if strings.Contains(sessionID, ".") {
					cmd := exec.Command("screen", "-S", sessionID, "-X", "quit")
					cmd.Run() // Ignore errors for individual session kills
				}
			}
		}
	}
	return nil
}

func (sm *SessionManager) screenRenameSession(oldName, newName string) error {
	return fmt.Errorf("screen does not support session renaming")
}

// screen-specific window management

func (sm *SessionManager) screenNewWindow(session, name string) error {
	return sm.runScreenCommand("-S", session, "-X", "screen", "-t", name)
}

func (sm *SessionManager) screenListWindows(session string) error {
	return sm.runScreenCommand("-S", session, "-X", "windows")
}

func (sm *SessionManager) screenKillWindow(session, window string) error {
	// Screen doesn't support killing windows by name directly
	return fmt.Errorf("screen does not support killing windows by name")
}

func (sm *SessionManager) screenNextWindow(session string) error {
	return sm.runScreenCommand("-S", session, "-X", "next")
}

func (sm *SessionManager) screenPreviousWindow(session string) error {
	return sm.runScreenCommand("-S", session, "-X", "prev")
}

func (sm *SessionManager) screenRenameWindow(session, oldName, newName string) error {
	return sm.runScreenCommand("-S", session, "-X", "title", newName)
}

func (sm *SessionManager) screenMoveWindow(srcSession, windowName, dstSession string) error {
	return fmt.Errorf("screen does not support moving windows between sessions")
}

func (sm *SessionManager) screenSwapWindow(session, windowName1, windowName2 string) error {
	return fmt.Errorf("screen does not support swapping windows")
}

// screen-specific pane management

func (sm *SessionManager) screenSplitWindow(session, window, direction string) error {
	if direction == "h" {
		return sm.runScreenCommand("-S", session, "-X", "split", "-v")
	}
	return fmt.Errorf("horizontal splitting not supported in screen")
}

func (sm *SessionManager) screenListPanes(session, window string) error {
	// Screen doesn't have direct pane listing like tmux
	return fmt.Errorf("screen does not support listing panes")
}

func (sm *SessionManager) screenKillPane(session, window, pane string) error {
	return fmt.Errorf("screen does not support killing individual panes")
}

func (sm *SessionManager) screenResizePane(session, window, pane, direction string, size int) error {
	return fmt.Errorf("screen does not support resizing panes")
}

func (sm *SessionManager) screenSendKeys(session, window, pane, keys string) error {
	return fmt.Errorf("screen does not support sending keys to specific panes")
}

func (sm *SessionManager) screenNukeAllSessions() error {
	// Kill all screen sessions
	cmd := exec.Command("screen", "-ls")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// Parse screen -ls output and kill all sessions
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "(Attached)") || strings.Contains(line, "(Detached)") {
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) > 0 {
				sessionID := parts[0]
				if strings.Contains(sessionID, ".") {
					// Remove ANSI color codes from session names if present
					colorCodeRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
					sessionID = colorCodeRegex.ReplaceAllString(sessionID, "")
					
					cmd := exec.Command("screen", "-S", sessionID, "-X", "quit")
					cmd.Run() // Ignore errors for individual session kills
				}
			}
		}
	}
	return nil
}