package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func (sm *SessionManager) renameSession(oldName, newName string) {
	if !sm.sessionExists(oldName) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", oldName))
		return
	}

	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("rename-session", "-t", oldName, newName); err != nil {
			sm.logError(fmt.Sprintf("Failed to rename tmux session from '%s' to '%s'", oldName, newName))
			return
		}
		sm.logInfo(fmt.Sprintf("Renamed tmux session from '%s' to '%s'", oldName, newName))
		return
	}

	sm.logError("Session renaming is not supported in GNU Screen")
}

func (sm *SessionManager) renameWindow(session, oldName, newName string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	if sm.tmuxAvailable {
		windowTarget := fmt.Sprintf("%s:%s", session, oldName)
		if err := sm.runTmuxCommand("rename-window", "-t", windowTarget, newName); err != nil {
			sm.logError(fmt.Sprintf("Failed to rename window from '%s' to '%s' in tmux session '%s'", oldName, newName, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Renamed window from '%s' to '%s' in tmux session '%s'", oldName, newName, session))
		return
	}

	if err := sm.runScreenCommand("-S", session, "-X", "title", newName); err != nil {
		sm.logError(fmt.Sprintf("Failed to rename window in screen session '%s'", session))
		return
	}
	sm.logInfo(fmt.Sprintf("Renamed window to '%s' in screen session '%s'", newName, session))
}

func (sm *SessionManager) moveWindow(srcSession, windowName, dstSession string) {
	if !sm.sessionExists(srcSession) || !sm.sessionExists(dstSession) {
		sm.logError(fmt.Sprintf("Either source session '%s' or destination session '%s' does not exist", srcSession, dstSession))
		return
	}

	if sm.tmuxAvailable {
		windowTarget := fmt.Sprintf("%s:%s", srcSession, windowName)
		if err := sm.runTmuxCommand("move-window", "-s", windowTarget, "-t", dstSession); err != nil {
			sm.logError(fmt.Sprintf("Failed to move window '%s' from session '%s' to session '%s'", windowName, srcSession, dstSession))
			return
		}
		sm.logInfo(fmt.Sprintf("Moved window '%s' from session '%s' to session '%s'", windowName, srcSession, dstSession))
		return
	}

	sm.logError("Moving windows between sessions is not supported in GNU Screen")
}

func (sm *SessionManager) swapWindow(session, windowName1, windowName2 string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	if sm.tmuxAvailable {
		windowTarget1 := fmt.Sprintf("%s:%s", session, windowName1)
		windowTarget2 := fmt.Sprintf("%s:%s", session, windowName2)
		if err := sm.runTmuxCommand("swap-window", "-s", windowTarget1, "-t", windowTarget2); err != nil {
			sm.logError(fmt.Sprintf("Failed to swap windows '%s' and '%s' in tmux session '%s'", windowName1, windowName2, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Swapped windows '%s' and '%s' in tmux session '%s'", windowName1, windowName2, session))
		return
	}

	sm.logError("Swapping windows is not supported in GNU Screen")
}

func (sm *SessionManager) splitWindow(session, window, direction string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	if sm.tmuxAvailable {
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
	}

	if direction == "v" {
		if err := sm.runScreenCommand("-S", session, "-X", "split"); err != nil {
			sm.logError(fmt.Sprintf("Failed to split window in screen session '%s'", session))
			return
		}
		sm.logInfo(fmt.Sprintf("Split window vertically in screen session '%s'", session))
		return
	}

	sm.logError("Horizontal window splitting is not supported in GNU Screen")
}

func (sm *SessionManager) listPanes(session, window string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	if sm.tmuxAvailable {
		windowTarget := fmt.Sprintf("%s:%s", session, window)
		if err := sm.runTmuxCommand("list-panes", "-t", windowTarget); err != nil {
			sm.logError(fmt.Sprintf("Failed to list panes in window '%s' of tmux session '%s'", window, session))
			return
		}
		return
	}

	sm.logError("Listing panes is not supported in GNU Screen")
}

func (sm *SessionManager) killPane(session, window, pane string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	if sm.tmuxAvailable {
		paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
		if err := sm.runTmuxCommand("kill-pane", "-t", paneTarget); err != nil {
			sm.logError(fmt.Sprintf("Failed to kill pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Killed pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
		return
	}

	sm.logError("Killing panes is not supported in GNU Screen")
}

func (sm *SessionManager) resizePane(session, window, pane, direction string, size int) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	if sm.tmuxAvailable {
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
	}

	sm.logError("Resizing panes is not supported in GNU Screen")
}

func (sm *SessionManager) sendKeys(session, window, pane, keys string) {
	if !sm.sessionExists(session) {
		sm.logError(fmt.Sprintf("Session '%s' does not exist", session))
		return
	}

	if sm.tmuxAvailable {
		paneTarget := fmt.Sprintf("%s:%s.%s", session, window, pane)
		if err := sm.runTmuxCommand("send-keys", "-t", paneTarget, keys); err != nil {
			sm.logError(fmt.Sprintf("Failed to send keys to pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
			return
		}
		sm.logInfo(fmt.Sprintf("Sent keys to pane '%s' in window '%s' of tmux session '%s'", pane, window, session))
		return
	}

	sm.logError("Sending keys to panes is not supported in GNU Screen")
}

func (sm *SessionManager) nukeAllSessions() {
	if sm.tmuxAvailable {
		if err := sm.runTmuxCommand("kill-server"); err != nil {
			sm.logError("Failed to kill all tmux sessions")
			return
		}
		sm.logInfo("Killed all tmux sessions")
		return
	}

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
		size := 5 // Default size
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
│ update        │                                             │ Update txm to the latest version         │
│ uninstall     │                                             │ Uninstall txm                           │
│ version       │ [--check-update]                           │ Show version and check for updates       │
└────────────────┴─────────────────────────────────────────────┴─────────────────────────────────────────┘

Options:
  -v, --verbose    Enable verbose output

Note: Screen has limited window management capabilities compared to tmux.
      Some commands may behave differently when using screen as the backend.
`
	fmt.Println(helpText)
}
