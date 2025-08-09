# txm Documentation

`txm` is a powerful terminal session manager that supports multiple backends: **tmux**, **zellij**, and **GNU Screen**. This documentation covers all available commands, configuration options, and backend-specific features.

## Table of Contents

- [Configuration System](#configuration-system)
- [Backend Support](#backend-support)
- [Command Line Options](#command-line-options)
- [Basic Commands](#basic-commands)
- [Window Management](#window-management)
- [Pane Operations](#pane-operations)
- [Configuration Commands](#configuration-commands)
- [Advanced Operations](#advanced-operations)
- [Environment Variables](#environment-variables)
- [Backend-Specific Notes](#backend-specific-notes)
- [Verbose Mode](#verbose-mode)

## Configuration System

txm includes a comprehensive configuration system that allows you to set your preferred backend and customize behavior.

### Configuration File

Configuration is stored in `~/.txm/config` using a simple `key=value` format:

```
backend=zellij
```

### Backend Selection Priority

1. **Environment Variable**: `TXM_DEFAULT_BACKEND` (highest priority)
2. **Config File**: `~/.txm/config` (persistent setting)
3. **Default**: tmux (if available, otherwise first available backend)

### Configuration Commands

```bash
# Set default backend
txm config set backend zellij

# Get specific configuration
txm config get backend

# Show all configuration
txm config show
```

## Backend Support

| Backend | Session Mgmt | Window/Tab Mgmt | Pane/Panel Ops | Advanced Features |
|---------|--------------|-----------------|----------------|-------------------|
| **tmux** | ✓ | ✓ | ✓ | Full feature set |
| **zellij** | ✓ | ✓ | ✓ | Modern workspace |
| **screen** | ✓ | ✓ | Basic | Fallback support |

## Command Line Options

```
txm [command] [arguments] [-v|--verbose]
```

The `-v` or `--verbose` flag enables detailed logging, useful for debugging or learning how commands work.

### Backend Override

```bash
# Temporarily use specific backend
TXM_DEFAULT_BACKEND=zellij txm create my-session

# Use with verbose mode
TXM_DEFAULT_BACKEND=tmux txm -v list
```

## Configuration Commands

### config set
Set a configuration value
```bash
txm config set backend zellij
txm config set backend tmux
txm config set backend screen
```

### config get
Get a configuration value
```bash
txm config get backend
```

### config show
Show all configuration
```bash
txm config show
```

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
Create a new window (supported across all backends)
```bash
txm new-window [session_name] [window_name]
```

### list-windows
List windows in a session (supported across all backends)
```bash
txm list-windows [session_name]
```

### kill-window
Remove a window (supported across all backends)
```bash
txm kill-window [session_name] [window_name]
```

### next-window
Switch to next window in session (supported across all backends)
```bash
txm next-window [session_name]
```

### prev-window
Switch to previous window in session (supported across all backends)
```bash
txm prev-window [session_name]
```

### rename-session
Rename an existing session (tmux and zellij only)
```bash
txm rename-session [old_name] [new_name]
```

### rename-window
Rename a window (supported across all backends)
```bash
txm rename-window [session_name] [old_window_name] [new_window_name]
```

### move-window
Move window between sessions (tmux and zellij only)
```bash
txm move-window [source_session] [window_name] [target_session]
```

### swap-window
Swap window positions (tmux only)
```bash
txm swap-window [session_name] [window1_name] [window2_name]
```

## Pane Operations

Note: These commands are available in tmux and zellij, with limited support in screen.

### split-window
Split a window into panes
```bash
txm split-window [session_name] [window_name] [v|h]
```
- `v`: vertical split (supported in tmux, zellij, and screen)
- `h`: horizontal split (tmux and zellij only)

### list-panes
List panes in a window (tmux and zellij only)
```bash
txm list-panes [session_name] [window_name]
```

### kill-pane
Remove a pane (tmux and zellij only)
```bash
txm kill-pane [session_name] [window_name] [pane_number]
```

### resize-pane
Resize a pane (tmux and zellij only)
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
Send keystrokes to a pane (tmux and zellij only)
```bash
txm send-keys [session_name] [window_name] [pane_number] [keys]
```

## Environment Variables

### TXM_DEFAULT_BACKEND
Override the default backend temporarily:
```bash
export TXM_DEFAULT_BACKEND=zellij
# or
TXM_DEFAULT_BACKEND=tmux txm create my-session
```

Supported values:
- `tmux`: Use tmux backend
- `zellij`: Use zellij backend  
- `screen`: Use GNU Screen backend

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

### Backend Configuration Examples

1. Set up your preferred backend:
```bash
# Set zellij as default
txm config set backend zellij

# Verify configuration
txm config show

# Create session with default backend
txm create my-project
```

2. Temporarily use different backend:
```bash
# Use tmux for this session only
TXM_DEFAULT_BACKEND=tmux txm create tmux-session

# Use zellij with verbose output
TXM_DEFAULT_BACKEND=zellij txm -v create zellij-session
```

### Multi-Backend Workflow

```bash
# Set up development environment with different backends
txm config set backend tmux
txm create main-dev

# Create specialized environments
TXM_DEFAULT_BACKEND=zellij txm create ui-work
TXM_DEFAULT_BACKEND=screen txm create legacy-work

# List all sessions (regardless of backend)
txm list
```

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
- **Full feature support**: Complete implementation of all window and pane operations
- **Advanced window management**: Move, swap, and complex window operations
- **Flexible pane splitting**: Both vertical and horizontal splitting with full control
- **Session persistence**: Robust session management with server/client architecture
- **Configuration**: Extensive customization via `.tmux.conf`

### zellij
- **Modern workspace paradigm**: Tab-based workflow with floating panes
- **Built-in layouts**: Predefined and custom layouts
- **Session management**: Contemporary approach to terminal multiplexing
- **Plugin system**: Extensible via WebAssembly plugins
- **Configuration**: YAML-based configuration system

### GNU Screen
- **Legacy compatibility**: Reliable fallback for older systems
- **Basic session management**: Core session operations
- **Limited pane support**: Only vertical splitting supported
- **Simple window operations**: Basic window navigation and management
- **Wide availability**: Usually pre-installed on most Unix systems
- **Configuration**: Classic `.screenrc` configuration

### Command Behavior Differences

| Operation | tmux | zellij | screen |
|-----------|------|--------|--------|
| Window/Tab naming | Window names | Tab names | Window names |
| Pane splitting | H/V + advanced | Floating + tiled | V only |
| Session listing | Detailed status | Workspace info | Basic list |
| Attach behavior | Multiple clients | Single session | Single attach |

## Troubleshooting

### Backend Selection Issues

1. **Check available backends**:
```bash
which tmux zellij screen
```

2. **Test backend functionality**:
```bash
# Test each backend individually
TXM_DEFAULT_BACKEND=tmux txm -v list
TXM_DEFAULT_BACKEND=zellij txm -v list
TXM_DEFAULT_BACKEND=screen txm -v list
```

3. **Reset configuration**:
```bash
rm -rf ~/.txm
txm config show  # Will recreate with defaults
```

### Backend-Specific Issues

1. **tmux not starting**:
   - Check tmux server status: `tmux info`
   - Verify tmux configuration: `tmux -f /dev/null list-sessions`

2. **zellij session problems**:
   - Check zellij version: `zellij --version`
   - Verify zellij config: `zellij setup --check`

3. **screen compatibility**:
   - Enable verbose mode: `txm -v create test-session`
   - Check screen version: `screen -version`
