# txm Documentation

`txm` is a powerful terminal session manager that supports a built-in **native** backend, as well as **tmux**, **zellij**, and **GNU Screen**. This documentation covers all available commands, configuration options, and backend-specific features.

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
- [Seamless SSH Workflow](#seamless-ssh-workflow)
- [Backend-Specific Notes](#backend-specific-notes)
- [Troubleshooting](#troubleshooting)

## Configuration System

txm includes a comprehensive configuration system that allows you to set your preferred backend and customize behavior.

### Configuration File

Configuration is stored in `~/.txm/config` using a simple `key=value` format:

```
backend=native
scrollback_size=131072
```

### Backend Selection Priority

1. **Environment Variable**: `TXM_DEFAULT_BACKEND` (highest priority)
2. **Config File**: `~/.txm/config` (persistent setting)
3. **Default**: Auto-detection (tmux → screen → zellij → native)

### Configuration Commands

```bash
# Set default backend
txm config set backend native

# Get specific configuration
txm config get backend

# Show all configuration
txm config show
```

## Backend Support

| Backend | Session Mgmt | Window/Tab Mgmt | Pane/Panel Ops | Advanced Features |
|---------|--------------|-----------------|----------------|-------------------|
| **native** | ✓ | ✗ | ✗ | Built-in, lightweight, zero dependencies |
| **tmux** | ✓ | ✓ | ✓ | Full feature set |
| **zellij** | ✓ | ✓ | ✓ | Modern workspace |
| **screen** | ✓ | ✓ | Basic | Fallback support |

## Command Line Options

```
txm [command] [arguments] [-v|--verbose]
```

The `-v` or `--verbose` flag enables detailed logging, useful for debugging or learning how commands work.

## Autocompletion

`txm` automatically installs native shell autocompletion for `bash`, `zsh`, and `fish` when you run `txm install`. The installer places the generated completion scripts into standard locations dynamically:

- **Bash**: `~/.local/share/bash-completion/completions/txm` (or `/usr/share/bash-completion/` for system installs)
- **Zsh**: `~/.zfunc/_txm` (You must add `fpath+=~/.zfunc` to your `.zshrc`)
- **Fish**: `~/.config/fish/completions/txm.fish`

For a temporary session, or for PowerShell, you can generate it on the fly:
```bash
source <(txm completion bash)   # bash
source <(txm completion zsh)    # zsh
txm completion fish | source    # fish
txm completion powershell       # powershell
```

When you uninstall the app using `txm uninstall`, these autocompletion files are automatically purged from your system.

## Basic Commands

### Interactive picker
Launch interactive fuzzy-finder picker to select and preview sessions
```bash
txm
```

### create
Create a new session (optionally run a specific command)
```bash
txm create [session_name] [command...]
```
- `--log`: Mirror PTY output to a persistent file with automatic size-based log rotation.

### list
List all active sessions and display the number of active clients attached
```bash
txm list
```

### attach
Attach to an existing session. If no name is provided, it automatically attaches to the only available session or creates a default one. Can also accept custom startup commands.
```bash
txm attach [session_name] [command...]
```
- `-r`, `--read-only`: Attach in read-only mode for safe, interference-free session monitoring.

### detach
Detach from current session. (Alternatively, use `Ctrl+\` when in a native session to gracefully detach).
```bash
txm detach
```

### delete
Delete a session
```bash
txm delete [session_name]
```

### exec
Remotely execute commands inside background sessions/panes.
```bash
txm exec [session] [window] [pane] [cmd]
```

### generate-ssh-config
Automatically generate zmx-style `ControlMaster` SSH configurations for seamless SSH workflows.
```bash
txm generate-ssh-config
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

## Window Management (`txm window`)

### new
Create a new window (supported across most backends except native)
```bash
txm window new [session_name] [window_name]
```

### list
List windows in a session
```bash
txm window list [session_name]
```

### kill
Remove a window
```bash
txm window kill [session_name] [window_name]
```

### next
Switch to next window in session
```bash
txm window next [session_name]
```

### prev
Switch to previous window in session
```bash
txm window prev [session_name]
```

### rename
Rename a window
```bash
txm window rename [session_name] [old_window_name] [new_window_name]
```

### split
Split a window into panes
```bash
txm window split [session_name] [window_name] [v|h]
```
- `v`: vertical split
- `h`: horizontal split

## Pane Operations (`txm pane`)

Note: These commands are available in tmux and zellij, with limited support in screen.

### list
List panes in a window
```bash
txm pane list [session_name] [window_name]
```

### kill
Remove a pane
```bash
txm pane kill [session_name] [window_name] [pane_number]
```

## Environment Variables

### TXM_DEFAULT_BACKEND
Override the default backend temporarily:
```bash
export TXM_DEFAULT_BACKEND=native
# or
TXM_DEFAULT_BACKEND=tmux txm create my-session
```

### TXM_SESSION_PREFIX
Automatically prefixes all session names globally.
```bash
export TXM_SESSION_PREFIX=my_env_
```

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

## Seamless SSH Workflow

Using `txm` with SSH is a first-class citizen. Instead of using SSH to remote into a system with a single terminal and managing `n` `tmux` panes remotely, we can open `n` local terminal tabs and run `ssh` for all of them. This allows us to leverage our local terminal emulator's native tabs, splits, and scrollback, while perfectly preserving the sessions remotely.

To make this a delightful workflow, we can add an SSH config entry for our remote dev server. Generate the recommended setup block instantly by running:
```bash
txm generate-ssh-config
```

The output will look something like this. Add it to our `~/.ssh/config`:
```bash
Host d.*
    HostName 192.168.1.xxx
    
    # Automatically attach to a txm session on the remote server
    # named after the remote host we're connecting to (%h).
    RemoteCommand txm attach %h
    RequestTTY yes

    # Multiplex multiple PTY sessions to a single server over one connection
    ControlMaster auto
    ControlPath ~/.ssh/cm-%r@%h:%p
    ControlPersist 10m
```

Architecturally, SSH supports multiplexing multiple channels of communication within a single TCP connection to a server. `ControlMaster` is the setting that tells SSH to multiplex multiple PTY sessions over that one connection.

Now we can spawn as many terminal sessions as we'd like:
```bash
ssh d.term
ssh d.irc
ssh d.dotfiles
```

Because `txm attach` is an "upsert" command, the first `ssh` command will automatically create the session on the remote host, and subsequent commands to the same host will attach to it (or create entirely new sessions if connecting to different hosts). 

### Auto-Reconnection
We can use the `autossh` tool to make SSH connections auto-reconnect. If we close our laptop lid, it will automatically reconnect all our SSH connections when we reopen it:
```bash
autossh -M 0 -q d.term
```
*(Tip: create an alias/abbr for this, e.g., `alias ash="autossh -M 0 -q"`)*

Now we can set up our OS tiling windows how we like them for our project and have as many windows as we'd like, replicating exactly what `tmux` does but with native windows, tabs, splits, and scrollback!

## Backend-Specific Notes

### native
- **Zero dependencies**: No external multiplexer needed
- **State & Scrollback**: Maintains configurable scrollback ring buffer
- **Lightweight**: Optimized for simple persistent single-pane sessions
- **Graceful Detach**: Keybinding `Ctrl+\` to detach cleanly

### tmux
- **Full feature support**: Complete implementation of all window and pane operations
- **Session persistence**: Robust session management with server/client architecture
- **Configuration**: Extensive customization via `.tmux.conf`

### zellij
- **Modern workspace paradigm**: Tab-based workflow with floating panes
- **Built-in layouts**: Predefined and custom layouts
- **Session management**: Contemporary approach to terminal multiplexing

### GNU Screen
- **Legacy compatibility**: Reliable fallback for older systems
- **Basic session management**: Core session operations

## Troubleshooting

### Backend Selection Issues

1. **Check available backends**:
```bash
which tmux zellij screen
```

2. **Test backend functionality**:
```bash
# Test each backend individually
TXM_DEFAULT_BACKEND=native txm -v list
TXM_DEFAULT_BACKEND=tmux txm -v list
TXM_DEFAULT_BACKEND=zellij txm -v list
```

3. **Reset configuration**:
```bash
rm -rf ~/.txm
txm config show  # Will recreate with defaults
```
