package main

import (
	"fmt"
	"os"
	"os/exec"
)

// tmux backend specific paths
var commonTmuxPaths = []string{
	"/usr/bin/tmux",
	"/usr/local/bin/tmux",
	"/opt/homebrew/bin/tmux",
	"/home/linuxbrew/.linuxbrew/bin/tmux",
}

// checkTmuxAvailable checks if tmux is available on the system
func checkTmuxAvailable() bool {
	if _, err := exec.LookPath("tmux"); err == nil {
		return true
	}

	for _, path := range commonTmuxPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// runTmuxCommand executes a tmux command with proper environment
func (sm *SessionManager) runTmuxCommand(args ...string) error {
	cmd := exec.Command("tmux", args...)
	preserveEnvironment(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// tmux-specific session management

func (sm *SessionManager) tmuxSessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

func (sm *SessionManager) tmuxCreateSession(name string) error {
	return sm.runTmuxCommand("new-session", "-d", "-s", name)
}

func (sm *SessionManager) tmuxListSessions() error {
	return sm.runTmuxCommand("list-sessions")
}

func (sm *SessionManager) tmuxAttachSession(name string) error {
	return sm.runTmuxCommand("attach-session", "-t", name)
}

func (sm *SessionManager) tmuxDetachSession() error {
	return sm.runTmuxCommand("detach-client")
}

func (sm *SessionManager) tmuxKillSession(name string) error {
	return sm.runTmuxCommand("kill-session", "-t", name)
}

func (sm *SessionManager) tmuxRenameSession(oldName, newName string) error {
	return sm.runTmuxCommand("rename-session", "-t", oldName, newName)
}

// tmux-specific window management

func (sm *SessionManager) tmuxNewWindow(session, name string) error {
	return sm.runTmuxCommand("new-window", "-t", session, "-n", name)
}

func (sm *SessionManager) tmuxListWindows(session string) error {
	return sm.runTmuxCommand("list-windows", "-t", session)
}

func (sm *SessionManager) tmuxKillWindow(session, window string) error {
	return sm.runTmuxCommand("kill-window", "-t", fmt.Sprintf("%s:%s", session, window))
}

func (sm *SessionManager) tmuxNextWindow(session string) error {
	return sm.runTmuxCommand("next-window", "-t", session)
}

func (sm *SessionManager) tmuxPreviousWindow(session string) error {
	return sm.runTmuxCommand("previous-window", "-t", session)
}

func (sm *SessionManager) tmuxRenameWindow(session, oldName, newName string) error {
	windowTarget := fmt.Sprintf("%s:%s", session, oldName)
	return sm.runTmuxCommand("rename-window", "-t", windowTarget, newName)
}

func (sm *SessionManager) tmuxMoveWindow(srcSession, windowName, dstSession string) error {
	windowTarget := fmt.Sprintf("%s:%s", srcSession, windowName)
	return sm.runTmuxCommand("move-window", "-s", windowTarget, "-t", dstSession)
}

func (sm *SessionManager) tmuxSwapWindow(session, windowName1, windowName2 string) error {
	windowTarget1 := fmt.Sprintf("%s:%s", session, windowName1)
	windowTarget2 := fmt.Sprintf("%s:%s", session, windowName2)
	return sm.runTmuxCommand("swap-window", "-s", windowTarget1, "-t", windowTarget2)
}

// tmux-specific pane management

func (sm *SessionManager) tmuxSplitWindow(session, window, direction string) error {
	windowTarget := fmt.Sprintf("%s:%s", session, window)
	splitFlag := "-v" // vertical split (one pane above another)
	if direction == "h" {
		splitFlag = "-h" // horizontal split (panes side by side)
	}
	return sm.runTmuxCommand("split-window", splitFlag, "-t", windowTarget)
}

func (sm *SessionManager) tmuxListPanes(session, window string) error {
	windowTarget := fmt.Sprintf("%s:%s", session, window)
	return sm.runTmuxCommand("list-panes", "-t", windowTarget)
}

func (sm *SessionManager) tmuxKillPane(session, window, pane string) error {
	paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
	return sm.runTmuxCommand("kill-pane", "-t", paneTarget)
}

func (sm *SessionManager) tmuxResizePane(session, window, pane, direction string, size int) error {
	paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
	resizeFlag := "-U" // up
	if direction == "D" {
		resizeFlag = "-D" // down
	} else if direction == "L" {
		resizeFlag = "-L" // left
	} else if direction == "R" {
		resizeFlag = "-R" // right
	}
	return sm.runTmuxCommand("resize-pane", resizeFlag, fmt.Sprintf("%d", size), "-t", paneTarget)
}

func (sm *SessionManager) tmuxSendKeys(session, window, pane, keys string) error {
	paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
	return sm.runTmuxCommand("send-keys", "-t", paneTarget, keys)
}

func (sm *SessionManager) tmuxNukeAllSessions() error {
	return sm.runTmuxCommand("kill-server")
}