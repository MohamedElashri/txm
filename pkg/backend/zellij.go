package backend

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var commonZellijPaths = []string{
	"/usr/bin/zellij",
	"/usr/local/bin/zellij",
	"/opt/homebrew/bin/zellij",
	"/home/linuxbrew/.linuxbrew/bin/zellij",
}

type ZellijBackend struct{}

func NewZellijBackend() *ZellijBackend {
	return &ZellijBackend{}
}

func (b *ZellijBackend) Name() string {
	return "zellij"
}

func (b *ZellijBackend) IsAvailable() bool {
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

func (b *ZellijBackend) runCommand(args ...string) error {
	cmd := exec.Command("zellij", args...)
	preserveEnvironment(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (b *ZellijBackend) runCommandWithSession(session string, args ...string) error {
	fullArgs := append([]string{"-s", session}, args...)
	cmd := exec.Command("zellij", fullArgs...)
	preserveEnvironment(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (b *ZellijBackend) runCommandOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("zellij", args...)
	preserveEnvironment(cmd)
	return cmd.Output()
}

func (b *ZellijBackend) SessionExists(name string) bool {
	output, err := b.runCommandOutput("list-sessions")
	if err != nil {
		return false
	}
	return strings.Contains(string(output), name)
}

func (b *ZellijBackend) CreateSession(name string) error {
	if b.SessionExists(name) {
		return fmt.Errorf("session with name \"%s\" already exists", name)
	}

	cmd := exec.Command("zellij", "attach", "--create-background", name)
	preserveEnvironment(cmd)

	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create zellij session: %v", err)
	}

	if !b.SessionExists(name) {
		return fmt.Errorf("session creation appeared to succeed but session not found")
	}

	return nil
}

func (b *ZellijBackend) ListSessions() error {
	return b.runCommand("list-sessions")
}

func (b *ZellijBackend) AttachSession(name string) error {
	return b.runCommand("attach", name)
}

func (b *ZellijBackend) KillSession(name string) error {
	if !b.SessionExists(name) {
		return fmt.Errorf("session '%s' does not exist", name)
	}
	err := b.runCommand("delete-session", name)
	if err != nil {
		return b.runCommand("delete-session", "--force", name)
	}
	return nil
}

func (b *ZellijBackend) RenameSession(oldName, newName string) error {
	return fmt.Errorf("zellij does not support session renaming")
}

func (b *ZellijBackend) NewWindow(session, name string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	if name != "" {
		return b.runCommandWithSession(session, "action", "new-tab", "--name", name)
	}
	return b.runCommandWithSession(session, "action", "new-tab")
}

func (b *ZellijBackend) ListWindows(session string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	err := b.runCommandWithSession(session, "action", "dump-layout")
	if err != nil {
		output, listErr := b.runCommandOutput("list-sessions")
		if listErr != nil {
			return fmt.Errorf("failed to get session information: %v", listErr)
		}
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, session) {
				fmt.Println(line)
			}
		}
		return nil
	}
	return nil
}

func (b *ZellijBackend) KillWindow(session, window string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	return b.runCommandWithSession(session, "action", "close-tab")
}

func (b *ZellijBackend) NextWindow(session string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	return b.runCommandWithSession(session, "action", "go-to-next-tab")
}

func (b *ZellijBackend) PreviousWindow(session string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	return b.runCommandWithSession(session, "action", "go-to-previous-tab")
}

func (b *ZellijBackend) RenameWindow(session, oldName, newName string) error {
	return fmt.Errorf("renaming tabs not directly supported in zellij")
}

func (b *ZellijBackend) MoveWindow(srcSession, windowName, dstSession string) error {
	return fmt.Errorf("zellij does not support moving windows between sessions")
}

func (b *ZellijBackend) SwapWindow(session, windowName1, windowName2 string) error {
	return fmt.Errorf("zellij does not support swapping windows")
}

func (b *ZellijBackend) SplitWindow(session, window, direction string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	switch direction {
	case "v":
		return b.runCommandWithSession(session, "action", "new-pane", "--direction", "down")
	case "h":
		return b.runCommandWithSession(session, "action", "new-pane", "--direction", "right")
	default:
		return fmt.Errorf("invalid split direction for zellij: %s", direction)
	}
}

func (b *ZellijBackend) focusPane(session, window, pane string) error {
	if pane == "" || pane == "0" {
		return nil
	}
	// Best-effort navigation; errors are intentionally ignored since focus
	// operations are best-effort in zellij's focus-based model.
	_ = b.runCommandWithSession(session, "action", "move-focus-or-tab", "left")
	_ = b.runCommandWithSession(session, "action", "move-focus-or-tab", "up")
	targetPane := 1
	if p, err := strconv.Atoi(pane); err == nil && p > 0 {
		targetPane = p
	}
	for i := 1; i < targetPane; i++ {
		if err := b.runCommandWithSession(session, "action", "focus-next-pane"); err != nil {
			break
		}
	}
	return nil
}

func (b *ZellijBackend) ListPanes(session, window string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	return b.runCommandWithSession(session, "action", "dump-screen", "/dev/stdout")
}

func (b *ZellijBackend) KillPane(session, window, pane string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	if err := b.focusPane(session, window, pane); err != nil {
		return err
	}
	return b.runCommandWithSession(session, "action", "close-pane")
}

func (b *ZellijBackend) ResizePane(session, window, pane, direction string, size int) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	if err := b.focusPane(session, window, pane); err != nil {
		return err
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
	for i := 0; i < size; i++ {
		if err := b.runCommandWithSession(session, "action", "resize", dir); err != nil {
			return err
		}
	}
	return nil
}

func (b *ZellijBackend) SendKeys(session, window, pane, keys string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist", session)
	}
	if err := b.focusPane(session, window, pane); err != nil {
		return err
	}
	return b.runCommandWithSession(session, "action", "write-chars", keys)
}

func (b *ZellijBackend) DetachSession() error {
	return fmt.Errorf("detach operation not supported in zellij")
}

func (b *ZellijBackend) NukeAllSessions() error {
	if err := b.runCommand("delete-all-sessions", "-y", "-f"); err == nil {
		return nil
	}

	output, err := b.runCommandOutput("list-sessions")
	if err != nil {
		return fmt.Errorf("failed to list zellij sessions: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	killCount := 0
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cleanLine := re.ReplaceAllString(line, "")
		parts := strings.Fields(cleanLine)
		if len(parts) > 0 {
			sessionName := parts[0]
			if err := b.runCommand("delete-session", sessionName); err != nil {
				// Best-effort force delete; ignore secondary error
				_ = b.runCommand("delete-session", "--force", sessionName)
			}
			killCount++
		}
	}

	if killCount == 0 {
		return fmt.Errorf("no zellij sessions found to delete")
	}

	return nil
}
