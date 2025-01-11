package main

import (
	"fmt"
	"os"
	"os/exec"
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
	tmuxAvailable bool
	useColors     bool
	verbose       bool
}

func NewSessionManager(verbose bool) *SessionManager {
	colorSupport := checkColorSupport(verbose)

	sm := &SessionManager{
		tmuxAvailable: checkTmuxAvailable(),
		useColors:     colorSupport,
		verbose:       verbose,
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "SessionManager initialized with useColors=%v\n", sm.useColors)
	}

	return sm
}

func checkColorSupport(verbose bool) bool {
	// Check if NO_COLOR environment variable is set
	if os.Getenv("NO_COLOR") != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "Colors disabled due to NO_COLOR environment variable\n")
		}
		return false
	}

	// Check if TERM is set
	termEnv := os.Getenv("TERM")
	if termEnv == "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "No TERM environment variable found\n")
		}
		return false
	}

	// Check if output is going to a terminal
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		if verbose {
			fmt.Fprintf(os.Stderr, "Output is not going to a terminal\n")
		}
		return false
	}

	// Check if TERM supports colors
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

func checkTmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func (sm *SessionManager) sessionExists(name string) bool {
	if sm.tmuxAvailable {
		cmd := exec.Command("tmux", "has-session", "-t", name)
		return cmd.Run() == nil
	}
	cmd := exec.Command("screen", "-ls")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), name)
}

func (sm *SessionManager) runTmuxCommand(args ...string) error {
	cmd := exec.Command("tmux", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (sm *SessionManager) runScreenCommand(args ...string) error {
	cmd := exec.Command("screen", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (sm *SessionManager) createSession(name string) {
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("new-session", "-d", "-s", name); err == nil {
			sm.logInfo(fmt.Sprintf("Session '%s' created with tmux", name))
			return
		}
		sm.logError(fmt.Sprintf("Failed to create tmux session '%s'", name))
		return
	}

	if err := sm.runScreenCommand("-S", name, "-dm"); err == nil {
		sm.logInfo(fmt.Sprintf("Session '%s' created with screen", name))
		return
	}
	sm.logError(fmt.Sprintf("Failed to create screen session '%s'", name))
}

func (sm *SessionManager) listSessions() {
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("list-sessions"); err != nil {
			sm.logWarning("No tmux sessions found")
		}
		return
	}

	if err := sm.runScreenCommand("-ls"); err != nil {
		sm.logWarning("No screen sessions found")
	}
}

func (sm *SessionManager) newWindow(session, name string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Window operations are only supported in tmux")
		return
	}
	if err := sm.runTmuxCommand("new-window", "-t", session, "-n", name); err != nil {
		sm.logError(fmt.Sprintf("Failed to create window '%s' in session '%s'", name, session))
		return
	}
	sm.logInfo(fmt.Sprintf("Window '%s' created in session '%s'", name, session))
}

func (sm *SessionManager) listWindows(session string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Window operations are only supported in tmux")
		return
	}
	if err := sm.runTmuxCommand("list-windows", "-t", session); err != nil {
		sm.logError(fmt.Sprintf("Failed to list windows in session '%s'", session))
	}
}

func (sm *SessionManager) killWindow(session, window string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Window operations are only supported in tmux")
		return
	}
	if err := sm.runTmuxCommand("kill-window", "-t", fmt.Sprintf("%s:%s", session, window)); err != nil {
		sm.logError(fmt.Sprintf("Failed to kill window '%s' in session '%s'", window, session))
		return
	}
	sm.logInfo(fmt.Sprintf("Window '%s' killed in session '%s'", window, session))
}

func (sm *SessionManager) renameSession(session, newName string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Session rename is only supported in tmux")
		return
	}
	if err := sm.runTmuxCommand("rename-session", "-t", session, newName); err != nil {
		sm.logError(fmt.Sprintf("Failed to rename session '%s' to '%s'", session, newName))
		return
	}
	sm.logInfo(fmt.Sprintf("Session renamed from '%s' to '%s'", session, newName))
}

