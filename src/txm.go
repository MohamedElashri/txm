package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const (
	colorRed     = "\x1b[31m"
	colorGreen   = "\x1b[32m"
	colorYellow  = "\x1b[33m"
	colorBlue    = "\x1b[34m"
	colorMagenta = "\x1b[35m"
	colorReset   = "\x1b[0m"
)

type SessionManager struct {
	tmuxAvailable   bool
	zellijAvailable bool
	useColors       bool
	verbose         bool
	config          *Config
	currentBackend  Backend
}

func NewSessionManager(verbose bool) *SessionManager {
	colorSupport := checkColorSupport(verbose)

	// Load configuration
	config, err := LoadConfig()
	if err != nil && verbose {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		config = NewDefaultConfig()
	}

	sm := &SessionManager{
		tmuxAvailable:   checkTmuxAvailable(),
		zellijAvailable: checkZellijAvailable(),
		useColors:       colorSupport,
		verbose:         verbose,
		config:          config,
	}

	// Determine the best available backend based on config and availability
	sm.currentBackend = sm.selectBestBackend()

	if verbose {
		fmt.Fprintf(os.Stderr, "SessionManager initialized with useColors=%v, backend=%v\n", sm.useColors, sm.currentBackend)
	}

	return sm
}

func checkColorSupport(verbose bool) bool {
	if os.Getenv("NO_COLOR") != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "Colors disabled due to NO_COLOR environment variable\n")
		}
		return false
	}

	termEnv := os.Getenv("TERM")
	if termEnv == "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "No TERM environment variable found\n")
		}
		return false
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		if verbose {
			fmt.Fprintf(os.Stderr, "Output is not going to a terminal\n")
		}
		return false
	}

	colorTerms := []string{"xterm", "xterm-256color", "screen", "screen-256color", "tmux", "tmux-256color", "linux"}
	for _, colorTerm := range colorTerms {
		if strings.HasPrefix(termEnv, colorTerm) {
			if verbose {
				fmt.Fprintf(os.Stderr, "Color support detected for TERM=%s\n", termEnv)
			}
			return true
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "TERM=%s doesn't appear to support colors\n", termEnv)
	}
	return false
}

func (sm *SessionManager) colorize(color, text string) string {
	if sm.useColors {
		result := color + text + colorReset
		if sm.verbose {
			fmt.Fprintf(os.Stderr, "Colorizing: input=%q, with_color=%q\n",
				text, result)
		}
		return result
	}
	if sm.verbose {
		fmt.Fprintf(os.Stderr, "Colors disabled for text: %q\n", text)
	}
	return text
}

func (sm *SessionManager) logInfo(msg string) {
	prefix := sm.colorize(colorGreen, "[INFO]")
	fmt.Printf("%s %s\n", prefix, msg)
}

func (sm *SessionManager) logWarning(msg string) {
	prefix := sm.colorize(colorYellow, "[WARNING]")
	fmt.Printf("%s %s\n", prefix, msg)
}

func (sm *SessionManager) logError(msg string) {
	prefix := sm.colorize(colorRed, "[ERROR]")
	fmt.Printf("%s %s\n", prefix, msg)
}

func (sm *SessionManager) sessionExists(name string) bool {
	switch sm.currentBackend {
	case BackendTmux:
		return sm.tmuxSessionExists(name)
	case BackendZellij:
		return sm.zellijSessionExists(name)
	case BackendScreen:
		return sm.screenSessionExists(name)
	default:
		return false
	}
}

func (sm *SessionManager) createSession(name string) {
	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.tmuxCreateSession(name); err == nil {
			sm.logInfo(fmt.Sprintf("Session '%s' created with tmux", name))
			return
		}
		sm.logError(fmt.Sprintf("Failed to create tmux session '%s'", name))
		return
	case BackendZellij:
		if err := sm.zellijCreateSession(name); err == nil {
			sm.logInfo(fmt.Sprintf("Session '%s' created with zellij", name))
			return
		}
		sm.logError(fmt.Sprintf("Failed to create zellij session '%s'", name))
		return
	case BackendScreen:
		if err := sm.screenCreateSession(name); err == nil {
			sm.logInfo(fmt.Sprintf("Session '%s' created with screen", name))
			return
		}
		sm.logError(fmt.Sprintf("Failed to create screen session '%s'", name))
		return
	default:
		sm.logError("No available backend to create session")
	}
}

