package backend

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var commonTmuxPaths = []string{
	"/usr/bin/tmux",
	"/usr/local/bin/tmux",
	"/opt/homebrew/bin/tmux",
	"/home/linuxbrew/.linuxbrew/bin/tmux",
}

type TmuxBackend struct{}

func NewTmuxBackend() *TmuxBackend {
	return &TmuxBackend{}
}

func (b *TmuxBackend) Name() string {
	return "tmux"
}

func (b *TmuxBackend) IsAvailable() bool {
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

func (b *TmuxBackend) runCommand(args ...string) error {
	cmd := exec.Command("tmux", args...)
	preserveEnvironment(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (b *TmuxBackend) SessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

func (b *TmuxBackend) CreateSession(name string, command ...string) error {
	args := []string{"new-session", "-d", "-s", name}
	if len(command) > 0 {
		args = append(args, command...)
	}
	return b.runCommand(args...)
}

func (b *TmuxBackend) ListSessions() error {
	return b.runCommand("list-sessions")
}

func (b *TmuxBackend) DumpSession(name string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-p", "-t", name).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (b *TmuxBackend) GetSessions() ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	preserveEnvironment(cmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	var sessions []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			sessions = append(sessions, line)
		}
	}
	return sessions, nil
}

func (b *TmuxBackend) AttachSession(name string) error {
	return b.runCommand("attach-session", "-t", name)
}

func (b *TmuxBackend) DetachSession() error {
	return b.runCommand("detach-client")
}

func (b *TmuxBackend) KillSession(name string) error {
	return b.runCommand("kill-session", "-t", name)
}

func (b *TmuxBackend) RenameSession(oldName, newName string) error {
	return b.runCommand("rename-session", "-t", oldName, newName)
}

func (b *TmuxBackend) NewWindow(session, name string) error {
	return b.runCommand("new-window", "-t", session, "-n", name)
}

func (b *TmuxBackend) ListWindows(session string) error {
	return b.runCommand("list-windows", "-t", session)
}

func (b *TmuxBackend) KillWindow(session, window string) error {
	return b.runCommand("kill-window", "-t", fmt.Sprintf("%s:%s", session, window))
}

func (b *TmuxBackend) NextWindow(session string) error {
	return b.runCommand("next-window", "-t", session)
}

func (b *TmuxBackend) PreviousWindow(session string) error {
	return b.runCommand("previous-window", "-t", session)
}

func (b *TmuxBackend) RenameWindow(session, oldName, newName string) error {
	windowTarget := fmt.Sprintf("%s:%s", session, oldName)
	return b.runCommand("rename-window", "-t", windowTarget, newName)
}

func (b *TmuxBackend) SplitWindow(session, window, direction string) error {
	windowTarget := fmt.Sprintf("%s:%s", session, window)
	splitFlag := "-v"
	if direction == "h" {
		splitFlag = "-h"
	}
	return b.runCommand("split-window", splitFlag, "-t", windowTarget)
}

func (b *TmuxBackend) ListPanes(session, window string) error {
	windowTarget := fmt.Sprintf("%s:%s", session, window)
	return b.runCommand("list-panes", "-t", windowTarget)
}

func (b *TmuxBackend) KillPane(session, window, pane string) error {
	paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
	return b.runCommand("kill-pane", "-t", paneTarget)
}

func (b *TmuxBackend) Exec(session, window, pane, command string) error {
	paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
	// For execution, we pass the command and append an enter key
	return b.runCommand("send-keys", "-t", paneTarget, command, "Enter")
}

func (b *TmuxBackend) NukeAllSessions() error {
	return b.runCommand("kill-server")
}