func (sm *SessionManager) renameWindow(session, windowIndex, newName string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Window operations are only supported in tmux")
		return
	}
	if err := sm.runTmuxCommand("rename-window", "-t", fmt.Sprintf("%s:%s", session, windowIndex), newName); err != nil {
		sm.logError(fmt.Sprintf("Failed to rename window %s in session '%s'", windowIndex, session))
		return
	}
	sm.logInfo(fmt.Sprintf("Window renamed to '%s' in session '%s'", newName, session))
}

func (sm *SessionManager) splitWindow(session, windowIndex, direction string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Window operations are only supported in tmux")
		return
	}
	flag := "-h"
	if direction == "v" {
		flag = "-v"
	}
	if err := sm.runTmuxCommand("split-window", "-t", fmt.Sprintf("%s:%s", session, windowIndex), flag); err != nil {
		sm.logError(fmt.Sprintf("Failed to split window %s in session '%s'", windowIndex, session))
		return
	}
	sm.logInfo("Window split successfully")
}

func (sm *SessionManager) listPanes(session, windowIndex string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Pane operations are only supported in tmux")
		return
	}
	if err := sm.runTmuxCommand("list-panes", "-t", fmt.Sprintf("%s:%s", session, windowIndex)); err != nil {
		sm.logError(fmt.Sprintf("Failed to list panes in window %s of session '%s'", windowIndex, session))
	}
}

func (sm *SessionManager) killPane(session, windowIndex, paneIndex string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Pane operations are only supported in tmux")
		return
	}
	target := fmt.Sprintf("%s:%s.%s", session, windowIndex, paneIndex)
	if err := sm.runTmuxCommand("kill-pane", "-t", target); err != nil {
		sm.logError(fmt.Sprintf("Failed to kill pane %s", target))
		return
	}
	sm.logInfo(fmt.Sprintf("Pane killed in session '%s', window %s", session, windowIndex))
}

func (sm *SessionManager) moveWindow(session, windowIndex, newSession string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Window operations are only supported in tmux")
		return
	}
	if err := sm.runTmuxCommand("move-window", "-s", fmt.Sprintf("%s:%s", session, windowIndex),
		"-t", fmt.Sprintf("%s:", newSession)); err != nil {
		sm.logError(fmt.Sprintf("Failed to move window %s to session '%s'", windowIndex, newSession))
		return
	}
	sm.logInfo(fmt.Sprintf("Window moved from session '%s' to '%s'", session, newSession))
}

func (sm *SessionManager) swapWindows(session, index1, index2 string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Window operations are only supported in tmux")
		return
	}
	if err := sm.runTmuxCommand("swap-window", "-s", fmt.Sprintf("%s:%s", session, index1),
		"-t", fmt.Sprintf("%s:%s", session, index2)); err != nil {
		sm.logError(fmt.Sprintf("Failed to swap windows %s and %s in session '%s'", index1, index2, session))
		return
	}
	sm.logInfo(fmt.Sprintf("Windows %s and %s swapped in session '%s'", index1, index2, session))
}

func (sm *SessionManager) resizePane(session, windowIndex, paneIndex, resizeOption string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Pane operations are only supported in tmux")
		return
	}
	target := fmt.Sprintf("%s:%s.%s", session, windowIndex, paneIndex)
	if err := sm.runTmuxCommand("resize-pane", "-t", target, resizeOption); err != nil {
		sm.logError(fmt.Sprintf("Failed to resize pane %s", target))
		return
	}
	sm.logInfo("Pane resized successfully")
}

func (sm *SessionManager) sendKeys(session, windowIndex, paneIndex, keys string) {
	if !sm.tmuxAvailable {
		sm.logWarning("Key sending is only supported in tmux")
		return
	}
	target := fmt.Sprintf("%s:%s.%s", session, windowIndex, paneIndex)
	if err := sm.runTmuxCommand("send-keys", "-t", target, keys); err != nil {
		sm.logError(fmt.Sprintf("Failed to send keys to pane %s", target))
		return
	}
	sm.logInfo("Keys sent successfully")
}

func (sm *SessionManager) attachSession(name string) {
	if !sm.sessionExists(name) {
		sm.logWarning(fmt.Sprintf("Session '%s' does not exist", name))
		return
	}

	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("attach-session", "-t", name); err != nil {
			sm.logError(fmt.Sprintf("Failed to attach to tmux session '%s'", name))
		}
		return
	}

	if err := sm.runScreenCommand("-r", name); err != nil {
		sm.logError(fmt.Sprintf("Failed to attach to screen session '%s'", name))
	}
}

