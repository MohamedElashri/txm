package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

var commonTmuxPaths = []string{
	"/usr/bin/tmux",
	"/usr/local/bin/tmux",
	"/opt/homebrew/bin/tmux",
	"/home/linuxbrew/.linuxbrew/bin/tmux",
}

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

func preserveEnvironment(cmd *exec.Cmd) {
	if os.Getuid() == 0 {
		userPath := os.Getenv("SUDO_USER")
		if userPath != "" {
			output, err := exec.Command("getent", "passwd", userPath).Output()
			if err == nil {
				fields := strings.Split(string(output), ":")
				if len(fields) > 5 {
					homeDir := fields[5]
					paths := []string{
						"/usr/local/bin",
						"/usr/bin",
						"/bin",
						filepath.Join(homeDir, ".local/bin"),
						"/home/linuxbrew/.linuxbrew/bin",
						filepath.Join(homeDir, "/.linuxbrew/bin"),
					}
					cmd.Env = append(os.Environ(), "PATH="+strings.Join(paths, ":"))
				}
			}
		}
	}
}

func (sm *SessionManager) newScreenWindow(session string) error {
	return sm.runScreenCommand("-S", session, "-X", "screen")
}

func (sm *SessionManager) listScreenWindows(session string) error {
	return sm.runScreenCommand("-S", session, "-Q", "windows")
}

func (sm *SessionManager) killScreenWindow(session string) error {
	return sm.runScreenCommand("-S", session, "-X", "kill")
}

func (sm *SessionManager) renameScreenWindow(session, newName string) error {
	return sm.runScreenCommand("-S", session, "-X", "title", newName)
}

func (sm *SessionManager) splitScreenWindow(session, direction string) error {
	if direction == "v" {
		return sm.runScreenCommand("-S", session, "-X", "split")
	}
	return fmt.Errorf("horizontal splitting not supported in screen")
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
	preserveEnvironment(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (sm *SessionManager) runScreenCommand(args ...string) error {
	cmd := exec.Command("screen", args...)
	preserveEnvironment(cmd)
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
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("new-window", "-t", session, "-n", name); err != nil {
			sm.logError(fmt.Sprintf("Failed to create window '%s' in tmux session '%s'", name, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Window '%s' created in tmux session '%s'", name, session))
		return
	}

	if err := sm.newScreenWindow(session); err != nil {
		sm.logError(fmt.Sprintf("Failed to create window in screen session '%s'", session))
		return
	}
	if err := sm.renameScreenWindow(session, name); err != nil {
		sm.logWarning(fmt.Sprintf("Created window but failed to rename it in screen session '%s'", session))
		return
	}
	sm.logInfo(fmt.Sprintf("Window '%s' created in screen session '%s'", name, session))
}

func (sm *SessionManager) listWindows(session string) {
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("list-windows", "-t", session); err != nil {
			sm.logError(fmt.Sprintf("Failed to list windows in tmux session '%s'", session))
		}
		return
	}

	if err := sm.listScreenWindows(session); err != nil {
		sm.logError(fmt.Sprintf("Failed to list windows in screen session '%s'", session))
	}
}

func (sm *SessionManager) killWindow(session, window string) {
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("kill-window", "-t", fmt.Sprintf("%s:%s", session, window)); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill window '%s' in tmux session '%s'", window, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Window '%s' killed in tmux session '%s'", window, session))
		return
	}

	if err := sm.killScreenWindow(session); err != nil {
		sm.logError(fmt.Sprintf("Failed to kill window in screen session '%s'", session))
		return
	}
	sm.logInfo(fmt.Sprintf("Window killed in screen session '%s'", session))
}

func (sm *SessionManager) nextWindow(session string) {
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("next-window", "-t", session); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to next window in tmux session '%s'", session))
			return
		}
	} else {
		if err := sm.runScreenCommand("-S", session, "-X", "next"); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to next window in screen session '%s'", session))
			return
		}
	}
	sm.logInfo("Switched to next window")
}

func (sm *SessionManager) previousWindow(session string) {
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("previous-window", "-t", session); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to previous window in tmux session '%s'", session))
			return
		}
	} else {
		if err := sm.runScreenCommand("-S", session, "-X", "prev"); err != nil {
			sm.logError(fmt.Sprintf("Failed to switch to previous window in screen session '%s'", session))
			return
		}
	}
	sm.logInfo("Switched to previous window")
}

func (sm *SessionManager) attachSession(name string) {
	if !sm.sessionExists(name) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", name))
		return
	}

	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("attach-session", "-t", name); err != nil {
			sm.logError(fmt.Sprintf("Failed to attach to tmux session '%s'", name))
			return
		}
		return
	}

	if err := sm.runScreenCommand("-r", name); err != nil {
		sm.logError(fmt.Sprintf("Failed to attach to screen session '%s'", name))
		return
	}
}

func (sm *SessionManager) detachSession() {
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("detach-client"); err != nil {
			sm.logError("Failed to detach from tmux session")
			return
		}
		sm.logInfo("Detached from tmux session")
		return
	}

	if err := sm.runScreenCommand("-d"); err != nil {
		sm.logError("Failed to detach from screen session")
		return
	}
	sm.logInfo("Detached from screen session")
}

func (sm *SessionManager) killSession(name string) {
	if !sm.sessionExists(name) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", name))
		return
	}

	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("kill-session", "-t", name); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill tmux session '%s'", name))
			return
		}
		sm.logInfo(fmt.Sprintf("Killed tmux session '%s'", name))
		return
	}

	if err := sm.runScreenCommand("-X", "-S", name, "quit"); err != nil {
		sm.logError(fmt.Sprintf("Failed to kill screen session '%s'", name))
		return
	}
	sm.logInfo(fmt.Sprintf("Killed screen session '%s'", name))
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
│ detach        │                                             │ Detach from current session              │
│ delete        │ [session_name]                              │ Delete a tmux or screen session          │
│ new-window    │ [session_name] [name]                       │ Create a new window in session           │
│ list-windows  │ [session_name]                              │ List windows in a session                │
│ kill-window   │ [session_name] [name]                       │ Kill a window in session                 │
│ next-window   │ [session_name]                              │ Switch to next window in session         │
│ prev-window   │ [session_name]                              │ Switch to previous window in session     │
│ version       │ [--check-update]                           │ Show version and check for updates       │
│ update        │                                             │ Update txm to the latest version         │
│ uninstall     │                                             │ Uninstall txm                           │
└────────────────┴─────────────────────────────────────────────┴─────────────────────────────────────────┘

Options:
  -v, --verbose    Enable verbose output

Note: Screen has limited window management capabilities compared to tmux.
      Some commands may behave differently when using screen as the backend.
`
	fmt.Println(helpText)
}
