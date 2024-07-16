package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
    "golang.org/x/crypto/ssh/terminal"
)

const (
    defaultSessionName = "my_session"
    logDirectory       = "logs/"
    layoutDirectory    = "$HOME/.txm/layouts"
    configFile         = "$HOME/.txm/config"
)

func main() {
    if len(os.Args) < 2 {
        displayHelp()
        return
    }

    switch os.Args[1] {
    case "new":
        newSession(getArg(2, defaultSessionName))
    case "list":
        listSessions()
    case "attach":
        attachSession(getArg(2, ""))
    case "detach":
        detachSession()
    case "rename":
        renameSession(getArg(2, ""), getArg(3, ""))
    case "kill":
        killSession(getArg(2, ""))
    case "switch":
        switchSession(getArg(2, ""))
    case "new-window":
        newWindow(getArg(2, ""))
    case "rename-window":
        renameWindow(getArg(2, ""), getArg(3, ""))
    case "close-window":
        closeWindow(getArg(2, ""))
    case "switch-window":
        switchWindow(getArg(2, ""))
    case "vsplit":
        vsplitPane()
    case "hsplit":
        hsplitPane()
    case "navigate":
        navigatePanes(getArg(2, ""))
    case "resize":
        resizePane(getArg(2, ""), getArg(3, ""))
    case "close-pane":
        closePane()
    case "zoom":
        zoomPane()
    case "run":
        executeCommand(os.Args[2:])
    case "save-layout":
        saveLayout(getArg(2, ""))
    case "restore-layout":
        restoreLayout(getArg(2, ""))
    case "set-option":
        setOption(getArg(2, ""), getArg(3, ""))
    case "execute-script":
        executeScript(getArg(2, ""), getArg(3, ""))
    case "broadcast":
        broadcastInput(os.Args[2:])
    case "help":
        displayHelp()
    default:
        displayHelp()
    }
}

func getArg(index int, defaultValue string) string {
    if index < len(os.Args) {
        return os.Args[index]
    }
    return defaultValue
}