func (sm *SessionManager) listSessions() {
	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.tmuxListSessions(); err != nil {
			sm.logWarning("No tmux sessions found")
		}
		return
	case BackendZellij:
		if err := sm.zellijListSessions(); err != nil {
			sm.logWarning("No zellij sessions found")
		}
		return
	case BackendScreen:
		if err := sm.screenListSessions(); err != nil {
			sm.logWarning("No screen sessions found")
		}
		return
	default:
		sm.logError("No available backend to list sessions")
	}
}

func (sm *SessionManager) newWindow(session, name string) {
	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.tmuxNewWindow(session, name); err != nil {
			sm.logError(fmt.Sprintf("Failed to create window '%s' in tmux session '%s'", name, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Window '%s' created in tmux session '%s'", name, session))
		return
	case BackendZellij:
		if err := sm.zellijNewWindow(session, name); err != nil {
			sm.logError(fmt.Sprintf("Failed to create tab '%s' in zellij session '%s'", name, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Tab '%s' created in zellij session '%s'", name, session))
		return
	case BackendScreen:
		if err := sm.screenNewWindow(session, name); err != nil {
			sm.logError(fmt.Sprintf("Failed to create window in screen session '%s'", session))
			return
		}
		if err := sm.screenRenameWindow(session, "", name); err != nil {
			sm.logWarning(fmt.Sprintf("Created window but failed to rename it in screen session '%s'", session))
			return
		}
		sm.logInfo(fmt.Sprintf("Window '%s' created in screen session '%s'", name, session))
		return
	default:
		sm.logError("No available backend to create window")
	}
}

func (sm *SessionManager) listWindows(session string) {
	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.runTmuxCommand("list-windows", "-t", session); err != nil {
			sm.logError(fmt.Sprintf("Failed to list windows in tmux session '%s'", session))
		}
		return
	case BackendZellij:
		if err := sm.zellijListWindows(session); err != nil {
			sm.logError(fmt.Sprintf("Failed to list tabs in zellij session '%s'", session))
		}
		return
	case BackendScreen:
		if err := sm.screenListWindows(session); err != nil {
			sm.logError(fmt.Sprintf("Failed to list windows in screen session '%s'", session))
		}
		return
	default:
		sm.logError("No available backend to list windows")
	}
}

func (sm *SessionManager) killWindow(session, window string) {
	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.runTmuxCommand("kill-window", "-t", fmt.Sprintf("%s:%s", session, window)); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill window '%s' in tmux session '%s'", window, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Window '%s' killed in tmux session '%s'", window, session))
		return
	case BackendZellij:
		if err := sm.zellijKillWindow(session, window); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill tab '%s' in zellij session '%s'", window, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Tab '%s' killed in zellij session '%s'", window, session))
		return
	case BackendScreen:
		if err := sm.screenKillWindow(session, window); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill window in screen session '%s'", session))
			return
		}
		sm.logInfo(fmt.Sprintf("Window killed in screen session '%s'", session))
		return
	default:
		sm.logError("No available backend to kill window")
	}
}

func (sm *SessionManager) nextWindow(session string) {
	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.runTmuxCommand("next-window", "-t", session); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to next window in tmux session '%s'", session))
			return
		}
	case BackendZellij:
		if err := sm.zellijNextWindow(session); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to next tab in zellij session '%s'", session))
			return
		}
	case BackendScreen:
		if err := sm.runScreenCommand("-S", session, "-X", "next"); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to next window in screen session '%s'", session))
			return
		}
	default:
		sm.logError("No available backend to switch windows")
		return
	}
	sm.logInfo("Switched to next window")
}

func (sm *SessionManager) previousWindow(session string) {
	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.runTmuxCommand("previous-window", "-t", session); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to previous window in tmux session '%s'", session))
			return
		}
	case BackendZellij:
		if err := sm.zellijPreviousWindow(session); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to previous tab in zellij session '%s'", session))
			return
		}
	case BackendScreen:
		if err := sm.runScreenCommand("-S", session, "-X", "prev"); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to previous window in screen session '%s'", session))
			return
		}
	default:
		sm.logError("No available backend to switch windows")
		return
	}
	sm.logInfo("Switched to previous window")
}

func (sm *SessionManager) attachSession(name string) {
	if !sm.sessionExists(name) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", name))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.runTmuxCommand("attach-session", "-t", name); err != nil {
			sm.logError(fmt.Sprintf("Failed to attach to tmux session '%s'", name))
			return
		}
		return
	case BackendZellij:
		if err := sm.zellijAttachSession(name); err != nil {
			sm.logError(fmt.Sprintf("Failed to attach to zellij session '%s'", name))
			return
		}
		return
	case BackendScreen:
		if err := sm.runScreenCommand("-r", name); err != nil {
			sm.logError(fmt.Sprintf("Failed to attach to screen session '%s'", name))
			return
		}
		return
	default:
		sm.logError("No available backend to attach to session")
	}
}

