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
Note: For GNU Screen, use Ctrl-a d instead.

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

## Window Management

Note: These commands are only available when using tmux.

### new-window
Create a new window
```bash
txm new-window [session_name] [window_name]
```

### list-windows
List windows in a session
```bash
txm list-windows [session_name]
```

### kill-window
Remove a window
```bash
txm kill-window [session_name] [window_name]
```

### rename-session
Rename an existing session
```bash
txm rename-session [old_name] [new_name]
```

### rename-window
Rename a window
```bash
txm rename-window [session_name] [window_index] [new_name]
```

### move-window
Move window between sessions
```bash
txm move-window [source_session] [window_index] [target_session]
```

### swap-window
Swap window positions
```bash
txm swap-window [session_name] [index1] [index2]
```

## Pane Operations

Note: These commands are only available when using tmux.

### split-window
Split a window into panes
```bash
txm split-window [session_name] [window_index] [v|h]
```
- `v`: vertical split
- `h`: horizontal split

### list-panes
List panes in a window
```bash
txm list-panes [session_name] [window_index]
```

### kill-pane
Remove a pane
```bash
txm kill-pane [session_name] [window_index] [pane_index]
```

### resize-pane
Resize a pane
```bash
txm resize-pane [session_name] [window_index] [pane_index] [option]
```
Options:
- `-U [n]`: Resize up by n cells
- `-D [n]`: Resize down by n cells
- `-L [n]`: Resize left by n cells
- `-R [n]`: Resize right by n cells

### send-keys
Send keystrokes to a pane
```bash
txm send-keys [session_name] [window_index] [pane_index] [keys]
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

2. Create a new window and split it:
```bash
txm new-window mysession mywindow
txm split-window mysession 0 v
```

3. Send commands to a pane:
```bash
txm send-keys mysession 0 1 "ls -la"
```

4. Complex window management:
```bash
# Create two sessions
txm create session1
txm create session2

# Create windows in session1
txm new-window session1 window1
txm new-window session1 window2

# Move window2 to session2
txm move-window session1 2 session2
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

## Notes

- When tmux is not available, txm automatically falls back to GNU Screen with reduced functionality
- Window and pane operations are only available in tmux
- Color support is automatically detected based on terminal capabilities
- Use verbose mode (-v) for debugging and learning