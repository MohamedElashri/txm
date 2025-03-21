# txm Documentation

`txm` is a terminal session manager that primarily works with tmux while providing GNU Screen as a fallback option. This documentation covers all available commands and their usage.

## Table of Contents

- [Command Line Options](#command-line-options)
- [Basic Commands](#basic-commands)
- [Window Management](#window-management)
- [Pane Operations](#pane-operations)
- [Advanced Operations](#advanced-operations)
- [Verbose Mode](#verbose-mode)

## Command Line Options

```
txm [command] [arguments] [-v|--verbose]
```

The `-v` or `--verbose` flag enables detailed logging, useful for debugging or learning how commands work.

## Basic Commands

### create
Create a new session
```bash
txm create [session_name]
```

### list
List all active sessions
```bash
txm list
```

### attach
Attach to an existing session
```bash
txm attach [session_name]
```

### detach
Detach from current session
```bash
txm detach
```

### delete
Delete a session
```bash
txm delete [session_name]
```

### nuke
Remove all sessions
```bash
txm nuke
```

### update
Update txm to the latest version
```bash
txm update
```

### uninstall
Uninstall txm from your system
```bash
txm uninstall
```

### version
Show version and check for updates
```bash
txm version [--check-update]
```

## Window Management

### new-window
Create a new window (supported in both tmux and screen)
```bash
txm new-window [session_name] [window_name]
```

### list-windows
List windows in a session (supported in both tmux and screen)
```bash
txm list-windows [session_name]
```

### kill-window
Remove a window (supported in both tmux and screen)
```bash
txm kill-window [session_name] [window_name]
```

### next-window
Switch to next window in session (supported in both tmux and screen)
```bash
txm next-window [session_name]
```

### prev-window
Switch to previous window in session (supported in both tmux and screen)
```bash
txm prev-window [session_name]
```

### rename-session
Rename an existing session (tmux only)
```bash
txm rename-session [old_name] [new_name]
```

### rename-window
Rename a window (supported in both tmux and screen)
```bash
txm rename-window [session_name] [old_window_name] [new_window_name]
```

### move-window
Move window between sessions (tmux only)
```bash
txm move-window [source_session] [window_name] [target_session]
```

### swap-window
Swap window positions (tmux only)
```bash
txm swap-window [session_name] [window1_name] [window2_name]
```

## Pane Operations

Note: These commands are only available when using tmux, except for split-window which has limited support in screen.

### split-window
Split a window into panes
```bash
txm split-window [session_name] [window_name] [v|h]
```
- `v`: vertical split (supported in both tmux and screen)
- `h`: horizontal split (tmux only)

### list-panes
List panes in a window (tmux only)
```bash
txm list-panes [session_name] [window_name]
```

### kill-pane
Remove a pane (tmux only)
```bash
txm kill-pane [session_name] [window_name] [pane_number]
```

### resize-pane
Resize a pane (tmux only)
```bash
txm resize-pane [session_name] [window_name] [pane_number] [direction] [size]
```
Directions:
- `U`: Resize up
- `D`: Resize down
- `L`: Resize left
- `R`: Resize right

The size parameter is optional and defaults to 5 cells.

### send-keys
Send keystrokes to a pane (tmux only)
```bash
txm send-keys [session_name] [window_name] [pane_number] [keys]
```

## Environment Variables

### NO_COLOR
Disable colored output:
```bash
export NO_COLOR=1
```

### TERM
Terminal type for capability detection. Common values:
- xterm
- xterm-256color
- screen
- screen-256color
- tmux
- tmux-256color
- linux

## Examples

1. Create and attach to a new session:
```bash
txm create mysession
txm attach mysession
```

2. Create and navigate windows (works in both tmux and screen):
```bash
txm new-window mysession mywindow
txm next-window mysession
txm prev-window mysession
```

3. Split window (vertical split works in both, horizontal in tmux only):
```bash
txm split-window mysession mywindow v  # works in both
txm split-window mysession mywindow h  # tmux only
```

4. Complex window management (tmux only):
```bash
# Create two sessions
txm create session1
txm create session2

# Create windows in session1
txm new-window session1 window1
txm new-window session1 window2

# Move window2 to session2
txm move-window session1 window2 session2
```

5. Manage panes (tmux only):
```bash
# Create a new window and split it horizontally
txm new-window mysession mywindow
txm split-window mysession mywindow h

# List panes in the window
txm list-panes mysession mywindow

# Resize a pane
txm resize-pane mysession mywindow 0 U 10  # Resize pane 0 up by 10 cells

# Send a command to a pane
txm send-keys mysession mywindow 0 "echo hello"  # Send command to pane
```

## Troubleshooting

1. Enable verbose mode to see detailed logs:
```bash
txm -v list
```

2. Check terminal capabilities:
```bash
echo $TERM
```

3. Verify tmux/screen installation:
```bash
which tmux
which screen
```

4. Color support issues:
- Check if NO_COLOR is set
- Verify TERM setting
- Ensure terminal supports colors

## Backend-Specific Notes

### tmux
- Full support for all window and pane operations
- Advanced window management (`move`, `swap`)
- Flexible pane splitting (both vertical and horizontal)

### GNU Screen
- Basic window management support
- Window navigation (`next`, `previous`)
- Window creation and renaming
- Only vertical splitting supported
- No pane management beyond basic splitting
- Some commands may behave differently than in tmux

## General Notes

- When tmux is not available, `txm` automatically falls back to GNU Screen
- Color support is automatically detected based on terminal capabilities
- Use verbose mode (-v) for debugging and learning
- Window management commands try to provide consistent behavior across both backends where possible