func (sm *SessionManager) detachSession() {
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("detach-client"); err != nil {
			sm.logError("Failed to detach. Are you in a tmux session?")
		}
		return
	}
	sm.logWarning("Detach is not supported for screen sessions from this tool. Use Ctrl-a d to detach.")
}

func (sm *SessionManager) killSession(name string) {
	if !sm.sessionExists(name) {
		sm.logWarning(fmt.Sprintf("Session '%s' does not exist", name))
		return
	}

	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("kill-session", "-t", name); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill tmux session '%s'", name))
		}
		return
	}

	if err := sm.runScreenCommand("-S", name, "-X", "quit"); err != nil {
		sm.logError(fmt.Sprintf("Failed to kill screen session '%s'", name))
	}
}

func (sm *SessionManager) nukeAllSessions() {
	if sm.tmuxAvailable {
		output, err := exec.Command("tmux", "list-sessions", "-F", "#S").Output()
		if err == nil {
			sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, session := range sessions {
				sm.runTmuxCommand("kill-session", "-t", session)
			}
			sm.logInfo("All tmux sessions have been nuked")
			return
		}
		sm.logWarning("No tmux sessions found to nuke")
		return
	}

	output, err := exec.Command("screen", "-ls").Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, ".") {
				session := strings.Fields(line)[0]
				sm.runScreenCommand("-S", session, "-X", "quit")
			}
		}
		sm.logInfo("All screen sessions have been nuked")
		return
	}
	sm.logWarning("No screen sessions found to nuke")
}