func runTmuxCommand(args ...string) {
    cmd := exec.Command("tmux", args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin
    err := cmd.Run()
    if err != nil {
        fmt.Printf("Error running tmux command: %v\nCommand: tmux %s\n", err, strings.Join(args, " "))
    }
}

func newSession(sessionName string) {
    runTmuxCommand("new-session", "-d", "-s", sessionName)
    fmt.Printf("Created new session: %s\n", sessionName)
}

func listSessions() {
    runTmuxCommand("list-sessions")
}

func attachSession(sessionName string) {
    if !terminal.IsTerminal(int(os.Stdin.Fd())) {
        fmt.Println("Not running in an interactive terminal.")
        return
    }
    runTmuxCommand("attach-session", "-t", sessionName)
}

func detachSession() {
    runTmuxCommand("detach-client")
}

func renameSession(oldName, newName string) {
    runTmuxCommand("rename-session", "-t", oldName, newName)
    fmt.Printf("Renamed session %s to %s\n", oldName, newName)
}

func killSession(sessionName string) {
    runTmuxCommand("kill-session", "-t", sessionName)
    fmt.Printf("Killed session: %s\n", sessionName)
}

func switchSession(sessionName string) {
    runTmuxCommand("switch-client", "-t", sessionName)
}

func newWindow(windowName string) {
    runTmuxCommand("new-window", "-n", windowName)
    fmt.Printf("Created new window: %s\n", windowName)
}

func renameWindow(oldName, newName string) {
    runTmuxCommand("rename-window", "-t", oldName, newName)
    fmt.Printf("Renamed window %s to %s\n", oldName, newName)
}

func closeWindow(windowName string) {
    runTmuxCommand("kill-window", "-t", windowName)
    fmt.Printf("Closed window: %s\n", windowName)
}

func switchWindow(windowName string) {
    runTmuxCommand("select-window", "-t", windowName)
}

func vsplitPane() {
    runTmuxCommand("split-window", "-v")
}

func hsplitPane() {
    runTmuxCommand("split-window", "-h")
}

func navigatePanes(direction string) {
    switch direction {
    case "U":
        runTmuxCommand("select-pane", "-U")
    case "D":
        runTmuxCommand("select-pane", "-D")
    case "L":
        runTmuxCommand("select-pane", "-L")
    case "R":
        runTmuxCommand("select-pane", "-R")
    default:
        fmt.Println("Invalid direction. Use U, D, L, or R.")
    }
}

func resizePane(direction, amount string) {
    switch direction {
    case "U":
        runTmuxCommand("resize-pane", "-U", amount)
    case "D":
        runTmuxCommand("resize-pane", "-D", amount)
    case "L":
        runTmuxCommand("resize-pane", "-L", amount)
    case "R":
        runTmuxCommand("resize-pane", "-R", amount)
    default:
        fmt.Println("Invalid direction. Use U, D, L, or R.")
    }
}

func closePane() {
    runTmuxCommand("kill-pane")
    fmt.Println("Closed the current pane")
}

func zoomPane() {
    runTmuxCommand("resize-pane", "-Z")
}

func executeCommand(command []string) {
    cmd := strings.Join(command, " ")
    runTmuxCommand("send-keys", cmd, "C-m")
}

func saveLayout(layoutName string) {
    layoutPath := filepath.Join(os.ExpandEnv(layoutDirectory), layoutName+".layout")
    err := os.MkdirAll(filepath.Dir(layoutPath), os.ModePerm)
    if err != nil {
        fmt.Printf("Error creating layout directory: %v\n", err)
        return
    }
    output, err := exec.Command("tmux", "display-message", "-p", "#{window_layout}").Output()
    if err != nil {
        fmt.Printf("Error getting window layout: %v\n", err)
        return
    }
    err = os.WriteFile(layoutPath, output, 0644)
    if err != nil {
        fmt.Printf("Error saving layout: %v\n", err)
        return
    }
    fmt.Printf("Saved layout as %s\n", layoutName)
}

func restoreLayout(layoutName string) {
    layoutPath := filepath.Join(os.ExpandEnv(layoutDirectory), layoutName+".layout")
    if _, err := os.Stat(layoutPath); os.IsNotExist(err) {
        fmt.Printf("Layout %s not found\n", layoutName)
        return
    }
    layout, err := os.ReadFile(layoutPath)
    if err != nil {
        fmt.Printf("Error reading layout file: %v\n", err)
        return
    }
    runTmuxCommand("select-layout", string(layout))
    fmt.Printf("Restored layout %s\n", layoutName)
}

func setOption(option, value string) {
    runTmuxCommand("set-option", "-g", option, value)
    fmt.Printf("Set option %s to %s\n", option, value)
}

func executeScript(paneID, scriptFile string) {
    script, err := os.ReadFile(scriptFile)
    if err != nil {
        fmt.Printf("Error reading script file: %v\n", err)
        return
    }
    runTmuxCommand("send-keys", "-t", paneID, string(script), "C-m")
    fmt.Printf("Executed script %s in pane %s\n", scriptFile, paneID)
}

func broadcastInput(input []string) {
    cmd := strings.Join(input, " ")
    runTmuxCommand("set-window-option", "synchronize-panes", "on")
    runTmuxCommand("send-keys", cmd, "C-m")
    runTmuxCommand("set-window-option", "synchronize-panes", "off")
    fmt.Println("Broadcasted input to all panes")
}

func displayHelp() {
    helpText := `
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                                        txm Help                                      │
├────────────────┬─────────────────────────────────────────────────────────────────────┤
│    Command     │                              Description                             │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ new            │ Create a new session                                                │
│   <SESSION_NAME> │   The name of the session to create (default: my_session)         │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ list           │ List all sessions                                                   │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ attach         │ Attach to a session                                                 │
│   <SESSION_NAME> │   The name of the session to attach to                              │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ detach         │ Detach from the current session                                     │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ rename         │ Rename a session                                                    │
│   <OLD_NAME>     │   The current name of the session                                   │
│   <NEW_NAME>     │   The new name for the session                                      │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ kill           │ Kill a session                                                      │
│   <SESSION_NAME> │   The name of the session to kill                                   │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ switch         │ Switch to a different session                                       │
│   <SESSION_NAME> │   The name of the session to switch to                              │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ new-window     │ Create a new window                                                 │
│   <WINDOW_NAME>  │   The name of the window to create                                  │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ rename-window  │ Rename a window                                                     │
│   <OLD_NAME>     │   The current name of the window                                    │
│   <NEW_NAME>     │   The new name for the window                                       │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ close-window   │ Close a window                                                      │
│   <WINDOW_NAME>  │   The name of the window to close                                   │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ switch-window  │ Switch to a different window                                        │
│   <WINDOW_NAME>  │   The name of the window to switch to                               │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ vsplit         │ Split pane vertically                                               │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ hsplit         │ Split pane horizontally                                             │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ navigate       │ Navigate between panes                                              │
│   <DIRECTION>    │   The direction to navigate (U: Up, D: Down, L: Left, R: Right)     │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ resize         │ Resize pane                                                         │
│   <DIRECTION>    │   The direction to resize (U: Up, D: Down, L: Left, R: Right)       │
│   <AMOUNT>       │   The amount to resize the pane by                                  │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ close-pane     │ Close the current pane                                              │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ zoom           │ Zoom in/out of the current pane                                     │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ run            │ Execute a command in the current pane                               │
│   <COMMAND>      │   The command to execute                                            │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ save-layout    │ Save the current session layout                                     │
│   <LAYOUT_NAME>  │   The name to save the layout as                                    │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ restore-layout │ Restore a previously saved session layout                           │
│   <LAYOUT_NAME>  │   The name of the layout to restore                                 │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ set-option     │ Set a tmux option                                                   │
│   <OPTION>       │   The tmux option to set                                            │
│   <VALUE>        │   The value to set the option to                                    │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ execute-script │ Execute a script in a specific pane                                 │
│   <PANE_ID>      │   The ID of the pane to execute the script in                       │
│   <SCRIPT_FILE>  │   The path to the script file to execute                            │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ broadcast      │ Broadcast input to all panes                                        │
│   <INPUT>        │   The input to broadcast to all panes                               │
├────────────────┼─────────────────────────────────────────────────────────────────────┤
│ help           │ Display this help information                                       │
└────────────────┴─────────────────────────────────────────────────────────────────────┘
`
    fmt.Println(helpText)
}