func (sm *SessionManager) detachSession() {
	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.runTmuxCommand("detach-client"); err != nil {
			sm.logError("Failed to detach from tmux session")
			return
		}
		sm.logInfo("Detached from tmux session")
		return
	case BackendZellij:
		if err := sm.zellijDetachSession(); err != nil {
			sm.logError("Failed to detach from zellij session")
			return
		}
		sm.logInfo("Detached from zellij session")
		return
	case BackendScreen:
		if err := sm.runScreenCommand("-d"); err != nil {
			sm.logError("Failed to detach from screen session")
			return
		}
		sm.logInfo("Detached from screen session")
		return
	default:
		sm.logError("No available backend to detach from session")
	}
}

func (sm *SessionManager) killSession(name string) {
	switch sm.currentBackend {
	case BackendTmux:
		if !sm.sessionExists(name) {
			sm.logError(fmt.Sprintf("Session '%s' does not exist", name))
			return
		}
		if err := sm.runTmuxCommand("kill-session", "-t", name); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill tmux session '%s'", name))
			return
		}
		sm.logInfo(fmt.Sprintf("Killed tmux session '%s'", name))
		return
	case BackendZellij:
		if !sm.sessionExists(name) {
			sm.logError(fmt.Sprintf("Session '%s' does not exist", name))
			return
		}
		if err := sm.zellijKillSession(name); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill zellij session '%s'", name))
			return
		}
		sm.logInfo(fmt.Sprintf("Killed zellij session '%s'", name))
		return
	case BackendScreen:
		// For screen, handle multiple sessions with the same name by killing all matching ones
		cmd := exec.Command("screen", "-ls")
		output, err := cmd.Output()
		if err != nil {
			sm.logError(fmt.Sprintf("Session '%s' does not exist", name))
			return
		}
		
		outputStr := string(output)
		lines := strings.Split(outputStr, "\n")
		killCount := 0
		
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "(") && strings.Contains(line, ")") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					sessionPart := parts[0]
					if strings.Contains(sessionPart, ".") {
						sessionName := strings.SplitN(sessionPart, ".", 2)[1]
						if sessionName == name {
							// Use the full session ID (pid.name) to avoid ambiguity
							if err := sm.runScreenCommand("-X", "-S", sessionPart, "quit"); err == nil {
								killCount++
							}
						}
					}
				}
			}
		}
		
		if killCount > 0 {
			sm.logInfo(fmt.Sprintf("Killed %d screen session(s) named '%s'", killCount, name))
		} else {
			sm.logError(fmt.Sprintf("Session '%s' does not exist", name))
		}
		return
	default:
		sm.logError("No available backend to kill session")
	}
}

func (sm *SessionManager) renameSession(oldName, newName string) {
	if !sm.sessionExists(oldName) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", oldName))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.runTmuxCommand("rename-session", "-t", oldName, newName); err != nil {
			sm.logError(fmt.Sprintf("Failed to rename tmux session from '%s' to '%s'", oldName, newName))
			return
		}
		sm.logInfo(fmt.Sprintf("Renamed tmux session from '%s' to '%s'", oldName, newName))
		return
	case BackendZellij:
		sm.logError("Session renaming is not supported in zellij")
		return
	case BackendScreen:
		sm.logError("Session renaming is not supported in GNU Screen")
		return
	default:
		sm.logError("No available backend to rename session")
	}
}