func main() {
	var verbose bool

	// Check for verbose flag before processing other arguments
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--verbose" {
			verbose = true
			// Remove the verbose flag from args
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

	if !sm.tmuxAvailable {
		if _, err := exec.LookPath("screen"); err != nil {
			sm.logError("Neither tmux nor screen is installed. Please install one of them and try again.")
			os.Exit(1)
		}
		sm.logWarning("tmux is not installed. Falling back to screen.")
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

	case "rename-session":
		session := getArg(2, "")
		newName := getArg(3, "")
		if session == "" || newName == "" {
			sm.logError("Please specify both current and new session names")
			displayHelp()
			os.Exit(1)
		}
		sm.renameSession(session, newName)

	case "rename-window":
		session := getArg(2, "")
		windowIndex := getArg(3, "")
		newName := getArg(4, "")
		if session == "" || windowIndex == "" || newName == "" {
			sm.logError("Please specify session name, window index, and new name")
			displayHelp()
			os.Exit(1)
		}
		sm.renameWindow(session, windowIndex, newName)

	case "split-window":
		session := getArg(2, "")
		windowIndex := getArg(3, "")
		direction := getArg(4, "")
		if session == "" || windowIndex == "" || direction == "" {
			sm.logError("Please specify session name, window index, and direction (v/h)")
			displayHelp()
			os.Exit(1)
		}
		if direction != "v" && direction != "h" {
			sm.logError("Direction must be 'v' for vertical or 'h' for horizontal")
			os.Exit(1)
		}
		sm.splitWindow(session, windowIndex, direction)

	case "list-panes":
		session := getArg(2, "")
		windowIndex := getArg(3, "")
		if session == "" || windowIndex == "" {
			sm.logError("Please specify session name and window index")
			displayHelp()
			os.Exit(1)
		}
		sm.listPanes(session, windowIndex)

	case "kill-pane":
		session := getArg(2, "")
		windowIndex := getArg(3, "")
		paneIndex := getArg(4, "")
		if session == "" || windowIndex == "" || paneIndex == "" {
			sm.logError("Please specify session name, window index, and pane index")
			displayHelp()
			os.Exit(1)
		}
		sm.killPane(session, windowIndex, paneIndex)

	case "move-window":
		session := getArg(2, "")
		windowIndex := getArg(3, "")
		newSession := getArg(4, "")
		if session == "" || windowIndex == "" || newSession == "" {
			sm.logError("Please specify source session, window index, and target session")
			displayHelp()
			os.Exit(1)
		}
		sm.moveWindow(session, windowIndex, newSession)

	case "swap-window":
		session := getArg(2, "")
		index1 := getArg(3, "")
		index2 := getArg(4, "")
		if session == "" || index1 == "" || index2 == "" {
			sm.logError("Please specify session name and both window indices")
			displayHelp()
			os.Exit(1)
		}
		sm.swapWindows(session, index1, index2)

	case "resize-pane":
		session := getArg(2, "")
		windowIndex := getArg(3, "")
		paneIndex := getArg(4, "")
		resizeOption := getArg(5, "")
		if session == "" || windowIndex == "" || paneIndex == "" || resizeOption == "" {
			sm.logError("Please specify session, window index, pane index, and resize option")
			displayHelp()
			os.Exit(1)
		}
		sm.resizePane(session, windowIndex, paneIndex, resizeOption)

	case "send-keys":
		session := getArg(2, "")
		windowIndex := getArg(3, "")
		paneIndex := getArg(4, "")
		keys := getArg(5, "")
		if session == "" || windowIndex == "" || paneIndex == "" || keys == "" {
			sm.logError("Please specify session, window index, pane index, and keys")
			displayHelp()
			os.Exit(1)
		}
		sm.sendKeys(session, windowIndex, paneIndex, keys)

	case "nuke":
		sm.nukeAllSessions()

	case "version":
		fmt.Printf("txm version %s\n", Version)
		if len(os.Args) > 2 && os.Args[2] == "--check-update" {
			if err := CheckForUpdates(sm); err != nil {
				sm.logError(err.Error())
				os.Exit(1)
			}
		}

	case "update":
		if err := UpdateBinary(sm); err != nil {
			sm.logError(err.Error())
			os.Exit(1)
		}

	case "uninstall":
		if err := UninstallTxm(sm); err != nil {
			sm.logError(err.Error())
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

func displayHelp() {
	helpText := `Usage: txm [command] [arguments]

Commands:
┌────────────────┬─────────────────────────────────────────────┬─────────────────────────────────────────┐
│    Command     │               Arguments                      │              Description                 │
├────────────────┼─────────────────────────────────────────────┼─────────────────────────────────────────┤
│ create         │ [session_name]                              │ Create a new tmux or screen session      │
│ list          │                                             │ List all tmux or screen sessions         │
│ attach        │ [session_name]                              │ Attach to a tmux or screen session       │
│ detach        │                                             │ Detach from current tmux session         │
│ delete        │ [session_name]                              │ Delete a tmux or screen session          │
│ new-window    │ [session_name] [name]                       │ Create a new window in tmux session      │
│ list-windows  │ [session_name]                              │ List windows in a tmux session           │
│ kill-window   │ [session_name] [name]                       │ Kill a window in a tmux session          │
│ rename-session│ [session_name] [new_name]                   │ Rename an existing tmux session          │
│ rename-window │ [session_name] [window_index] [new_name]    │ Rename a window in tmux session          │
│ split-window  │ [session_name] [window_index] [v|h]        │ Split a pane in tmux window              │
│ list-panes    │ [session_name] [window_index]              │ List all panes in tmux window            │
│ kill-pane     │ [session_name] [window_index] [pane_index] │ Kill a specific pane in tmux window      │
│ move-window   │ [session_name] [window_index] [new_session]│ Move window to another tmux session      │
│ swap-window   │ [session_name] [index1] [index2]          │ Swap two windows in tmux session         │
│ resize-pane   │ [session_name] [window] [pane] [option]   │ Resize a pane in tmux window            │
│ send-keys     │ [session_name] [window] [pane] [keys]     │ Send keys to a pane in tmux window       │
│ nuke          │                                             │ Remove all tmux or screen sessions       │
│ version       │ [--check-update]                           │ Show version and check for updates       │
│ update        │                                             │ Update txm to the latest version         │
│ uninstall     │                                             │ Uninstall txm                           │
└────────────────┴─────────────────────────────────────────────┴─────────────────────────────────────────┘

Options:
  -v, --verbose    Enable verbose output
`
	fmt.Println(helpText)
}
