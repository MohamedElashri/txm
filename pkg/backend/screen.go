package backend

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type ScreenBackend struct{}

func NewScreenBackend() *ScreenBackend {
	return &ScreenBackend{}
}

func (b *ScreenBackend) Name() string {
	return "screen"
}

func (b *ScreenBackend) IsAvailable() bool {
	if _, err := exec.LookPath("screen"); err == nil {
		return true
	}
	return false
}

func (b *ScreenBackend) runCommand(args ...string) error {
	cmd := exec.Command("screen", args...)
	preserveEnvironment(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (b *ScreenBackend) SessionExists(name string) bool {
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

func (b *ScreenBackend) CreateSession(name string) error {
	return b.runCommand("-dmS", name)
}

func (b *ScreenBackend) ListSessions() error {
	return b.runCommand("-ls")
}

func (b *ScreenBackend) AttachSession(name string) error {
	return b.runCommand("-r", name)
}

func (b *ScreenBackend) DetachSession() error {
	return fmt.Errorf("screen detach must be done manually with Ctrl-A d")
}

func (b *ScreenBackend) KillSession(name string) error {
	cmd := exec.Command("screen", "-ls")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "."+name) && (strings.Contains(line, "(Attached)") || strings.Contains(line, "(Detached)")) {
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) > 0 {
				sessionID := parts[0]
				if strings.Contains(sessionID, ".") {
					// Best-effort kill; ignore error since screen may have already exited
					_ = exec.Command("screen", "-S", sessionID, "-X", "quit").Run()
				}
			}
		}
	}
	return nil
}

func (b *ScreenBackend) RenameSession(oldName, newName string) error {
	return fmt.Errorf("screen does not support session renaming")
}

func (b *ScreenBackend) NewWindow(session, name string) error {
	return b.runCommand("-S", session, "-X", "screen", "-t", name)
}

func (b *ScreenBackend) ListWindows(session string) error {
	return b.runCommand("-S", session, "-X", "windows")
}

func (b *ScreenBackend) KillWindow(session, window string) error {
	return fmt.Errorf("screen does not support killing windows by name")
}

func (b *ScreenBackend) NextWindow(session string) error {
	return b.runCommand("-S", session, "-X", "next")
}

func (b *ScreenBackend) PreviousWindow(session string) error {
	return b.runCommand("-S", session, "-X", "prev")
}

func (b *ScreenBackend) RenameWindow(session, oldName, newName string) error {
	return b.runCommand("-S", session, "-X", "title", newName)
}

func (b *ScreenBackend) MoveWindow(srcSession, windowName, dstSession string) error {
	return fmt.Errorf("screen does not support moving windows between sessions")
}

func (b *ScreenBackend) SwapWindow(session, windowName1, windowName2 string) error {
	return fmt.Errorf("screen does not support swapping windows")
}

func (b *ScreenBackend) SplitWindow(session, window, direction string) error {
	if direction == "v" {
		return b.runCommand("-S", session, "-X", "split", "-v")
	}
	return fmt.Errorf("horizontal splitting not supported in screen")
}

func (b *ScreenBackend) ListPanes(session, window string) error {
	return fmt.Errorf("screen does not support listing panes")
}

func (b *ScreenBackend) KillPane(session, window, pane string) error {
	return fmt.Errorf("screen does not support killing individual panes")
}

func (b *ScreenBackend) ResizePane(session, window, pane, direction string, size int) error {
	return fmt.Errorf("screen does not support resizing panes")
}

func (b *ScreenBackend) SendKeys(session, window, pane, keys string) error {
	return fmt.Errorf("screen does not support sending keys to specific panes")
}

func (b *ScreenBackend) NukeAllSessions() error {
	cmd := exec.Command("screen", "-ls")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "(Attached)") || strings.Contains(line, "(Detached)") {
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) > 0 {
				sessionID := parts[0]
				if strings.Contains(sessionID, ".") {
					colorCodeRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
					sessionID = colorCodeRegex.ReplaceAllString(sessionID, "")
					// Best-effort kill; ignore error since screen may have already exited
					_ = exec.Command("screen", "-S", sessionID, "-X", "quit").Run()
				}
			}
		}
	}
	return nil
}