func (sm *SessionManager) renameWindow(session, oldName, newName string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		windowTarget := fmt.Sprintf("%s:%s", session, oldName)
		if err := sm.runTmuxCommand("rename-window", "-t", windowTarget, newName); err != nil {
			sm.logError(fmt.Sprintf("Failed to rename window from '%s' to '%s' in tmux session '%s'", oldName, newName, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Renamed window from '%s' to '%s' in tmux session '%s'", oldName, newName, session))
		return
	case BackendZellij:
		if err := sm.zellijRenameWindow(session, oldName, newName); err != nil {
			sm.logError(fmt.Sprintf("Failed to rename tab in zellij session '%s': %v", session, err))
			return
		}
		sm.logInfo(fmt.Sprintf("Renamed tab from '%s' to '%s' in zellij session '%s'", oldName, newName, session))
		return
	case BackendScreen:
		if err := sm.runScreenCommand("-S", session, "-X", "title", newName); err != nil {
			sm.logError(fmt.Sprintf("Failed to rename window in screen session '%s'", session))
			return
		}
		sm.logInfo(fmt.Sprintf("Renamed window to '%s' in screen session '%s'", newName, session))
	default:
		sm.logError(fmt.Sprintf("Backend %v does not support window renaming", sm.currentBackend))
	}
}

func (sm *SessionManager) moveWindow(srcSession, windowName, dstSession string) {
	if !sm.sessionExists(srcSession) || !sm.sessionExists(dstSession) {
		sm.logError(fmt.Sprintf("Either source session '%s' or destination session '%s' does not exist", srcSession, dstSession))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		windowTarget := fmt.Sprintf("%s:%s", srcSession, windowName)
		if err := sm.runTmuxCommand("move-window", "-s", windowTarget, "-t", dstSession); err != nil {
			sm.logError(fmt.Sprintf("Failed to move window '%s' from session '%s' to session '%s'", windowName, srcSession, dstSession))
			return
		}
		sm.logInfo(fmt.Sprintf("Moved window '%s' from session '%s' to session '%s'", windowName, srcSession, dstSession))
		return
	case BackendZellij:
		sm.logError("Moving windows between sessions is not supported in zellij")
		return
	case BackendScreen:
		sm.logError("Moving windows between sessions is not supported in GNU Screen")
	default:
		sm.logError(fmt.Sprintf("Backend %v does not support moving windows", sm.currentBackend))
	}
}

func (sm *SessionManager) swapWindow(session, windowName1, windowName2 string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		windowTarget1 := fmt.Sprintf("%s:%s", session, windowName1)
		windowTarget2 := fmt.Sprintf("%s:%s", session, windowName2)
		if err := sm.runTmuxCommand("swap-window", "-s", windowTarget1, "-t", windowTarget2); err != nil {
			sm.logError(fmt.Sprintf("Failed to swap windows '%s' and '%s' in tmux session '%s'", windowName1, windowName2, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Swapped windows '%s' and '%s' in tmux session '%s'", windowName1, windowName2, session))
		return
	case BackendZellij:
		sm.logError("Swapping windows is not supported in zellij")
		return
	case BackendScreen:
		sm.logError("Swapping windows is not supported in GNU Screen")
	default:
		sm.logError(fmt.Sprintf("Backend %v does not support swapping windows", sm.currentBackend))
	}
}

func (sm *SessionManager) splitWindow(session, window, direction string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		windowTarget := fmt.Sprintf("%s:%s", session, window)
		splitFlag := "-v" // vertical split (one pane above another)
		if direction == "h" {
			splitFlag = "-h" // horizontal split (panes side by side)
		}

		if err := sm.runTmuxCommand("split-window", splitFlag, "-t", windowTarget); err != nil {
			sm.logError(fmt.Sprintf("Failed to split window '%s' in tmux session '%s'", window, session))
			return
		}
		splitDesc := "vertically"
		if direction == "h" {
			splitDesc = "horizontally"
		}
		sm.logInfo(fmt.Sprintf("Split window '%s' %s in tmux session '%s'", window, splitDesc, session))
		return
	case BackendZellij:
		if err := sm.zellijSplitWindow(session, window, direction); err != nil {
			sm.logError(fmt.Sprintf("Failed to split pane in zellij session '%s': %v", session, err))
			return
		}
		splitDesc := "vertically"
		if direction == "h" {
			splitDesc = "horizontally"
		}
		sm.logInfo(fmt.Sprintf("Split pane %s in zellij session '%s'", splitDesc, session))
		return
	case BackendScreen:
		if direction == "v" {
			if err := sm.runScreenCommand("-S", session, "-X", "split"); err != nil {
				sm.logError(fmt.Sprintf("Failed to split window in screen session '%s'", session))
				return
			}
			sm.logInfo(fmt.Sprintf("Split window vertically in screen session '%s'", session))
			return
		}
		sm.logError("Horizontal window splitting is not supported in GNU Screen")
	default:
		sm.logError(fmt.Sprintf("Backend %v does not support window splitting", sm.currentBackend))
	}
}

func (sm *SessionManager) listPanes(session, window string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		windowTarget := fmt.Sprintf("%s:%s", session, window)
		if err := sm.runTmuxCommand("list-panes", "-t", windowTarget); err != nil {
			sm.logError(fmt.Sprintf("Failed to list panes in window '%s' of tmux session '%s'", window, session))
			return
		}
		return
	case BackendZellij:
		if err := sm.zellijListPanes(session, window); err != nil {
			sm.logError(fmt.Sprintf("Failed to list panes in zellij session '%s': %v", session, err))
			return
		}
		return
	case BackendScreen:
		sm.logError("Listing panes is not supported in GNU Screen")
	default:
		sm.logError(fmt.Sprintf("Backend %v does not support listing panes", sm.currentBackend))
	}
}

func (sm *SessionManager) killPane(session, window, pane string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
		if err := sm.runTmuxCommand("kill-pane", "-t", paneTarget); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Killed pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
		return
	case BackendZellij:
		if err := sm.zellijKillPane(session, window, pane); err != nil {
			sm.logError(fmt.Sprintf("Failed to close pane in zellij session '%s': %v", session, err))
			return
		}
		sm.logInfo(fmt.Sprintf("Closed pane in zellij session '%s'", session))
		return
	case BackendScreen:
		sm.logError("Killing panes is not supported in GNU Screen")
	default:
		sm.logError(fmt.Sprintf("Backend %v does not support killing panes", sm.currentBackend))
	}
}

func (sm *SessionManager) resizePane(session, window, pane, direction string, size int) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
		resizeFlag := "-U" // up
		if direction == "D" {
			resizeFlag = "-D" // down
		} else if direction == "L" {
			resizeFlag = "-L" // left
		} else if direction == "R" {
			resizeFlag = "-R" // right
		}

		if err := sm.runTmuxCommand("resize-pane", resizeFlag, fmt.Sprintf("%d", size), "-t", paneTarget); err != nil {
			sm.logError(fmt.Sprintf("Failed to resize pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Resized pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
		return
	case BackendZellij:
		if err := sm.zellijResizePane(session, window, pane, direction, size); err != nil {
			sm.logError(fmt.Sprintf("Failed to resize pane in zellij session '%s': %v", session, err))
			return
		}
		sm.logInfo(fmt.Sprintf("Resized pane in zellij session '%s'", session))
		return
	case BackendScreen:
		sm.logError("Resizing panes is not supported in GNU Screen")
	default:
		sm.logError(fmt.Sprintf("Backend %v does not support resizing panes", sm.currentBackend))
	}
}

func (sm *SessionManager) sendKeys(session, window, pane, keys string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	switch sm.currentBackend {
	case BackendTmux:
		paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
		if err := sm.runTmuxCommand("send-keys", "-t", paneTarget, keys); err != nil {
			sm.logError(fmt.Sprintf("Failed to send keys to pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Sent keys to pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
		return
	case BackendZellij:
		if err := sm.zellijSendKeys(session, window, pane, keys); err != nil {
			sm.logError(fmt.Sprintf("Failed to send keys to zellij session '%s': %v", session, err))
			return
		}
		sm.logInfo(fmt.Sprintf("Sent keys to zellij session '%s'", session))
		return
	case BackendScreen:
		sm.logError("Sending keys to panes is not supported in GNU Screen")
	default:
		sm.logError(fmt.Sprintf("Backend %v does not support sending keys to panes", sm.currentBackend))
	}
}

func (sm *SessionManager) nukeAllSessions() {
	switch sm.currentBackend {
	case BackendTmux:
		if err := sm.runTmuxCommand("kill-server"); err != nil {
			sm.logError("Failed to kill all tmux sessions")
			return
		}
		sm.logInfo("Killed all tmux sessions")
		return
	case BackendZellij:
		if err := sm.zellijNukeAllSessions(); err != nil {
			sm.logError("Failed to kill all zellij sessions")
			return
		}
		sm.logInfo("Killed all zellij sessions")
		return
	case BackendScreen:
		// For screen, we need to parse the output of screen -ls and kill each session
		cmd := exec.Command("screen", "-ls")
		output, err := cmd.Output()
		if err != nil {
			sm.logError("No screen sessions found")
			return
		}

		// Parse the output to find session names
		outputStr := string(output)
		lines := strings.Split(outputStr, "\n")
		killCount := 0

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "(") && strings.Contains(line, ")") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					sessionName := strings.Split(parts[0], ".")[0]
					if err := sm.runScreenCommand("-X", "-S", sessionName, "quit"); err == nil {
						killCount++
					}
				}
			}
		}

		if killCount > 0 {
			sm.logInfo(fmt.Sprintf("Killed %d screen sessions", killCount))
		} else {
			sm.logWarning("No screen sessions were killed")
		}
		return
	default:
		sm.logError("No available backend to nuke sessions")
	}
}

func main() {
	var verbose bool

	for _, arg := range os.Args {
		if arg == "-v" || arg == "--verbose" {
			verbose = true
			newArgs := make([]string, 0)
			for _, a := range os.Args {
				if a != "-v" && a != "--verbose" {
					newArgs = append(newArgs, a)
				}
			}
			os.Args = newArgs
			break
		}
	}

	sm := NewSessionManager(verbose)

	// Check if any backend is available
	if !sm.isBackendAvailable(sm.currentBackend) {
		// Check if any backends are available at all
		hasAnyBackend := sm.tmuxAvailable || sm.zellijAvailable
		if _, err := exec.LookPath("screen"); err == nil {
			hasAnyBackend = true
		}
		
		if !hasAnyBackend {
			sm.logError("None of tmux, zellij, or screen is installed. Please install at least one and try again.")
			os.Exit(1)
		}
		sm.logWarning(fmt.Sprintf("Configured backend '%s' is not available. Using fallback.", sm.currentBackend))
	}

	// Show current backend info in verbose mode
	if verbose {
		availableBackends := []string{}
		if sm.tmuxAvailable {
			availableBackends = append(availableBackends, "tmux")
		}
		if sm.zellijAvailable {
			availableBackends = append(availableBackends, "zellij")
		}
		if _, err := exec.LookPath("screen"); err == nil {
			availableBackends = append(availableBackends, "screen")
		}
		sm.logInfo(fmt.Sprintf("Available backends: %s", strings.Join(availableBackends, ", ")))
		sm.logInfo(fmt.Sprintf("Using backend: %s", sm.currentBackend))
	}

	if len(os.Args) < 2 {
		displayHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		name := getArg(2, "")
		if name == "" {
			sm.logError("Please specify a session name")
			displayHelp()
			os.Exit(1)
		}
		sm.createSession(name)

	case "list":
		sm.listSessions()

	case "attach":
		name := getArg(2, "")
		if name == "" {
			sm.logError("Please specify a session name")
			displayHelp()
			os.Exit(1)
		}
		sm.attachSession(name)

	case "detach":
		sm.detachSession()

	case "delete":
		name := getArg(2, "")
		if name == "" {
			sm.logError("Please specify a session name")
			displayHelp()
			os.Exit(1)
		}
		sm.killSession(name)

	case "nuke":
		sm.nukeAllSessions()

	case "new-window":
		session := getArg(2, "")
		name := getArg(3, "")
		if session == "" || name == "" {
			sm.logError("Please specify both session name and window name")
			displayHelp()
			os.Exit(1)
		}
		sm.newWindow(session, name)

	case "list-windows":
		session := getArg(2, "")
		if session == "" {
			sm.logError("Please specify a session name")
			displayHelp()
			os.Exit(1)
		}
		sm.listWindows(session)

	case "kill-window":
		session := getArg(2, "")
		window := getArg(3, "")
		if session == "" || window == "" {
			sm.logError("Please specify both session name and window name")
			displayHelp()
			os.Exit(1)
		}
		sm.killWindow(session, window)

	case "next-window":
		session := getArg(2, "")
		if session == "" {
			sm.logError("Please specify a session name")
			displayHelp()
			os.Exit(1)
		}
		sm.nextWindow(session)

	case "prev-window":
		session := getArg(2, "")
		if session == "" {
			sm.logError("Please specify a session name")
			displayHelp()
			os.Exit(1)
		}
		sm.previousWindow(session)

	case "rename-session":
		oldName := getArg(2, "")
		newName := getArg(3, "")
		if oldName == "" || newName == "" {
			sm.logError("Please specify both old session name and new session name")
			displayHelp()
			os.Exit(1)
		}
		sm.renameSession(oldName, newName)

	case "rename-window":
		session := getArg(2, "")
		oldName := getArg(3, "")
		newName := getArg(4, "")
		if session == "" || oldName == "" || newName == "" {
			sm.logError("Please specify session name, old window name, and new window name")
			displayHelp()
			os.Exit(1)
		}
		sm.renameWindow(session, oldName, newName)

	case "move-window":
		srcSession := getArg(2, "")
		window := getArg(3, "")
		dstSession := getArg(4, "")
		if srcSession == "" || window == "" || dstSession == "" {
			sm.logError("Please specify source session, window name, and destination session")
			displayHelp()
			os.Exit(1)
		}
		sm.moveWindow(srcSession, window, dstSession)

	case "swap-window":
		session := getArg(2, "")
		window1 := getArg(3, "")
		window2 := getArg(4, "")
		if session == "" || window1 == "" || window2 == "" {
			sm.logError("Please specify session name, first window name, and second window name")
			displayHelp()
			os.Exit(1)
		}
		sm.swapWindow(session, window1, window2)

	case "split-window":
		session := getArg(2, "")
		window := getArg(3, "")
		direction := getArg(4, "v") // Default to vertical split
		if session == "" || window == "" {
			sm.logError("Please specify session name and window name")
			displayHelp()
			os.Exit(1)
		}
		sm.splitWindow(session, window, direction)

	case "list-panes":
		session := getArg(2, "")
		window := getArg(3, "")
		if session == "" || window == "" {
			sm.logError("Please specify session name and window name")
			displayHelp()
			os.Exit(1)
		}
		sm.listPanes(session, window)

	case "kill-pane":
		session := getArg(2, "")
		window := getArg(3, "")
		pane := getArg(4, "")
		if session == "" || window == "" || pane == "" {
			sm.logError("Please specify session name, window name, and pane number")
			displayHelp()
			os.Exit(1)
		}
		sm.killPane(session, window, pane)

	case "resize-pane":
		session := getArg(2, "")
		window := getArg(3, "")
		pane := getArg(4, "")
		direction := getArg(5, "U") // Default to resize up
		size := 5                   // Default size
		sizeArg := getArg(6, "")
		if sizeArg != "" {
			var err error
			size, err = strconv.Atoi(sizeArg)
			if err != nil {
				sm.logError("Size must be a number")
				displayHelp()
				os.Exit(1)
			}
		}
		if session == "" || window == "" || pane == "" {
			sm.logError("Please specify session name, window name, pane number, and direction (U/D/L/R)")
			displayHelp()
			os.Exit(1)
		}
		sm.resizePane(session, window, pane, direction, size)

	case "send-keys":
		session := getArg(2, "")
		window := getArg(3, "")
		pane := getArg(4, "")
		keys := getArg(5, "")
		if session == "" || window == "" || pane == "" || keys == "" {
			sm.logError("Please specify session name, window name, pane number, and keys to send")
			displayHelp()
			os.Exit(1)
		}
		sm.sendKeys(session, window, pane, keys)

	case "version":
		fmt.Printf("txm version %s\n", Version)
		if len(os.Args) > 2 && os.Args[2] == "--check-update" {
			if err := CheckForUpdates(sm); err != nil {
				sm.logError(err.Error())
				os.Exit(1)
			}
		}

	case "update":
		if len(os.Args) > 2 && os.Args[2] == "--check-update" {
			if err := CheckForUpdates(sm); err != nil {
				sm.logError(err.Error())
				os.Exit(1)
			}
		} else {
			if err := UpdateBinary(sm); err != nil {
				sm.logError(err.Error())
				os.Exit(1)
			}
		}

	case "uninstall":
		if err := UninstallTxm(sm); err != nil {
			sm.logError(err.Error())
			os.Exit(1)
		}

	case "config":
		subcommand := getArg(2, "")
		switch subcommand {
		case "set":
			key := getArg(3, "")
			value := getArg(4, "")
			if key == "" || value == "" {
				sm.logError("Usage: txm config set <key> <value>")
				sm.logError("Available keys: backend")
				os.Exit(1)
			}
			if err := setConfigValue(sm, key, value); err != nil {
				sm.logError(err.Error())
				os.Exit(1)
			}
		case "get":
			key := getArg(3, "")
			if key == "" {
				sm.logError("Usage: txm config get <key>")
				sm.logError("Available keys: backend")
				os.Exit(1)
			}
			if err := getConfigValue(sm, key); err != nil {
				sm.logError(err.Error())
				os.Exit(1)
			}
		case "show":
			showCurrentConfig(sm)
		default:
			sm.logError("Usage: txm config <set|get|show>")
			sm.logError("  set <key> <value> - Set configuration value")
			sm.logError("  get <key>         - Get configuration value")
			sm.logError("  show              - Show current configuration")
			os.Exit(1)
		}

	case "help":
		displayHelp()

	default:
		sm.logError("Invalid command")
		displayHelp()
		os.Exit(1)
	}
}

func getArg(index int, defaultValue string) string {
	if index < len(os.Args) {
		return os.Args[index]
	}
	return defaultValue
}

func setConfigValue(sm *SessionManager, key, value string) error {
	switch strings.ToLower(key) {
	case "backend":
		backend, err := ParseBackend(value)
		if err != nil {
			return err
		}
		if !sm.isBackendAvailable(backend) {
			return fmt.Errorf("backend '%s' is not available on this system", value)
		}
		sm.config.DefaultBackend = backend
		if err := SaveConfig(sm.config); err != nil {
			return fmt.Errorf("failed to save config: %v", err)
		}
		sm.logInfo(fmt.Sprintf("Default backend set to '%s'", backend))
		return nil
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}
}

func getConfigValue(sm *SessionManager, key string) error {
	switch strings.ToLower(key) {
	case "backend":
		fmt.Printf("%s\n", sm.config.DefaultBackend)
		return nil
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}
}

func showCurrentConfig(sm *SessionManager) {
	sm.logInfo("Current Configuration:")
	fmt.Printf("  Default Backend: %s\n", sm.config.DefaultBackend)
	fmt.Printf("  Current Backend: %s\n", sm.currentBackend)
	
	fmt.Printf("  Available Backends: ")
	available := []string{}
	for _, backend := range []Backend{BackendTmux, BackendZellij, BackendScreen} {
		if sm.isBackendAvailable(backend) {
			available = append(available, backend.String())
		}
	}
	fmt.Printf("%s\n", strings.Join(available, ", "))
}

func displayHelp() {
	helpText := `Usage: txm [command] [arguments]

Commands:
┌────────────────┬─────────────────────────────────────────────┬─────────────────────────────────────────┐
│    Command     │               Arguments                      │              Description                 │
├────────────────┼─────────────────────────────────────────────┼─────────────────────────────────────────┤
│ create         │ [session_name]                              │ Create a new tmux or screen session      │
│ list          │                                             │ List all tmux or screen sessions         │
│ attach        │ [session_name]                              │ Attach to a tmux or screen session       │
│ detach        │                                             │ Detach from current session              │
│ delete        │ [session_name]                              │ Delete a tmux or screen session          │
│ new-window    │ [session_name] [name]                       │ Create a new window in session           │
│ list-windows  │ [session_name]                              │ List windows in a session                │
│ kill-window   │ [session_name] [name]                       │ Kill a window in session                 │
│ next-window   │ [session_name]                              │ Switch to next window in session         │
│ prev-window   │ [session_name]                              │ Switch to previous window in session     │
│ nuke          │                                             │ Kill all tmux or screen sessions         │
│ rename-session│ [old_session_name] [new_session_name]       │ Rename a tmux session                    │
│ rename-window │ [session_name] [old_window_name] [new_window_name]│ Rename a window in a tmux session     │
│ move-window   │ [src_session_name] [window_name] [dst_session_name]│ Move a window to another session     │
│ swap-window   │ [session_name] [window1_name] [window2_name] │ Swap two windows in a tmux session       │
│ split-window  │ [session_name] [window_name] [direction]     │ Split a window in a tmux session         │
│ list-panes    │ [session_name] [window_name]                │ List panes in a window of a tmux session │
│ kill-pane     │ [session_name] [window_name] [pane_number]   │ Kill a pane in a window of a tmux session│
│ resize-pane   │ [session_name] [window_name] [pane_number] [direction] [size]│ Resize a pane in a window of a tmux session│
│ send-keys     │ [session_name] [window_name] [pane_number] [keys]│ Send keys to a pane in a window of a tmux session│
│ config        │ [set|get|show] [key] [value]                │ Manage configuration                      │
│ update        │                                             │ Update txm to the latest version         │
│ uninstall     │                                             │ Uninstall txm                           │
│ version       │ [--check-update]                           │ Show version and check for updates       │
└────────────────┴─────────────────────────────────────────────┴─────────────────────────────────────────┘

Options:
  -v, --verbose    Enable verbose output

Configuration Commands:
  txm config set backend <tmux|zellij|screen>  Set default backend
  txm config get backend                       Show current default backend
  txm config show                              Show all configuration

Environment Variables:
  TXM_DEFAULT_BACKEND    Set default backend (overrides config file)

Supported Backends:
  tmux     - Primary backend with full feature support
  zellij   - Modern terminal workspace with good feature support  
  screen   - Fallback backend with limited features

Note: Feature availability varies by backend. Some advanced commands 
      are only available with specific backends.
`
	fmt.Println(helpText)
}
